package projecthooks

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"xkpane/internal/utils/filex"
)

// skillTemplates 内嵌 templates/skills/<name>/SKILL.md 全树。这些 bundled skill 移植自
// cc-pane / ccpanes 的 13 个 skill 中 panes MCP 工具齐备的 9 个，改写为 XK-Pane 版，在
// 【被启动项目】里同时落到 .claude/skills/ 与 .codex/skills/，供项目内 Claude/Codex 子实例使用。
//
// 文件名全 ASCII（xkpane-* 目录 + SKILL.md），无需像 bedrock 那样走 zip 绕开 embed 的 CJK 文件名
// 限制；内容是 UTF-8 中文，embed 对文件【内容】无字符限制。
//
//go:embed all:templates/skills
var skillTemplates embed.FS

const skillTemplatesRoot = "templates/skills"

// projectSkillsVersion 是 bundled skill 内容版本。bump 它会让已部署项目在下次启动会话时整体重写
// （内容有更新时务必 +1）。版本写到目标根的 .xkpane-skills-version 做幂等比对，避免每次启动都重写
// 9×2 个文件。
const projectSkillsVersion = "1"

const skillsVersionMarker = ".xkpane-skills-version"

// syncClaudeSkills 把内嵌 xkpane-* skill 落到 <proj>/.claude/skills/<name>/SKILL.md
// （Claude Code 项目级 skills 目录）。
func syncClaudeSkills(projectPath string) error {
	return syncProjectSkills(filepath.Join(projectPath, ".claude", "skills"))
}

// syncCodexSkills 把内嵌 xkpane-* skill 落到 <proj>/.codex/skills/<name>/SKILL.md。
// codex 项目级 skills 路径据 codex 源码 core-skills/src/loader.rs 确认：Project 配置层的
// config_folder 即 <proj>/.codex，config_folder.join("skills") → <proj>/.codex/skills（Repo scope）。
// frontmatter 与 claude 同构（仅 name≤64 / description≤1024B，无其它约束），故两侧共用同一份 SKILL.md。
func syncCodexSkills(projectPath string) error {
	return syncProjectSkills(filepath.Join(projectPath, ".codex", "skills"))
}

// syncProjectSkills 把内嵌 skill 树写到 destRoot。版本标记一致则整体跳过（幂等）；否则逐个
// filex.WriteAtomic 写出并刷新标记。失败返回 error（调用方 Sync 仅 warn，不阻断会话启动）。
func syncProjectSkills(destRoot string) error {
	// 幂等：标记文件版本与当前一致即跳过（不重写）。
	markerPath := filepath.Join(destRoot, skillsVersionMarker)
	if data, err := os.ReadFile(markerPath); err == nil &&
		strings.TrimSpace(string(data)) == projectSkillsVersion {
		return nil
	}

	entries, err := fs.ReadDir(skillTemplates, skillTemplatesRoot)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// embed.FS 路径分隔符恒为 '/'，不受 OS 影响。
		content, err := skillTemplates.ReadFile(skillTemplatesRoot + "/" + name + "/SKILL.md")
		if err != nil {
			return err
		}
		dst := filepath.Join(destRoot, name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := filex.WriteAtomic(dst, content, 0o644); err != nil {
			return err
		}
	}
	return filex.WriteAtomic(markerPath, []byte(projectSkillsVersion+"\n"), 0o644)
}
