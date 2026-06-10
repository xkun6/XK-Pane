// Package worktreesvc 提供「会话隔离 worktree」的自动创建服务（借鉴 paseo 的
// auto-worktree 设计），把 parallel-run skill 的"同项目并行 git 冲突"铁律从
// prompt 叮嘱升级为机制隔离。
//
// 创建流程（用户拍板的顺序，三步）：
//  1. .gitignore 守门：.claude/.codex/.xkpanes 三个工具目录是 XK-Pane 启动会话前
//     必写的（projecthooks.Sync / 本地历史），本就不该入库——未被忽略则追加进主仓
//     .gitignore 并单独提交一次（pathspec 限定提交，绝不带走用户已暂存的内容）；
//     已被忽略（无论 .gitignore / info/exclude / 全局规则）则跳过，直接建树。
//     这同时解决了"worktree 一启动就恒脏 → 干净判定永不成立"的死锁。
//  2. 集中根建树：<UserDataDir>/worktrees/<hash8(主仓根)>/<形容词-动物 slug>，
//     `git worktree add -b <slug>`（基于当前 HEAD）。集中根（而非项目目录）=
//     paseo 的"路径即所有权凭证"：不污染源 checkout、删除前只验前缀。
//  3. 登记：把 worktree 路径注册进主仓所属 workspace 的 workspace_projects（alias =
//     slug）——这是唯一同时满足「orchestrator 注册闸门放行 + 兄弟项目注入生效」的
//     归属（兄弟注入只查 workspace_projects 精确匹配）。主仓不在任何 workspace 时
//     跳过登记（桌面启动不查注册，仍可用）。
//
// 生命周期收口（自动清理/归档）刻意不在本单元：删除链涉及 lhwatcher 句柄、
// localhistorysvc SQLite 句柄的硬顺序，独立单元处理；当前 GitPage 手动 worktree
// 管理可用。
package worktreesvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"xkpane/internal/storage"
	"xkpane/internal/utils/filex"
	"xkpane/internal/utils/gitx"
	"xkpane/internal/utils/logx"
)

// toolDirs 是 XK-Pane 会写进项目目录的三个工具目录（会话 hook 配置 / 注入 skill /
// 本地历史）。.gitignore 里以目录形式（尾斜杠）追加。
var toolDirs = []string{".claude", ".codex", ".xkpanes"}

// ignoreCommitMessage 是自动补 .gitignore 时的提交信息（写进用户仓库，保持简短双语义）。
const ignoreCommitMessage = "chore: ignore XK-Pane tool dirs (.claude/.codex/.xkpanes)"

// WorkspaceRegistry 是 workspace service 的窄接口（worktreesvc 只消费这四个方法；
// 窄化便于单测 stub，对齐 webapi 窄接口范式）。
type WorkspaceRegistry interface {
	List(ctx context.Context) ([]storage.Workspace, error)
	AddProject(ctx context.Context, workspaceID, path string) (storage.WorkspaceProject, error)
	SetProjectAlias(ctx context.Context, projectID string, alias *string) error
	RemoveProject(ctx context.Context, projectID string) error
}

// Service 是自动 worktree 的 Wails service。无 storage holder（worktree 自身无表，
// 归属经 WorkspaceRegistry 落 workspace_projects）；worktreesRoot 可注入（单测用），
// 缺省 <UserDataDir>/worktrees。
type Service struct {
	workspaces    WorkspaceRegistry
	worktreesRoot string     // ""=惰性解析 UserDataDir
	mu            sync.Mutex // 串行化 CreateForProject：slug 占用检查与建树之间无原子性，靠锁闭并发窗口
}

// New 创建 worktreesvc。workspaces 可为 nil（仅跳过登记，创建仍可用）。
func New(workspaces WorkspaceRegistry) *Service {
	return &Service{workspaces: workspaces}
}

// setWorktreesRoot 覆盖集中根目录（单测注入临时目录用；非导出 → 不进 bindings，
// 前端不应能重定向 worktree 根）。
func (s *Service) setWorktreesRoot(root string) { s.worktreesRoot = root }

