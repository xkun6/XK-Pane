package agentcfg

// 本文件实现 Skill 的 SSOT + 投影：agent_skills 表 + <dataDir>/agent-skills/<id>/ 目录
// 是单一事实来源，按 EnabledClaude / EnabledCodex 开关把整棵目录复制投影到
// <home>/.claude/skills/<id>/ 与 <home>/.codex/skills/<id>/。「禁用」= 从对应 app
// 目录移除但 SSOT 保留；「删除」= 两侧 app 目录移除 + SSOT 目录挪进回收目录 + 行删除。
//
// id 铁律：必须匹配 skillIDPattern 且不等于 "."/".."——所有 RemoveAll / Rename 都只
// 作用于 <白名单目录>/<合法 id>，任何非法 id 在触碰文件系统之前就被拒绝，防止路径逃逸。

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"xkpane/internal/storage"

	"gorm.io/gorm"
)

var errSkillNotFound = errors.New("agentcfg: skill does not exist")

// skillIDPattern 是 Skill id（同时也是 SSOT / 两侧 app 目录名）的合法字符集。
var skillIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// validateSkillID 除字符集校验外，额外拒绝 "." 与 ".."——这两个值虽然满足字符集
// （'.' 本身是允许字符），但作为目录名会被 filepath.Join 解释成「当前目录」/
// 「上级目录」，是唯一可能让后续 RemoveAll / Rename 逃出白名单目录的输入。
func validateSkillID(id string) error {
	if !skillIDPattern.MatchString(id) {
		return fmt.Errorf("agentcfg: skill id 不合法（仅允许字母数字 . _ -，且非空）: %q", id)
	}
	if id == "." || id == ".." {
		return fmt.Errorf("agentcfg: skill id 不合法: %q", id)
	}
	return nil
}

// agentSkillsDir 是 SSOT 目录根：<dataDir>/agent-skills/。
func (s *Service) agentSkillsDir() string {
	return filepath.Join(s.dataDir, "agent-skills")
}

// agentSkillsTrashDir 是删除后的回收目录根：<dataDir>/agent-skills-trash/。
func (s *Service) agentSkillsTrashDir() string {
	return filepath.Join(s.dataDir, "agent-skills-trash")
}

// skillSSOTDir 是某个 skill 的 SSOT 目录：<dataDir>/agent-skills/<id>/。
func (s *Service) skillSSOTDir(id string) string {
	return filepath.Join(s.agentSkillsDir(), id)
}

// claudeSkillDir 是 Claude 侧 app 目录：<home>/.claude/skills/<id>/。
func (s *Service) claudeSkillDir(id string) string {
	return filepath.Join(s.home, ".claude", "skills", id)
}

// codexSkillDir 是 Codex 侧 app 目录：<home>/.codex/skills/<id>/。
func (s *Service) codexSkillDir(id string) string {
	return filepath.Join(s.home, ".codex", "skills", id)
}

