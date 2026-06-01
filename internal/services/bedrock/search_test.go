package bedrock

import (
	"strings"
	"testing"
)

func u32(v uint32) *uint32 { return &v }

// ── API（移植 api.rs tests，含真实数据 CanSee / HideSneakGui）──────────────

func TestSearchAPI_PlayerHits(t *testing.T) {
	resetState()
	out, err := searchAPI("Player")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Found ") || !(strings.Contains(out, "Player") || strings.Contains(out, "player")) {
		t.Fatalf("unexpected:\n%s", out)
	}
}

func TestGetAPI_CanSeeRendersParams(t *testing.T) {
	resetState()
	out, err := getAPI("CanSee", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# CanSee", "## Parameters", "## Description"} {
		if !strings.Contains(out, want) {
			t.Fatalf("缺 %q:\n%s", want, out)
		}
	}
}

func TestGetAPI_FiltersTableArtifacts(t *testing.T) {
	resetState()
	out, _ := getAPI("CanSee", nil)
	for _, bad := range []string{"`参数名`", "数据类型", "- `------"} {
		if strings.Contains(out, bad) {
			t.Fatalf("不应渲染表头伪行 %q:\n%s", bad, out)
		}
	}
}

func TestGetAPI_DerivedCategoryAndAgentNote(t *testing.T) {
	resetState()
	out, err := getAPI("HideSneakGui", sp("client"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Category: 原生UI", "## Agent Note", "下马按钮"} {
		if !strings.Contains(out, want) {
			t.Fatalf("缺 %q:\n%s", want, out)
		}
	}
}

func TestGetAPI_SideFilterNoSplit(t *testing.T) {
	resetState()
	out, err := getAPI("CanSee", sp("server"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "# CanSee  [server]") {
		t.Fatalf("缺 server 标头:\n%s", out)
	}
	if strings.Contains(out, "\n---\n") {
		t.Fatalf("过滤后不应分段:\n%s", out)
	}
}

func TestGetAPI_InvalidSideErrors(t *testing.T) {
	resetState()
	if _, err := getAPI("CanSee", sp("middle")); err == nil || !strings.Contains(err.Error(), "invalid side") {
		t.Fatalf("期望 invalid side 错误，得 %v", err)
	}
}

func TestGetAPI_UnknownDidYouMean(t *testing.T) {
	resetState()
	out, _ := getAPI("ZzzMissingApiZzz", nil)
	if !strings.HasPrefix(out, "No API named ") {
		t.Fatalf("unexpected:\n%s", out)
	}
}

func TestValidateCall(t *testing.T) {
	resetState()
	if out, _ := validateCall("CanSee", "server"); !strings.HasPrefix(out, "VALID:") {
		t.Fatalf("VALID 期望:\n%s", out)
	}
	if out, _ := validateCall("ZzzMissingApiZzz", "server"); !strings.HasPrefix(out, "INVALID: no API/event") {
		t.Fatalf("unexpected:\n%s", out)
	}
	if out, _ := validateCall("CanSee", "middle"); !strings.HasPrefix(out, "INVALID: side must be") {
		t.Fatalf("unexpected:\n%s", out)
	}
}

func TestAPIStats(t *testing.T) {
	resetState()
	out, err := apiStats()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"APIs:", "Events:", "Top 10 categories"} {
		if !strings.Contains(out, want) {
			t.Fatalf("缺 %q:\n%s", want, out)
		}
	}
}

// ── Event ──────────────────────────────────────────────────────────────

func sampleEventName(t *testing.T) string {
	t.Helper()
	idx, err := getIndices()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range idx.allEvents {
		if e.Name != "" {
			return e.Name
		}
	}
	t.Fatal("无 event")
	return ""
}

func TestSearchEvent(t *testing.T) {
	resetState()
	out, err := searchEvent("Event")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Found ") {
		t.Fatalf("unexpected:\n%s", out)
	}
	if !strings.Contains(out, "[server]") && !strings.Contains(out, "[client]") {
		t.Fatalf("缺 side 标签:\n%s", out)
	}
}

