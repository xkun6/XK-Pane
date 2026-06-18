package gitsvc

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"xkpane/internal/localhistory"
	"xkpane/internal/utils/gitx"
)

// VSCode 源代码管理面板的 service 方法。设计同 gitsvc 其余部分：按 path 现开仓库 → 调 gitx
// 原语 → 收口 DTO。diff 复用 localhistory.DiffResult（domain 纯数据类型，前端 DiffView 同款），
// 故 service→domain import 合法、且前端零新 diff 模型。

// StatusFiles 返回结构化变更文件列表（SCM 三段式：staged / changes / untracked 由 flags 分组）。
// 非仓库透传 ErrNotARepo（UI 仅在 IsRepo 时调用）。
func (s *Service) StatusFiles(ctx context.Context, path string) ([]gitx.FileStatus, error) {
	repo, err := gitx.Open(path)
	if err != nil {
		return nil, err
	}
	return repo.StatusFiles(ctx)
}

// RepoRoot 返回 path 所在 Git 仓库的工作树根（绝对路径）。非仓库透传 ErrNotARepo。
// 供文件树 Git 状态装饰把 StatusFiles 的仓库相对路径还原为绝对路径与树节点比对。
func (s *Service) RepoRoot(ctx context.Context, path string) (string, error) {
	repo, err := gitx.Open(path)
	if err != nil {
		return "", err
	}
	return repo.TopLevel(ctx)
}

// DiffFile 返回单文件 diff（staged 决定暂存区/工作树侧），解析为 localhistory.DiffResult
// 复用前端 DiffView。行级 diff（无字符级 inline，对齐 VSCode 默认）；二进制 → IsBinary。
func (s *Service) DiffFile(ctx context.Context, path, file string, staged bool) (localhistory.DiffResult, error) {
	repo, err := gitx.Open(path)
	if err != nil {
		return localhistory.DiffResult{}, err
	}
	raw, err := repo.DiffFileRaw(ctx, file, staged)
	if err != nil {
		return localhistory.DiffResult{}, err
	}
	return parseUnifiedDiff(raw), nil
}

// DiffFileAbs 按「文件绝对路径」返回其工作树相对 HEAD 的 diff（供文件页就地查看改动）。
// 自行解析该文件所属仓库根与相对路径，免去前端拼相对路径。非仓库透传 ErrNotARepo。
func (s *Service) DiffFileAbs(ctx context.Context, absPath string) (localhistory.DiffResult, error) {
	repo, err := gitx.Open(filepath.Dir(absPath))
	if err != nil {
		return localhistory.DiffResult{}, err
	}
	root, err := repo.TopLevel(ctx)
	if err != nil {
		return localhistory.DiffResult{}, err
	}
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return localhistory.DiffResult{}, err
	}
	raw, err := repo.DiffFileRaw(ctx, filepath.ToSlash(rel), false)
	if err != nil {
		return localhistory.DiffResult{}, err
	}
	return parseUnifiedDiff(raw), nil
}

// Stage 把文件加入暂存区。非仓库透传 ErrNotARepo。
func (s *Service) Stage(ctx context.Context, path string, files []string) error {
	repo, err := gitx.Open(path)
	if err != nil {
		return err
	}
	return repo.Stage(ctx, files)
}

// Unstage 撤销暂存（git restore --staged）。需仓库已有 ≥1 次提交（见 gitx.Unstage 备注）。
func (s *Service) Unstage(ctx context.Context, path string, files []string) error {
	repo, err := gitx.Open(path)
	if err != nil {
		return err
	}
	return repo.Unstage(ctx, files)
}

// Discard 丢弃改动（破坏性，前端须先 NativeDialogs.confirm）：已跟踪走 git restore 撤回工作树
// 修改/删除；未跟踪直接删除工作树文件（git restore 对未跟踪无效）。按当前 status 区分二者，
// 不信任前端传入的分类（防误删）。
func (s *Service) Discard(ctx context.Context, path string, files []string) error {
	repo, err := gitx.Open(path)
	if err != nil {
		return err
	}
	statuses, err := repo.StatusFiles(ctx)
	if err != nil {
		return err
	}
	untracked := make(map[string]bool)
	for _, st := range statuses {
		if st.Untracked {
			untracked[st.Path] = true
		}
	}
	var tracked []string
	for _, f := range files {
		if untracked[f] {
			// 未跟踪文件：删除工作树副本（忽略不存在错误）。
			_ = os.Remove(filepath.Join(path, filepath.FromSlash(f)))
		} else {
			tracked = append(tracked, f)
		}
	}
	return repo.RestoreWorktree(ctx, tracked)
}

