package bedrock

import (
	"strings"
	"testing"
)

func TestGetStandard_KnownAndFuzzyAndUnknown(t *testing.T) {
	resetState()
	// 精确命中：# stem\n\n{content}。
	out, err := getStandard("modsdk-expert")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "# modsdk-expert") || len(out) < 200 {
		t.Fatalf("精确命中不符（len=%d）:\n%.120s", len(out), out)
	}
	// 带 .md 后缀也应命中同一文档。
	if o2, _ := getStandard("modsdk-expert.md"); !strings.HasPrefix(o2, "# modsdk-expert") {
		t.Fatalf(".md 后缀未命中:\n%.80s", o2)
	}
	// 模糊唯一命中：modsdk-json → modsdk-json-generator。
	if o3, _ := getStandard("modsdk-json"); !strings.HasPrefix(o3, "# modsdk-json-generator") {
		t.Fatalf("模糊未唯一解析:\n%.80s", o3)
	}
	// 未知。
	if o4, _ := getStandard("totally-not-a-standard"); !strings.HasPrefix(o4, "No standard named") {
		t.Fatalf("未知 standard 文案不符:\n%s", o4)
	}
	// 空名 → error。
	if _, err := getStandard(""); err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("空名应报错，得 %v", err)
	}
}

func TestFetchRequiredGuides_NonEmpty(t *testing.T) {
	resetState()
	for name, fn := range map[string]func() (string, error){
		"expert":       expertGuide,
		"review":       reviewRules,
		"optimization": optimizationRules,
	} {
		s, err := fn()
		if err != nil || s == "" {
			t.Errorf("%s: err=%v empty=%v", name, err, s == "")
		}
	}
	// fetch_required 契约：返回原文，**不 prepend `# {stem}\n\n`**（与 get_standard 不同）。
	// 验证关系：getStandard 输出 == "# {stem}\n\n" + fetch_required 原文。
	raw, _ := expertGuide()
	withHeader, _ := getStandard("modsdk-expert")
	if withHeader != "# modsdk-expert\n\n"+raw {
		t.Fatalf("getStandard 应在 fetch_required 原文前加 `# stem\\n\\n` 头；不符")
	}
}

func TestJSONSpec(t *testing.T) {
	resetState()
	both, err := jsonSpec(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(both, "\n\n---\n\n") || len(both) < 400 {
		t.Fatalf("both 应含分隔符且足够长（len=%d）", len(both))
	}
	ne := "netease"
	if s, _ := jsonSpec(&ne); strings.Contains(s, "\n\n---\n\n") || s == "" {
		t.Fatalf("netease-only 不应含分隔符且非空")
	}
	ms := "modsdk"
	if s, _ := jsonSpec(&ms); strings.Contains(s, "\n\n---\n\n") || s == "" {
		t.Fatalf("modsdk-only 不应含分隔符且非空")
	}
}

func TestHelpText(t *testing.T) {
	resetState()
	s, _ := helpText()
	for _, want := range []string{"search_api", "search_guide", "bedrock_help"} {
		if !strings.Contains(s, want) {
			t.Errorf("help 缺 %q", want)
		}
	}
}

// TestService_Delegation 确认 Service 方法正确委托到底层函数（抽查几个）。
func TestService_Delegation(t *testing.T) {
	resetState()
	svc := New()
	if err := svc.Warm(); err != nil {
		t.Fatalf("Warm: %v", err)
	}
	if out, err := svc.GetStandard("modsdk-expert"); err != nil || !strings.HasPrefix(out, "# modsdk-expert") {
		t.Fatalf("GetStandard 委托错: err=%v", err)
	}
	if out, err := svc.SearchAPI("CanSee"); err != nil || !strings.Contains(out, "Found ") {
		t.Fatalf("SearchAPI 委托错: err=%v out=%.60s", err, out)
	}
	ne := "netease"
	if out, err := svc.GetJSONSpec(&ne); err != nil || out == "" {
		t.Fatalf("GetJSONSpec 委托错: err=%v", err)
	}
}