// CreateResult 是一次自动创建的结果（前端据此 launchProject 进 worktree）。
type CreateResult struct {
	WorktreePath    string `json:"worktreePath"`    // 新 worktree 绝对路径（= 新会话的 projectPath/cwd）
	Branch          string `json:"branch"`          // 创建并检出的新分支名（= slug）
	RepoRoot        string `json:"repoRoot"`        // 主仓根（前端可作 workspacePath 传 --add-dir）
	IgnoreCommitted bool   `json:"ignoreCommitted"` // 本次是否补写了 .gitignore 并提交
	WorkspaceID     string `json:"workspaceId"`     // 登记进的工作空间 id；"" = 未登记
}

// CreateForProject 为 projectPath 所在仓库自动创建一个隔离 worktree（完整三步：
// .gitignore 守门 → 建树 → workspace 登记）。projectPath 非 git 仓库时报错。
func (s *Service) CreateForProject(ctx context.Context, projectPath string) (CreateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(projectPath) == "" {
		return CreateResult{}, errors.New("worktreesvc: project path required")
	}
	repo, err := gitx.Open(projectPath)
	if err != nil {
		return CreateResult{}, err
	}
	root, err := repo.TopLevel(ctx)
	if err != nil {
		return CreateResult{}, err
	}
	// projectPath 可能是仓库子目录（workspace 登记任意目录）：后续所有 git 操作的
	// pathspec（.gitignore / check-ignore 探针）都必须以仓库根为 cwd 解析，否则会
	// 提交错文件 / 误判 ignore —— 拿到根后立即重开句柄。
	if root != "" && normalizePath(root) != normalizePath(projectPath) {
		if repo, err = gitx.Open(root); err != nil {
			return CreateResult{}, err
		}
	}

	// ① .gitignore 守门（已忽略则跳过——用户拍板的"如果在 .gitignore 就正常创建"）。
	committed, err := s.ensureToolDirsIgnored(ctx, repo, root)
	if err != nil {
		return CreateResult{}, err
	}

	// ② 集中根 + 随机 slug 建树（分支 = slug，基于当前 HEAD）。
	groupDir, err := s.groupDirFor(root)
	if err != nil {
		return CreateResult{}, err
	}
	slug, wtPath, err := s.pickSlug(ctx, repo, groupDir)
	if err != nil {
		return CreateResult{}, err
	}
	if err := os.MkdirAll(groupDir, 0o755); err != nil {
		return CreateResult{}, err
	}
	if err := repo.WorktreeAdd(ctx, wtPath, slug); err != nil {
		return CreateResult{}, err
	}

	// ③ 登记进主仓所属 workspace（找不到归属 / 登记失败仅降级：worktree 已可用，
	// 缺的只是编排面闸门 + 兄弟注入，不应让整次创建报错回滚）。归属匹配兼容两个口径：
	// workspace 里登记的可能是仓库根，也可能是用户实际选的子目录（projectPath）。
	wsID := s.registerInWorkspace(ctx, []string{root, projectPath}, wtPath, slug)

	logx.For("worktreesvc").Info("worktree created",
		"repo", root, "worktree", wtPath, "branch", slug,
		"ignoreCommitted", committed, "workspaceId", wsID)
	return CreateResult{
		WorktreePath:    wtPath,
		Branch:          slug,
		RepoRoot:        root,
		IgnoreCommitted: committed,
		WorkspaceID:     wsID,
	}, nil
}

// subPattern 是「整目录已被跟踪」时的退化 ignore 单元：只忽略 XK-Pane 运行时实际
// 写入的产物路径，不动仓库有意入库的资产。probe 给 check-ignore（glob pattern 用一个
// 具体示例路径，目录型 pattern 探目录内虚拟文件）；trackSpec 给 ls-files（产物本身也
// 被入库——如本仓 bundled skills——则尊重现状跳过）。
type subPattern struct {
	pattern   string // 追加进 .gitignore 的行
	probe     string // check-ignore 探针路径
	trackSpec string // ls-files pathspec（默认 wildmatch，* 可跨层）
}

