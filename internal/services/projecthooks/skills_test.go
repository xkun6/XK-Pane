package projecthooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wantSkillCount 是内嵌 bundled skill 的数量（templates/skills 下 10 个 xkpane-*）。
const wantSkillCount = 10

func TestSyncClaudeSkillsLandsAllSkills(t *testing.T) {
	dir := t.TempDir()
	if err := syncClaudeSkills(dir); err != nil {
		t.Fatalf("syncClaudeSkills: %v", err)
	}
	assertSkillsLanded(t, filepath.Join(dir, ".claude", "skills"))
}

func TestSyncCodexSkillsLandsAllSkills(t *testing.T) {
	dir := t.TempDir()
	if err := syncCodexSkills(dir); err != nil {
		t.Fatalf("syncCodexSkills: %v", err)
	}
	assertSkillsLanded(t, filepath.Join(dir, ".codex", "skills"))
}

// assertSkillsLanded 校验 root 下落了全部 9 个 xkpane-* skill：每个含 SKILL.md，frontmatter 的
// name 与目录名一致；并校验版本标记文件存在且为当前版本。
func assertSkillsLanded(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read skills root %s: %v", root, err)
	}
	var count int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !strings.HasPrefix(e.Name(), "xkpane-") {
			t.Errorf("意外 skill 目录名（应 xkpane- 前缀）: %s", e.Name())
		}
		md := filepath.Join(root, e.Name(), "SKILL.md")
		data, err := os.ReadFile(md)
		if err != nil {
			t.Errorf("读 %s: %v", md, err)
			continue
		}
		if want := "---\nname: " + e.Name(); !strings.HasPrefix(string(data), want) {
			t.Errorf("%s frontmatter name 应等于目录名 %s", md, e.Name())
		}
		count++
	}
	if count != wantSkillCount {
		t.Errorf("落盘 skill 数 = %d, 期望 %d", count, wantSkillCount)
	}

	marker := filepath.Join(root, skillsVersionMarker)
	mv, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("读版本标记 %s: %v", marker, err)
	}
	if got := strings.TrimSpace(string(mv)); got != projectSkillsVersion {
		t.Errorf("marker 版本 = %q, 期望 %q", got, projectSkillsVersion)
	}
}

// TestSyncProjectSkillsIdempotent 验证版本标记一致时整体跳过（不重写已有文件）。
func TestSyncProjectSkillsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := syncClaudeSkills(dir); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	probe := filepath.Join(dir, ".claude", "skills", "xkpane-launch-task", "SKILL.md")
	if err := os.WriteFile(probe, []byte("TAMPERED"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncClaudeSkills(dir); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	data, err := os.ReadFile(probe)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "TAMPERED" {
		t.Errorf("版本未变时应跳过重写，但探针文件被改回了（幂等失效）")
	}
}