func TestGetEvent(t *testing.T) {
	resetState()
	name := sampleEventName(t)
	out, err := getEvent(name, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "# "+name) {
		t.Fatalf("缺标头 # %s:\n%s", name, out)
	}
	out2, _ := getEvent("ZzzMissingEventZzz", nil)
	if !strings.HasPrefix(out2, "No event named ") {
		t.Fatalf("unexpected:\n%s", out2)
	}
}

// ── Category ───────────────────────────────────────────────────────────

func TestListCategories(t *testing.T) {
	resetState()
	out, err := listCategories()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "APIs)") {
		t.Fatalf("格式不符:\n%s", out)
	}
}

func TestListApisByCategory(t *testing.T) {
	resetState()
	idx, _ := getIndices()
	var cat string
	for k := range idx.apisByCategory {
		cat = k
		break
	}
	out, err := listApisByCategory(cat)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Category: "+cat) || !strings.Contains(out, "APIs (") {
		t.Fatalf("缺标头:\n%s", out)
	}
	out2, _ := listApisByCategory("__nope__nope__nope__")
	if !strings.HasPrefix(out2, "No category matched") {
		t.Fatalf("unexpected:\n%s", out2)
	}
}

// ── Component（含 Definition JSON 字节级断言）────────────────────────────

func TestSearchComponent(t *testing.T) {
	resetState()
	if out, err := searchComponent("armor", "all"); err != nil || !strings.Contains(out, "Found ") {
		t.Fatalf("armor: err=%v out=%s", err, out)
	}
	if _, err := searchComponent("food", "bogus_type"); err == nil || !strings.Contains(err.Error(), "invalid component_type") {
		t.Fatalf("期望 invalid component_type，得 %v", err)
	}
}

func TestGetComponent_DefinitionJSON(t *testing.T) {
	resetState()
	out, err := getComponent("minecraft:max_stack_size")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "# minecraft:max_stack_size  [item]") {
		t.Fatalf("缺标头:\n%s", out)
	}
	// renderExtraJSON 字节级契约：键升序 + 2 空格缩进 + 空 {}/[] 内联 + 不转义。
	wantJSON := "{\n  \"data_type\": \"integer\",\n  \"example\": 64,\n  \"properties\": {},\n  \"values\": []\n}"
	if !strings.Contains(out, wantJSON) {
		t.Fatalf("Definition JSON 不符；期望含:\n%s\n实际:\n%s", wantJSON, out)
	}
}

func TestGetComponent_Unknown(t *testing.T) {
	resetState()
	out, _ := getComponent("definitely:not_a_real_component_id_zzz")
	if !strings.HasPrefix(out, "No component with id") {
		t.Fatalf("unexpected:\n%s", out)
	}
}

// ── Guide ──────────────────────────────────────────────────────────────

func TestSearchGuide_ChineseKeyword(t *testing.T) {
	resetState()
	out, err := searchGuide("自定义", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Found ") {
		t.Fatalf("期望命中:\n%s", out)
	}
	if n := strings.Count(out, "\n- ["); n < 5 {
		t.Fatalf("期望 >=5 命中，得 %d:\n%s", n, out)
	}
}