// trackedDirSubPatterns 按工具目录列出 projecthooks.Sync / skills 同步的固定写入清单
// （来源：projecthooks/claude.go settings.local.json、codex.go config.toml、skills.go
// 两侧 skills/<xkpane-*>/SKILL.md + skills/.xkpane-skills-version）。.xkpanes（本地历史）
// 无固定子清单：整目录被跟踪时只能整体跳过。
var trackedDirSubPatterns = map[string][]subPattern{
	".claude": {
		{".claude/settings.local.json", ".claude/settings.local.json", ".claude/settings.local.json"},
		{".claude/skills/xkpane-*/", ".claude/skills/xkpane-probe/SKILL.md", ".claude/skills/xkpane-*"},
		{".claude/skills/.xkpane-skills-version", ".claude/skills/.xkpane-skills-version", ".claude/skills/.xkpane-skills-version"},
	},
	".codex": {
		{".codex/config.toml", ".codex/config.toml", ".codex/config.toml"},
		{".codex/skills/xkpane-*/", ".codex/skills/xkpane-probe/SKILL.md", ".codex/skills/xkpane-*"},
		{".codex/skills/.xkpane-skills-version", ".codex/skills/.xkpane-skills-version", ".codex/skills/.xkpane-skills-version"},
	},
}

// ensureToolDirsIgnored 确保三个工具目录被 git 忽略：逐个 check-ignore（综合所有
// ignore 来源），全部已忽略 → 直接返回；有缺失 → 追加进 <root>/.gitignore 并以
// pathspec 限定提交一次。返回"本次是否发生了提交"。
//
// 三段守门（对抗审查实证过的边界）：
//   - 目录下已有**被跟踪文件**（如本仓自己的 .claude/skills/**）→ 不 ignore 整目录
//     （gitignore 不影响已跟踪文件，追加只会让用户后续新增文件被 git add 静默漏收），
//     退化为只 ignore XK-Pane 运行时产物（trackedDirSubPatterns），产物本身也入库的再跳过；
//   - 用户对 .gitignore 自身有未提交/已暂存改动 → 明确报错（而非把 WIP 静默卷进
//     自动提交 / 覆盖暂存版）。
func (s *Service) ensureToolDirsIgnored(ctx context.Context, repo *gitx.Repo, root string) (bool, error) {
	var missing []string
	for _, dir := range toolDirs {
		// 探测「目录内的虚拟文件」而非目录本身：目录型 pattern（".claude/"）对磁盘上
		// 尚不存在的裸路径 ".claude" 不命中（git 无法判定其为目录），但目录下的任意
		// 路径必命中；同时也兼容 ".claude"（无斜杠）这类用户手写 pattern。
		ignored, err := repo.IsPathIgnored(ctx, dir+"/.xkpane-probe")
		if err != nil {
			return false, err
		}
		if ignored {
			continue
		}
		tracked, err := repo.HasTrackedFiles(ctx, dir)
		if err != nil {
			return false, err
		}
		if !tracked {
			missing = append(missing, dir+"/")
			continue
		}
		// 该目录内容是仓库有意入库的资产 → 退化为文件级 pattern（无清单的 .xkpanes 整体跳过）。
		subs := trackedDirSubPatterns[dir]
		if len(subs) == 0 {
			logx.For("worktreesvc").Info("skip auto-ignore: dir has tracked files", "repo", root, "dir", dir)
			continue
		}
		for _, sp := range subs {
			ignored, err := repo.IsPathIgnored(ctx, sp.probe)
			if err != nil {
				return false, err
			}
			if ignored {
				continue
			}
			tracked, err := repo.HasTrackedFiles(ctx, sp.trackSpec)
			if err != nil {
				return false, err
			}
			if tracked {
				// 产物本身被仓库入库（如 XK-Pane 自己跟踪 bundled skills）→ 尊重现状。
				logx.For("worktreesvc").Info("skip auto-ignore: artifact tracked", "repo", root, "path", sp.trackSpec)
				continue
			}
			missing = append(missing, sp.pattern)
		}
	}
	if len(missing) == 0 {
		return false, nil
	}

	// .gitignore 自身有未提交改动（工作树或暂存区）→ 拒绝自动提交：避免把用户 WIP
	// 卷进 "chore:" 提交、或用工作树版覆盖暂存版。文案英文（人类可见错误归前端 i18n）。
	dirty, err := repo.PathHasChanges(ctx, ".gitignore")
	if err != nil {
		return false, err
	}
	if dirty {
		return false, errors.New("worktreesvc: .gitignore has uncommitted changes; commit or stash it before creating an isolated worktree")
	}

	giPath := filepath.Join(root, ".gitignore")
	existing, err := os.ReadFile(giPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	var b strings.Builder
	b.Write(existing)
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		b.WriteByte('\n')
	}
	b.WriteString("\n# XK-Pane tool dirs (session hooks / injected skills / local history)\n")
	for _, p := range missing {
		b.WriteString(p + "\n")
	}
	if err := filex.WriteAtomic(giPath, []byte(b.String()), 0o644); err != nil {
		return false, err
	}
	// 先 stage（.gitignore 可能是全新未跟踪文件，pathspec 提交只认 git 已知路径），
	// 再以 pathspec 限定提交——用户已暂存的其它内容原样留在暂存区。
	if err := repo.Stage(ctx, []string{".gitignore"}); err != nil {
		return false, err
	}
	if _, err := repo.CommitPaths(ctx, ignoreCommitMessage, []string{".gitignore"}); err != nil {
		return false, err
	}
	return true, nil
}