// Commit 提交暂存区内容（git commit -m）；提交前打 Local History 标签（对齐 cc-pane
// auto_label_before_git）。无暂存内容 / 空消息时 git 报错透传，前端经 NativeDialogs 提示。
func (s *Service) Commit(ctx context.Context, path, message string) (string, error) {
	repo, err := gitx.Open(path)
	if err != nil {
		return "", err
	}
	s.labelBefore(ctx, repo, path, "Before Commit", "git_commit")
	return repo.Commit(ctx, message)
}

// ── unified diff 解析（git patch → localhistory.DiffResult，行级）────────────────

// parseUnifiedDiff 把 `git diff` 的 unified patch 解析为 DiffResult。
// 只取 @@ hunk 与其中的 +/-/空格 行；文件头（diff --git / index / --- / +++ / mode）跳过；
// "Binary files ... differ" → IsBinary；"\ No newline at end of file" 忽略。
// 行级 diff 不含字符级 inline（DiffView 对 inlineChanges 缺省即整行高亮，无碍）。
func parseUnifiedDiff(patch string) localhistory.DiffResult {
	res := localhistory.DiffResult{Hunks: []localhistory.DiffHunk{}}
	if patch == "" {
		return res
	}

	var cur *localhistory.DiffHunk
	oldNo, newNo := 0, 0
	flush := func() {
		if cur != nil {
			res.Hunks = append(res.Hunks, *cur)
			cur = nil
		}
	}

	for _, ln := range strings.Split(patch, "\n") {
		if strings.HasPrefix(ln, "Binary files ") {
			flush()
			res.IsBinary = true
			res.Hunks = []localhistory.DiffHunk{}
			return res
		}
		if strings.HasPrefix(ln, "@@") {
			flush()
			oldStart, _, newStart, _, ok := parseHunkHeader(ln)
			if !ok {
				continue
			}
			cur = &localhistory.DiffHunk{OldStart: oldStart, NewStart: newStart, Lines: []localhistory.DiffLine{}}
			oldNo, newNo = oldStart, newStart
			continue
		}
		if cur == nil || ln == "" {
			continue // 文件头 / 空尾行
		}
		switch ln[0] {
		case ' ':
			o, n := oldNo, newNo
			cur.Lines = append(cur.Lines, localhistory.DiffLine{
				ChangeType: localhistory.ChangeEqual, Content: ln[1:], OldLineNo: &o, NewLineNo: &n,
			})
			cur.OldCount++
			cur.NewCount++
			oldNo++
			newNo++
		case '+':
			n := newNo
			cur.Lines = append(cur.Lines, localhistory.DiffLine{
				ChangeType: localhistory.ChangeInsert, Content: ln[1:], NewLineNo: &n,
			})
			cur.NewCount++
			newNo++
			res.Stats.Additions++
		case '-':
			o := oldNo
			cur.Lines = append(cur.Lines, localhistory.DiffLine{
				ChangeType: localhistory.ChangeDelete, Content: ln[1:], OldLineNo: &o,
			})
			cur.OldCount++
			oldNo++
			res.Stats.Deletions++
		case '\\':
			// "\ No newline at end of file" —— 忽略
		}
	}
	flush()
	res.Stats.Changes = min(res.Stats.Additions, res.Stats.Deletions)
	return res
}

// parseHunkHeader 解析 "@@ -oldStart[,oldCount] +newStart[,newCount] @@ [ctx]"。
func parseHunkHeader(h string) (oldStart, oldCount, newStart, newCount int, ok bool) {
	if !strings.HasPrefix(h, "@@ ") {
		return 0, 0, 0, 0, false
	}
	rest := h[3:]
	end := strings.Index(rest, " @@")
	if end < 0 {
		return 0, 0, 0, 0, false
	}
	parts := strings.Fields(rest[:end]) // ["-12,7", "+12,8"]
	if len(parts) != 2 {
		return 0, 0, 0, 0, false
	}
	oldStart, oldCount = parseHunkRange(parts[0])
	newStart, newCount = parseHunkRange(parts[1])
	return oldStart, oldCount, newStart, newCount, true
}

// parseHunkRange 解析 "-12,7" / "+12" 形式（去前导符号，count 缺省=1）。
func parseHunkRange(s string) (start, count int) {
	if len(s) > 0 && (s[0] == '-' || s[0] == '+') {
		s = s[1:]
	}
	count = 1
	if i := strings.IndexByte(s, ','); i >= 0 {
		start, _ = strconv.Atoi(s[:i])
		count, _ = strconv.Atoi(s[i+1:])
	} else {
		start, _ = strconv.Atoi(s)
	}
	return start, count
}
