package bedrock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 这些测试改动进程级 userDataRoot（全局）。Go 同包测试默认串行执行（未调 t.Parallel），
// 每个有状态用例开头 resetState() 清根 + 失效缓存，保证相互隔离。

func resetState() { SetDataRoot("") }

// ── 内嵌基线加载 ─────────────────────────────────────────────────────────

func TestLoadApis_AtLeast1000(t *testing.T) {
	resetState()
	v, err := loadApis()
	if err != nil {
		t.Fatalf("loadApis: %v", err)
	}
	if len(v) < 1000 {
		t.Fatalf("apis: 期望 >= 1000 条，得 %d", len(v))
	}
	// 首条应为 AddActorAnimation（顺序确定，json 保数组序）。
	if v[0].Name != "AddActorAnimation" || v[0].Kind != "api" || v[0].Side != "client" {
		t.Fatalf("首条 API 不符: name=%q kind=%q side=%q", v[0].Name, v[0].Kind, v[0].Side)
	}
}

func TestLoadComponents_NonEmpty(t *testing.T) {
	resetState()
	v, err := loadComponents()
	if err != nil {
		t.Fatalf("loadComponents: %v", err)
	}
	if len(v) == 0 {
		t.Fatal("components: 期望非空")
	}
	// 首条 minecraft:max_stack_size，且 Extra 兜住 data_type/properties/values/example。
	c := v[0]
	if c.ID != "minecraft:max_stack_size" || c.Kind != "item" {
		t.Fatalf("首条 component 不符: id=%q kind=%q", c.ID, c.Kind)
	}
	for _, k := range []string{"data_type", "properties", "values", "example"} {
		if _, ok := c.Extra[k]; !ok {
			t.Fatalf("component Extra 缺键 %q（flatten 保真失败）", k)
		}
	}
	// 已知字段不应漏进 Extra。
	for _, k := range []string{"id", "type", "name", "description"} {
		if _, ok := c.Extra[k]; ok {
			t.Fatalf("已知字段 %q 不应进 Extra", k)
		}
	}
}

func TestLoadTemplates_NonEmpty(t *testing.T) {
	resetState()
	v, err := loadTemplates()
	if err != nil {
		t.Fatalf("loadTemplates: %v", err)
	}
	if len(v) == 0 {
		t.Fatal("templates: 期望非空")
	}
}

func TestStandards_HasModsdkExpertAndCJKKey(t *testing.T) {
	resetState()
	m, err := loadStandards()
	if err != nil {
		t.Fatalf("loadStandards: %v", err)
	}
	if _, ok := m["modsdk-expert"]; !ok {
		t.Fatal("standards 缺 modsdk-expert")
	}
	// CJK 文件 stem 作 key（如「内存优化.md」→「内存优化」）。
	if _, ok := m["内存优化"]; !ok {
		t.Fatalf("standards 缺 CJK key「内存优化」；现有 keys=%v", keysOf(m))
	}
}

// ── ComponentEntry 自定义 UnmarshalJSON（flatten extra）──────────────────

func TestComponentEntry_UnmarshalFlatten(t *testing.T) {
	raw := `{"id":"x:foo","type":"block","name":"测试","description":"d",
	         "data_type":"integer","properties":{"a":1},"values":[1,2],"example":64,"custom":"keep"}`
	var c ComponentEntry
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.ID != "x:foo" || c.Kind != "block" || c.Name != "测试" || c.Description != "d" {
		t.Fatalf("已知字段解析错: %+v", c)
	}
	for _, k := range []string{"data_type", "properties", "values", "example", "custom"} {
		if _, ok := c.Extra[k]; !ok {
			t.Fatalf("Extra 缺键 %q", k)
		}
	}
	if len(c.Extra) != 5 {
		t.Fatalf("Extra 期望 5 键，得 %d: %v", len(c.Extra), keysOfRaw(c.Extra))
	}
}

// ── 双源加载（用户数据根覆盖内嵌）────────────────────────────────────────