// AgentSkillDTO 是前端可见视图。
type AgentSkillDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	EnabledClaude bool      `json:"enabledClaude"`
	EnabledCodex  bool      `json:"enabledCodex"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func skillToDTO(row storage.AgentSkill) AgentSkillDTO {
	return AgentSkillDTO{
		ID:            row.ID,
		Name:          row.Name,
		Description:   row.Description,
		EnabledClaude: row.EnabledClaude,
		EnabledCodex:  row.EnabledCodex,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

// ListAgentSkills 按 Name 排序列出全部 Skill。
func (s *Service) ListAgentSkills(ctx context.Context) ([]AgentSkillDTO, error) {
	var rows []storage.AgentSkill
	if err := s.db().WithContext(ctx).Order("name").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AgentSkillDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, skillToDTO(row))
	}
	return out, nil
}

// ImportSkillDir 把 srcPath 指向的一个 skill 目录导入 SSOT（两开关均置 false，导入后
// 需要用户显式启用才会投影到任何一侧 app 目录）。srcPath 必须含 SKILL.md；id 取
// filepath.Base(srcPath)。
func (s *Service) ImportSkillDir(ctx context.Context, srcPath string) (*AgentSkillDTO, error) {
	srcPath = filepath.Clean(srcPath)
	skillMdPath := filepath.Join(srcPath, "SKILL.md")
	if _, err := os.Stat(skillMdPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("agentcfg: %s 不含 SKILL.md，不是合法的 skill 目录", srcPath)
		}
		return nil, fmt.Errorf("agentcfg: 检查 %s 失败: %w", skillMdPath, err)
	}

	id := filepath.Base(srcPath)
	if err := validateSkillID(id); err != nil {
		return nil, err
	}

	dest := s.skillSSOTDir(id)
	if _, err := os.Stat(dest); err == nil {
		return nil, fmt.Errorf("agentcfg: SSOT 中已存在同 id 的 skill: %s", id)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("agentcfg: 检查 %s 失败: %w", dest, err)
	}

	if err := s.copySkillTree(srcPath, dest); err != nil {
		_ = os.RemoveAll(dest)
		return nil, err
	}

	name, description := parseSkillFrontmatter(skillMdPath)
	if name == "" {
		name = id
	}

	row := storage.AgentSkill{ID: id, Name: name, Description: description}
	if err := s.db().WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	dto := skillToDTO(row)
	return &dto, nil
}

// SetAgentSkillEnabled 按新开关值同步两侧 app 目录（先对比 DB 现值算出每侧的
// 变化：false→true 从 SSOT 复制过去，true→false 清空目标目录，未变化 no-op），
// 最后更新行。
func (s *Service) SetAgentSkillEnabled(ctx context.Context, id string, enabledClaude, enabledCodex bool) (*AgentSkillDTO, error) {
	if err := validateSkillID(id); err != nil {
		return nil, err
	}
	var row storage.AgentSkill
	if err := s.db().WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errSkillNotFound
		}
		return nil, err
	}

	if err := s.syncSkillSide(id, row.EnabledClaude, enabledClaude, s.claudeSkillDir(id)); err != nil {
		return nil, err
	}
	if err := s.syncSkillSide(id, row.EnabledCodex, enabledCodex, s.codexSkillDir(id)); err != nil {
		return nil, err
	}

	if err := s.db().WithContext(ctx).Model(&storage.AgentSkill{}).Where("id = ?", id).
		Updates(map[string]any{"enabled_claude": enabledClaude, "enabled_codex": enabledCodex}).Error; err != nil {
		return nil, err
	}
	var updated storage.AgentSkill
	if err := s.db().WithContext(ctx).First(&updated, "id = ?", id).Error; err != nil {
		return nil, err
	}
	dto := skillToDTO(updated)
	return &dto, nil
}

// syncSkillSide 是 SetAgentSkillEnabled 单侧同步逻辑：was/now 相同 → no-op；
// true→false → 清空 targetDir；false→true → 先清空再从 SSOT 整棵复制过去（先清空
// 保证目标目录内容与 SSOT 精确一致，不残留上次启用时的旧文件）。
func (s *Service) syncSkillSide(id string, was, now bool, targetDir string) error {
	if was == now {
		return nil
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("agentcfg: 清空 %s 失败: %w", targetDir, err)
	}
	if !now {
		return nil
	}
	return s.copySkillTree(s.skillSSOTDir(id), targetDir)
}

// DeleteAgentSkill 删除一个 Skill：先清掉两侧 app 目录，再把 SSOT 目录整体挪进回收
// 目录（同盘 os.Rename；跨盘失败则复制+删除源目录兜底），最后删行。
func (s *Service) DeleteAgentSkill(ctx context.Context, id string) error {
	if err := validateSkillID(id); err != nil {
		return err
	}
	var row storage.AgentSkill
	if err := s.db().WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errSkillNotFound
		}
		return err
	}

	if err := os.RemoveAll(s.claudeSkillDir(id)); err != nil {
		return fmt.Errorf("agentcfg: 清理 claude 侧 skill 目录失败: %w", err)
	}
	if err := os.RemoveAll(s.codexSkillDir(id)); err != nil {
		return fmt.Errorf("agentcfg: 清理 codex 侧 skill 目录失败: %w", err)
	}

	ssot := s.skillSSOTDir(id)
	if _, statErr := os.Stat(ssot); statErr == nil {
		trashDir := filepath.Join(s.agentSkillsTrashDir(), fmt.Sprintf("%s-%d", id, time.Now().Unix()))
		if err := os.MkdirAll(s.agentSkillsTrashDir(), 0o755); err != nil {
			return fmt.Errorf("agentcfg: 创建回收目录失败: %w", err)
		}
		if err := os.Rename(ssot, trashDir); err != nil {
			// 跨盘 rename 失败兜底：复制到回收目录后删除原 SSOT 目录。
			if copyErr := s.copySkillTree(ssot, trashDir); copyErr != nil {
				return fmt.Errorf("agentcfg: 挪 SSOT 到回收目录失败: %w", copyErr)
			}
			if err := os.RemoveAll(ssot); err != nil {
				return fmt.Errorf("agentcfg: 删除原 SSOT 目录失败: %w", err)
			}
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("agentcfg: 检查 SSOT 目录失败: %w", statErr)
	}

	return s.db().WithContext(ctx).Delete(&storage.AgentSkill{}, "id = ?", id).Error
}

// ImportSkillsFromApps 扫 <home>/.claude/skills 与 <home>/.codex/skills 的一级子目录，
// 把尚未在 SSOT 中登记的 id 复制进 SSOT 并建行（该侧 Enabled=true）；已在 SSOT 中的
// id 只把对应侧 Enabled 置 true（目录不重复复制，SSOT 内容为准）。返回新增行数。
func (s *Service) ImportSkillsFromApps(ctx context.Context) (int, error) {
	added := 0
	claudeDir := filepath.Join(s.home, ".claude", "skills")
	codexDir := filepath.Join(s.home, ".codex", "skills")

	claudeIDs, err := s.scanAppSkillDir(claudeDir)
	if err != nil {
		return added, err
	}
	codexIDs, err := s.scanAppSkillDir(codexDir)
	if err != nil {
		return added, err
	}
	inClaude := make(map[string]bool, len(claudeIDs))
	for _, id := range claudeIDs {
		inClaude[id] = true
	}
	inCodex := make(map[string]bool, len(codexIDs))
	for _, id := range codexIDs {
		inCodex[id] = true
	}

	ids := make(map[string]bool, len(claudeIDs)+len(codexIDs))
	for id := range inClaude {
		ids[id] = true
	}
	for id := range inCodex {
		ids[id] = true
	}
	sorted := make([]string, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)

	for _, id := range sorted {
		var existing storage.AgentSkill
		err := s.db().WithContext(ctx).First(&existing, "id = ?", id).Error
		switch {
		case err == nil:
			updates := map[string]any{}
			if inClaude[id] && !existing.EnabledClaude {
				updates["enabled_claude"] = true
			}
			if inCodex[id] && !existing.EnabledCodex {
				updates["enabled_codex"] = true
			}
			if len(updates) > 0 {
				if err := s.db().WithContext(ctx).Model(&storage.AgentSkill{}).Where("id = ?", id).Updates(updates).Error; err != nil {
					return added, err
				}
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			srcDir := filepath.Join(claudeDir, id)
			if !inClaude[id] {
				srcDir = filepath.Join(codexDir, id)
			}
			dest := s.skillSSOTDir(id)
			if _, statErr := os.Stat(dest); statErr != nil {
				if !errors.Is(statErr, os.ErrNotExist) {
					return added, fmt.Errorf("agentcfg: 检查 %s 失败: %w", dest, statErr)
				}
				if err := s.copySkillTree(srcDir, dest); err != nil {
					return added, err
				}
			}
			name, description := parseSkillFrontmatter(filepath.Join(srcDir, "SKILL.md"))
			if name == "" {
				name = id
			}
			row := storage.AgentSkill{
				ID:            id,
				Name:          name,
				Description:   description,
				EnabledClaude: inClaude[id],
				EnabledCodex:  inCodex[id],
			}
			if err := s.db().WithContext(ctx).Create(&row).Error; err != nil {
				return added, err
			}
			added++
		default:
			return added, err
		}
	}
	return added, nil
}

// scanAppSkillDir 扫某个 app 侧 skills 目录的一级子目录，返回其中合法的 skill id 列表
// （须含 SKILL.md、id 合法字符集，否则跳过并记日志）。目录不存在 → 空列表，不报错。
func (s *Service) scanAppSkillDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("agentcfg: 读取 %s 失败: %w", dir, err)
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		if err := validateSkillID(id); err != nil {
			s.log.Warn("跳过非法 skill id", "dir", dir, "name", id, "err", err)
			continue
		}
		if _, statErr := os.Stat(filepath.Join(dir, id, "SKILL.md")); statErr != nil {
			s.log.Warn("跳过不含 SKILL.md 的目录", "dir", filepath.Join(dir, id))
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// copySkillTree 把 src 目录整棵递归复制到 dest（dest 及其父目录按需创建）。遇到符号
// 链接跳过并记日志——避免把宿主机上意外的符号链接跟随进 SSOT，造成路径逃逸或复制进
// 循环引用。
func (s *Service) copySkillTree(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dest, rel)

		if d.Type()&fs.ModeSymlink != 0 {
			s.log.Warn("skill 目录中存在符号链接，已跳过", "path", path)
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copySkillFile(path, target)
	})
}

// copySkillFile 复制单个文件并保留源文件权限位。skill 文件是数据文件（非应用配置），
// 直接用 os.WriteFile 即可，不必走 filex.WriteAtomic（那是给配置文件的原子写保护）。
func copySkillFile(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("agentcfg: 检查 %s 失败: %w", src, err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("agentcfg: 读取 %s 失败: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("agentcfg: 创建 %s 失败: %w", filepath.Dir(dest), err)
	}
	if err := os.WriteFile(dest, data, info.Mode().Perm()); err != nil {
		return fmt.Errorf("agentcfg: 写入 %s 失败: %w", dest, err)
	}
	return nil
}

// parseSkillFrontmatter 解析 SKILL.md 开头的 YAML frontmatter（--- 包裹的首段），
// 提取 name / description 两个字段的值。不引入完整 YAML 解析器——逐行按 "key:" 前缀
// 取值，去掉首尾空白与包裹引号即可满足 SKILL.md 的实际写法。解析失败或字段缺失 →
// 对应返回空字符串，调用方兜底成 id。
func parseSkillFrontmatter(path string) (name, description string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	lines := strings.Split(string(data), "\n")

	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			start = i
			break
		}
	}
	if start < 0 {
		return "", ""
	}
	end := -1
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return "", ""
	}

	for _, line := range lines[start+1 : end] {
		trimmed := strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(trimmed, "name:"); ok {
			name = unquoteYAMLValue(v)
		} else if v, ok := strings.CutPrefix(trimmed, "description:"); ok {
			description = unquoteYAMLValue(v)
		}
	}
	return name, description
}

// unquoteYAMLValue 去掉值前后空白，以及可能包裹的一对单/双引号。
func unquoteYAMLValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			v = v[1 : len(v)-1]
		}
	}
	return v
}
