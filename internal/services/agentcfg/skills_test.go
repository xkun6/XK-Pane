package agentcfg

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// writeSkillDir 在 dir/id 下创建一个最小 skill 目录：SKILL.md（含 frontmatter）+
// 一个附属文件，用于验证递归复制不丢文件。
func writeSkillDir(t *testing.T, dir, id, name, description string) string {
	t.Helper()
	skillDir := filepath.Join(dir, id)
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	skillMd := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMd), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "scripts", "run.py"), []byte("print('hi')\n"), 0o644); err != nil {
		t.Fatalf("write run.py: %v", err)
	}
	return skillDir
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ---- 5. ImportSkillDir → SetEnabled(claude) 出现 → 关闭消失 → Delete 挪进回收目录 ----

func TestSkillImportEnableDisableDelete(t *testing.T) {
	svc := newTestService(t)
	srcRoot := t.TempDir()
	srcDir := writeSkillDir(t, srcRoot, "demo-skill", "Demo Skill", "一个示例 skill")

	ctx := context.Background()
	dto, err := svc.ImportSkillDir(ctx, srcDir)
	if err != nil {
		t.Fatalf("ImportSkillDir: %v", err)
	}
	if dto.Name != "Demo Skill" || dto.Description != "一个示例 skill" {
		t.Fatalf("frontmatter 解析不对: %+v", dto)
	}
	ssotScript := filepath.Join(svc.skillSSOTDir("demo-skill"), "scripts", "run.py")
	if !fileExists(ssotScript) {
		t.Fatal("SSOT 中未出现附属文件")
	}

	if _, err := svc.SetAgentSkillEnabled(ctx, "demo-skill", true, false); err != nil {
		t.Fatalf("SetAgentSkillEnabled(claude=true): %v", err)
	}
	claudeScript := filepath.Join(svc.claudeSkillDir("demo-skill"), "scripts", "run.py")
	if !fileExists(claudeScript) {
		t.Fatal("启用 claude 侧后附属文件应出现在 ~/.claude/skills/demo-skill/")
	}

	if _, err := svc.SetAgentSkillEnabled(ctx, "demo-skill", false, false); err != nil {
		t.Fatalf("SetAgentSkillEnabled(claude=false): %v", err)
	}
	if fileExists(claudeScript) {
		t.Fatal("关闭 claude 侧后目录应消失")
	}

	if err := svc.DeleteAgentSkill(ctx, "demo-skill"); err != nil {
		t.Fatalf("DeleteAgentSkill: %v", err)
	}
	if fileExists(svc.skillSSOTDir("demo-skill")) {
		t.Fatal("Delete 后 SSOT 目录应挪走")
	}
	entries, err := os.ReadDir(svc.agentSkillsTrashDir())
	if err != nil {
		t.Fatalf("read trash dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("回收目录应有 1 项, got %d", len(entries))
	}
	list, err := svc.ListAgentSkills(ctx)
	if err != nil {
		t.Fatalf("ListAgentSkills: %v", err)
	}
	for _, row := range list {
		if row.ID == "demo-skill" {
			t.Fatal("Delete 后 SSOT 行应消失")
		}
	}
}

// ---- 6. ImportSkillsFromApps：frontmatter 解析正确 + 幂等 ----

func TestImportSkillsFromApps(t *testing.T) {
	svc := newTestService(t)
	claudeSkillsDir := filepath.Join(svc.home, ".claude", "skills")
	writeSkillDir(t, claudeSkillsDir, "foo", "Foo Skill", "foo 的描述")

	ctx := context.Background()
	n, err := svc.ImportSkillsFromApps(ctx)
	if err != nil {
		t.Fatalf("ImportSkillsFromApps: %v", err)
	}
	if n != 1 {
		t.Fatalf("应新增 1 行, got %d", n)
	}

	list, err := svc.ListAgentSkills(ctx)
	if err != nil {
		t.Fatalf("ListAgentSkills: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	row := list[0]
	if row.ID != "foo" || row.Name != "Foo Skill" || row.Description != "foo 的描述" {
		t.Fatalf("导入行不对: %+v", row)
	}
	if !row.EnabledClaude || row.EnabledCodex {
		t.Fatalf("应只启用 claude 侧: %+v", row)
	}

	n2, err := svc.ImportSkillsFromApps(ctx)
	if err != nil {
		t.Fatalf("ImportSkillsFromApps(第二次): %v", err)
	}
	if n2 != 0 {
		t.Fatalf("重复导入应为 0, got %d", n2)
	}
}

// ---- 7. 非法 id（含 .. / 路径分隔符 / 非法字符）在 Import/SetEnabled/Delete 全部被拒 ----

func TestSkillIllegalIDRejected(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	if _, err := svc.SetAgentSkillEnabled(ctx, "..", true, false); err == nil {
		t.Fatal("SetAgentSkillEnabled(..) 应被拒")
	}
	if err := svc.DeleteAgentSkill(ctx, "../escape"); err == nil {
		t.Fatal("DeleteAgentSkill(路径分隔符) 应被拒")
	}

	srcRoot := t.TempDir()
	badDir := writeSkillDir(t, srcRoot, "bad name", "Bad", "desc") // 空格不在合法字符集内
	if _, err := svc.ImportSkillDir(ctx, badDir); err == nil {
		t.Fatal("ImportSkillDir(非法 id) 应被拒")
	}
}