// ResolveRepoRoot 返回 projectPath 所在 git 仓库的根目录（只读廉价探测，不做任何写
// 操作）。供前端在建树前确认实际目标仓库：projectPath 可能只是更上层仓库里的子目录
// （.git 向上探测），自动 .gitignore 提交会落到那个根上，应让用户知情确认。非 git
// 仓库时报错（与 CreateForProject 同口径）。
func (s *Service) ResolveRepoRoot(ctx context.Context, projectPath string) (string, error) {
	if strings.TrimSpace(projectPath) == "" {
		return "", errors.New("worktreesvc: project path required")
	}
	repo, err := gitx.Open(projectPath)
	if err != nil {
		return "", err
	}
	return repo.TopLevel(ctx)
}

// UnregisterWorktree 删除 workspace_projects 中指向 worktreePath 的登记行（GitPage
// 手动删树后调用，避免侧栏留下指向已不存在目录的死行）。路径即所有权：仅当
// worktreePath 位于本服务集中根（<UserDataDir>/worktrees）之下才动手——普通项目 /
// 用户自建 worktree 静默返回 false。找不到登记行不算错（幂等）。返回是否删了行。
func (s *Service) UnregisterWorktree(ctx context.Context, worktreePath string) (bool, error) {
	if s.workspaces == nil || strings.TrimSpace(worktreePath) == "" {
		return false, nil
	}
	base := s.worktreesRoot
	if base == "" {
		dataDir, err := storage.UserDataDir()
		if err != nil {
			return false, err
		}
		base = filepath.Join(dataDir, "worktrees")
	}
	normPath := normalizePath(worktreePath)
	if !strings.HasPrefix(normPath, normalizePath(base)+"/") {
		return false, nil
	}
	wss, err := s.workspaces.List(ctx)
	if err != nil {
		return false, err
	}
	removed := false
	for _, ws := range wss {
		for _, p := range ws.Projects {
			if normalizePath(p.Path) != normPath {
				continue
			}
			if err := s.workspaces.RemoveProject(ctx, p.ID); err != nil {
				return removed, err
			}
			removed = true
		}
	}
	if removed {
		logx.For("worktreesvc").Info("worktree unregistered", "path", worktreePath)
	}
	return removed, nil
}

