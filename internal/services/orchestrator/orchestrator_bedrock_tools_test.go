package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeBedrock 记录最近一次调用入参，可注入错误。实现 Bedrock 接口（10 方法）。
type fakeBedrock struct {
	err           error
	gotSide       *string
	gotSideCalled bool
	gotCompType   string
}

func (f *fakeBedrock) SearchAPI(name string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "SEARCH_API:" + name, nil
}
func (f *fakeBedrock) GetAPI(name string, side *string) (string, error) {
	f.gotSide, f.gotSideCalled = side, true
	return "GET_API:" + name, nil
}
func (f *fakeBedrock) SearchEvent(name string) (string, error) { return "SEARCH_EVENT:" + name, nil }
func (f *fakeBedrock) GetEvent(name string, side *string) (string, error) {
	f.gotSide, f.gotSideCalled = side, true
	return "GET_EVENT:" + name, nil
}
func (f *fakeBedrock) ValidateCall(name, side string) (string, error) {
	return "VALIDATE:" + name + ":" + side, nil
}
func (f *fakeBedrock) APIStats() (string, error)       { return "STATS", nil }
func (f *fakeBedrock) ListCategories() (string, error) { return "CATS", nil }
func (f *fakeBedrock) ListApisByCategory(category string) (string, error) {
	return "BYCAT:" + category, nil
}
func (f *fakeBedrock) SearchComponent(query, componentType string) (string, error) {
	f.gotCompType = componentType
	return "SC:" + query + ":" + componentType, nil
}
func (f *fakeBedrock) GetComponent(componentID string) (string, error) {
	return "GC:" + componentID, nil
}
func (f *fakeBedrock) SearchGuide(keyword string, source *string) (string, error) {
	f.gotSide, f.gotSideCalled = source, true
	return "SG:" + keyword, nil
}
func (f *fakeBedrock) ReadGuide(path string, maxLines *uint32) (string, error) {
	return "RG:" + path, nil
}
func (f *fakeBedrock) MatchTemplate(tags []string, keyword string) (string, error) {
	return "MT:" + strings.Join(tags, ",") + ":" + keyword, nil
}
func (f *fakeBedrock) GetTemplateInfo(templateID string) (string, error) {
	return "TI:" + templateID, nil
}
func (f *fakeBedrock) GetTemplateFile(id, fp string) (string, error) {
	return "TF:" + id + ":" + fp, nil
}
func (f *fakeBedrock) GetStandard(name string) (string, error) { return "STD:" + name, nil }
func (f *fakeBedrock) GetExpertGuide() (string, error)         { return "EXPERT", nil }
func (f *fakeBedrock) GetReviewRules() (string, error)         { return "REVIEW", nil }
func (f *fakeBedrock) GetOptimizationRules() (string, error)   { return "OPT", nil }
func (f *fakeBedrock) GetJSONSpec(contentType *string) (string, error) {
	f.gotSide, f.gotSideCalled = contentType, true
	return "JSON", nil
}
func (f *fakeBedrock) BedrockHelp() (string, error)  { return "HELP", nil }
func (f *fakeBedrock) ListModPacks() (string, error) { return "MODPACKS", nil }
func (f *fakeBedrock) ReviewCode(code string, filename *string) (string, error) {
	f.gotSide, f.gotSideCalled = filename, true
	return "REVIEW_CODE:" + code, nil
}

func TestBedrockTools_NilFallback(t *testing.T) {
	ctx := context.Background()
	sNil := New(Deps{})
	res, _, _ := sNil.toolBedrockSearchAPI(ctx, nil, bedrockNameParams{Name: "x"})
	if txt := resultText(t, res); !strings.Contains(txt, "Bedrock 知识库不可用") {
		t.Errorf("nil 兜底应返回不可用文本，实际 %q", txt)
	}
	// 另抽查一个 stats（无参）也要兜底。
	res2, _, _ := sNil.toolBedrockAPIStats(ctx, nil, struct{}{})
	if txt := resultText(t, res2); !strings.Contains(txt, "不可用") {
		t.Errorf("stats nil 兜底应不可用，实际 %q", txt)
	}
}

func TestBedrockTools_Passthrough(t *testing.T) {
	ctx := context.Background()
	fb := &fakeBedrock{}
	s := New(Deps{Bedrock: fb})

	if txt := resultText(t, mustCall(s.toolBedrockSearchAPI(ctx, nil, bedrockNameParams{Name: "Foo"}))); txt != "SEARCH_API:Foo" {
		t.Errorf("search_api 透传错: %q", txt)
	}
	if txt := resultText(t, mustCall(s.toolBedrockValidateCall(ctx, nil, bedrockValidateCallParams{Name: "Foo", Side: "server"}))); txt != "VALIDATE:Foo:server" {
		t.Errorf("validate_call 透传错: %q", txt)
	}
	if txt := resultText(t, mustCall(s.toolBedrockListApisByCategory(ctx, nil, bedrockCategoryParams{Category: "物理"}))); txt != "BYCAT:物理" {
		t.Errorf("by_category 透传错: %q", txt)
	}
}

func TestBedrockTools_SearchComponentDefault(t *testing.T) {
	ctx := context.Background()
	fb := &fakeBedrock{}
	s := New(Deps{Bedrock: fb})

	// componentType 省略 → 默认 "all"。
	s.toolBedrockSearchComponent(ctx, nil, bedrockSearchComponentParams{Query: "armor"})
	if fb.gotCompType != "all" {
		t.Errorf("默认 componentType 应为 all，实际 %q", fb.gotCompType)
	}
	// 显式 item → 透传 item。
	item := "item"
	s.toolBedrockSearchComponent(ctx, nil, bedrockSearchComponentParams{Query: "armor", ComponentType: &item})
	if fb.gotCompType != "item" {
		t.Errorf("componentType 应透传 item，实际 %q", fb.gotCompType)
	}
}

func TestBedrockTools_SidePassthrough(t *testing.T) {
	ctx := context.Background()
	fb := &fakeBedrock{}
	s := New(Deps{Bedrock: fb})

	// side 省略 → nil 指针透传。
	s.toolBedrockGetAPI(ctx, nil, bedrockGetAPIParams{Name: "CanSee"})
	if !fb.gotSideCalled || fb.gotSide != nil {
		t.Errorf("省略 side 应透传 nil，实际 called=%v side=%v", fb.gotSideCalled, fb.gotSide)
	}
	// side=client → 透传非 nil。
	client := "client"
	s.toolBedrockGetAPI(ctx, nil, bedrockGetAPIParams{Name: "CanSee", Side: &client})
	if fb.gotSide == nil || *fb.gotSide != "client" {
		t.Errorf("side 应透传 client，实际 %v", fb.gotSide)
	}
}

func TestBedrockTools_ErrorPassthrough(t *testing.T) {
	ctx := context.Background()
	fb := &fakeBedrock{err: errors.New("invalid side: middle (expected 'server' or 'client')")}
	s := New(Deps{Bedrock: fb})
	res, _, _ := s.toolBedrockSearchAPI(ctx, nil, bedrockNameParams{Name: "x"})
	txt := resultText(t, res)
	if !strings.HasPrefix(txt, "错误: ") || !strings.Contains(txt, "invalid side") {
		t.Errorf("错误应以「错误: 」前缀 + 忠实英文消息，实际 %q", txt)
	}
}

// mustCall 取工具 handler 三返回值的首个（结果），丢弃 any/error（handler 恒返 nil error）。
func mustCall(res *mcp.CallToolResult, _ any, _ error) *mcp.CallToolResult { return res }