func TestUserDataOverridesEmbedded(t *testing.T) {
	resetState()
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	mustMkdir(t, dataDir)
	mustWrite(t, filepath.Join(dataDir, "api-index.json"),
		`[{"type":"api","name":"FakePhaseATestApi","side":"server"}]`)

	SetDataRoot(root)
	defer resetState()
	v, err := loadApis()
	if err != nil {
		t.Fatalf("loadApis(override): %v", err)
	}
	if len(v) != 1 || v[0].Name != "FakePhaseATestApi" {
		t.Fatalf("用户覆盖失败：得 %d 条", len(v))
	}
}

func TestMissingUserDataFallsBack(t *testing.T) {
	resetState()
	SetDataRoot(filepath.Join(t.TempDir(), "nonexistent"))
	defer resetState()
	v, err := loadApis()
	if err != nil {
		t.Fatalf("loadApis(fallback): %v", err)
	}
	if len(v) < 1000 {
		t.Fatalf("内嵌回退失败：得 %d 条", len(v))
	}
}

func TestCorruptedUserDataErrors(t *testing.T) {
	resetState()
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	mustMkdir(t, dataDir)
	mustWrite(t, filepath.Join(dataDir, "api-index.json"), "not json")

	SetDataRoot(root)
	defer resetState()
	if _, err := loadApis(); err == nil {
		t.Fatal("期望 parse 报错（用户数据损坏不静默回退）")
	}
}

func TestWalkDirUsesUserDataWhenPresent(t *testing.T) {
	resetState()
	root := t.TempDir()
	docsDir := filepath.Join(root, "data", "docs", "mcguide")
	mustMkdir(t, docsDir)
	mustWrite(t, filepath.Join(docsDir, "phase-a-test.md"), "# Phase A test\n")

	SetDataRoot(root)
	defer resetState()
	entries := walkResourceDir("data/docs", "md")
	found := false
	for _, e := range entries {
		if e.Path == "mcguide/phase-a-test.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("walkResourceDir 应整树替换为用户目录；得 %d 条: %v", len(entries), pathsOf(entries))
	}
	// 目录级替换：用户目录只有 1 个 md，内嵌 1982 文件不应混入。
	if len(entries) != 1 {
		t.Fatalf("期望目录级替换（仅 1 条），得 %d 条", len(entries))
	}
}

func TestSetDataRootEmptyClears(t *testing.T) {
	SetDataRoot(filepath.Join(t.TempDir(), "whatever"))
	SetDataRoot("")
	if CurrentDataRoot() != "" {
		t.Fatalf("SetDataRoot(\"\") 后应清空，得 %q", CurrentDataRoot())
	}
}

// ── 路径辅助函数 ─────────────────────────────────────────────────────────

func TestMatchExt(t *testing.T) {
	cases := []struct {
		name, ext string
		want      bool
	}{
		{"foo.md", "md", true},
		{"foo.MD", "md", true},
		{"a/b/c.json", "json", true},
		{"内存优化.md", "md", true},
		{"noext", "md", false},
		{"dir.x/file", "x", false}, // 点在目录段
		{"foo.json", "md", false},
	}
	for _, c := range cases {
		if got := matchExt(c.name, c.ext); got != c.want {
			t.Errorf("matchExt(%q,%q)=%v 期望 %v", c.name, c.ext, got, c.want)
		}
	}
}

func TestFileStem(t *testing.T) {
	cases := map[string]string{
		"modsdk-expert.md": "modsdk-expert",
		"内存优化.md":          "内存优化",
		"sub/foo.md":       "foo",
		"foo.tar.gz":       "foo.tar",
		"noext":            "noext",
	}
	for in, want := range cases {
		if got := fileStem(in); got != want {
			t.Errorf("fileStem(%q)=%q 期望 %q", in, got, want)
		}
	}
}

// ── 测试辅助 ─────────────────────────────────────────────────────────────

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func keysOfRaw(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func pathsOf(entries []resourceFile) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Path)
	}
	return out
}