// groupDirFor 返回该仓库的 worktree 分组目录：<root>/worktrees/<hash8>。
// hash8 = 主仓根归一化路径的 SHA-256 前 8 hex（paseo 同款"项目身份编码进路径"，
// 隔离不同项目的同名 slug；归一化吃掉大小写/分隔符/符号链接差异保证稳定）。
func (s *Service) groupDirFor(repoRoot string) (string, error) {
	base := s.worktreesRoot
	if base == "" {
		dataDir, err := storage.UserDataDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(dataDir, "worktrees")
	}
	norm := repoRoot
	if resolved, err := filepath.EvalSymlinks(repoRoot); err == nil {
		norm = resolved
	}
	norm = strings.ReplaceAll(norm, "\\", "/")
	if runtime.GOOS == "windows" {
		norm = strings.ToLower(norm)
	}
	sum := sha256.Sum256([]byte(norm))
	return filepath.Join(base, hex.EncodeToString(sum[:4])), nil
}

// pickSlug 选一个目录与本地分支都未占用的 slug；8 次随机碰壁后退化为「slug-4hex」。
// 分支表走 CLI（BranchNamesCLI）：go-git 版 Branches() 在 linked worktree 内打开时
// 看不到共享 refs（恒空表），会让占用检测失效、撞已有分支名。
func (s *Service) pickSlug(ctx context.Context, repo *gitx.Repo, groupDir string) (slug, wtPath string, err error) {
	branches, err := repo.BranchNamesCLI(ctx)
	if err != nil {
		return "", "", err
	}
	taken := make(map[string]bool, len(branches))
	for _, name := range branches {
		taken[name] = true
	}
	candidate := func(name string) (string, bool) {
		p := filepath.Join(groupDir, name)
		if taken[name] {
			return "", false
		}
		if _, statErr := os.Stat(p); statErr == nil {
			return "", false
		}
		return p, true
	}
	for try := 0; try < 8; try++ {
		name := randomSlug()
		if p, ok := candidate(name); ok {
			return name, p, nil
		}
	}
	name := randomSlug() + "-" + randomHexSuffix()
	p, ok := candidate(name)
	if !ok {
		return "", "", errors.New("worktreesvc: cannot find available worktree name")
	}
	return name, p, nil
}

// registerInWorkspace 把 worktree 登记进「包含任一候选路径（仓库根 / 用户登记的
// 子目录）」的第一个 workspace（归一化精确匹配，口径对齐 workspace service 的去重
// 比较）。alias = slug，便于侧栏一眼区分。任何失败仅记日志返回 ""（降级不阻断）。
func (s *Service) registerInWorkspace(ctx context.Context, candidates []string, wtPath, slug string) string {
	if s.workspaces == nil {
		return ""
	}
	wss, err := s.workspaces.List(ctx)
	if err != nil {
		logx.For("worktreesvc").Warn("list workspaces failed; skip register", "err", err)
		return ""
	}
	norms := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		if c != "" {
			norms[normalizePath(c)] = true
		}
	}
	for _, ws := range wss {
		for _, p := range ws.Projects {
			if !norms[normalizePath(p.Path)] {
				continue
			}
			wp, err := s.workspaces.AddProject(ctx, ws.ID, wtPath)
			if err != nil {
				logx.For("worktreesvc").Warn("register worktree failed", "workspace", ws.ID, "err", err)
				return ""
			}
			alias := slug
			if err := s.workspaces.SetProjectAlias(ctx, wp.ID, &alias); err != nil {
				logx.For("worktreesvc").Warn("set worktree alias failed", "err", err)
			}
			return ws.ID
		}
	}
	return ""
}

// normalizePath 与 workspace service 的 normalizeProjectPath 同口径（统一斜杠 +
// 去尾斜杠 + Windows 转小写），保证「主仓在不在该 workspace」的判定一致。
func normalizePath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimRight(p, "/")
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}