func TestSearchGuide_SourceFilter(t *testing.T) {
	resetState()
	out, err := searchGuide("UI", sp("mcguide"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Found ") {
		t.Fatalf("期望命中:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "- [") && !strings.HasPrefix(line, "- [mcguide]") {
			t.Fatalf("非 mcguide 结果泄漏: %s", line)
		}
	}
}

func TestSearchGuide_Errors(t *testing.T) {
	resetState()
	if _, err := searchGuide("UI", sp("notarealsource")); err == nil || !strings.Contains(err.Error(), "invalid source") {
		t.Fatalf("期望 invalid source，得 %v", err)
	}
	if _, err := searchGuide("   ", nil); err == nil || !strings.Contains(err.Error(), "keyword must not be empty") {
		t.Fatalf("期望空 keyword 错误，得 %v", err)
	}
}

func TestReadGuide(t *testing.T) {
	resetState()
	idx, _ := getIndices()
	sample := idx.guides[0].Path
	out, err := readGuide(sample, u32(10))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "File: data/docs/") || !strings.Contains(out, "Lines: ") {
		t.Fatalf("缺标头:\n%s", out)
	}
}

func TestReadGuide_Blocks(t *testing.T) {
	resetState()
	getIndices() // 确保索引已建（路径校验需要）
	cases := []struct {
		path, want string
	}{
		{"../etc/passwd", ".."},
		{"/etc/passwd.md", "leading slash"},
		{"C:/Windows/system32.md", "absolute"},
		{"mcguide/cheat.txt", ".md"},
		{"mcguide/totally-not-a-real-guide.md", "not a known guide"},
	}
	for _, c := range cases {
		_, err := readGuide(c.path, nil)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("readGuide(%q): 期望含 %q，得 %v", c.path, c.want, err)
		}
	}
}

// ── Template ───────────────────────────────────────────────────────────

func firstTemplateID(t *testing.T) string {
	t.Helper()
	idx, _ := getIndices()
	for k := range idx.templates {
		return k
	}
	t.Fatal("无 template")
	return ""
}

func TestMatchTemplate(t *testing.T) {
	resetState()
	if out, err := matchTemplate(nil, "UI"); err != nil || !strings.HasPrefix(out, "Found ") {
		t.Fatalf("keyword UI: err=%v out=%s", err, out)
	}
	idx, _ := getIndices()
	var tag string
	for _, tm := range idx.templates {
		if len(tm.Tags) > 0 {
			tag = tm.Tags[0]
			break
		}
	}
	if tag != "" {
		if out, err := matchTemplate([]string{tag}, ""); err != nil || !strings.HasPrefix(out, "Found ") {
			t.Fatalf("tag %q: err=%v out=%s", tag, err, out)
		}
	}
	if _, err := matchTemplate(nil, ""); err == nil || !strings.Contains(err.Error(), "provide tags or keyword") {
		t.Fatalf("期望 provide tags or keyword，得 %v", err)
	}
}

func TestTemplateInfo(t *testing.T) {
	resetState()
	id := firstTemplateID(t)
	out, err := templateInfo(id)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "# "+id) || !strings.Contains(out, "Tags:") {
		t.Fatalf("缺标头/Tags:\n%s", out)
	}
	if strings.Contains(out, "Agent/bedrock-upstream") {
		t.Fatalf("不应保留死链前缀:\n%s", out)
	}
	if !strings.Contains(out, "(source not embedded)") {
		t.Fatalf("缺 source not embedded 标记:\n%s", out)
	}
	out2, _ := templateInfo("Zzz-no-such-template-zzz")
	if !strings.HasPrefix(out2, "No template ") {
		t.Fatalf("unexpected:\n%s", out2)
	}
}

func TestTemplateFile_Blocks(t *testing.T) {
	resetState()
	id := firstTemplateID(t)
	if _, err := templateFile(id, "../etc/passwd"); err == nil || !strings.Contains(err.Error(), "..") {
		t.Fatalf("traversal: %v", err)
	}
	if _, err := templateFile(id, "/etc/passwd"); err == nil || !strings.Contains(err.Error(), "leading slash") {
		t.Fatalf("absolute: %v", err)
	}
	_, err := templateFile(id, "modMain.py")
	if err == nil || !(strings.Contains(err.Error(), "not embedded") || strings.Contains(err.Error(), "get_template_info")) {
		t.Fatalf("期望 not embedded，得 %v", err)
	}
}

func TestListModPacks(t *testing.T) {
	resetState()
	out, err := listModPacks()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Mod Packs (") {
		t.Fatalf("缺标头:\n%s", out)
	}
	headers := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "## ") {
			headers++
		}
	}
	if headers < 1 {
		t.Fatalf("期望 >=1 组:\n%s", out)
	}
}
