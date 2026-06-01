package bedrock

import (
	"strings"
	"testing"
)

func TestIndicesBuild(t *testing.T) {
	resetState()
	idx, err := getIndices()
	if err != nil {
		t.Fatalf("getIndices: %v", err)
	}
	if len(idx.allApis) < 1000 {
		t.Errorf("allApis 太少: %d", len(idx.allApis))
	}
	if len(idx.allEvents) < 50 {
		t.Errorf("allEvents 太少: %d", len(idx.allEvents))
	}
	if len(idx.components) == 0 {
		t.Error("components 空")
	}
	if len(idx.templates) == 0 {
		t.Error("templates 空")
	}
	if len(idx.guides) == 0 {
		t.Error("guides 空")
	}
	if _, ok := idx.standards["modsdk-expert"]; !ok {
		t.Error("standards 缺 modsdk-expert")
	}
}

func TestGuidesKnownSources(t *testing.T) {
	resetState()
	idx, err := getIndices()
	if err != nil {
		t.Fatalf("getIndices: %v", err)
	}
	sources := map[string]bool{}
	for _, g := range idx.guides {
		sources[g.Source] = true
	}
	for _, s := range []string{"mcguide", "mconline", "wiki"} {
		if !sources[s] {
			t.Errorf("guides 缺来源 %q", s)
		}
	}
}

func TestApiByNameLowercased(t *testing.T) {
	resetState()
	idx, err := getIndices()
	if err != nil {
		t.Fatalf("getIndices: %v", err)
	}
	var first *ApiEntry
	for _, e := range idx.allApis {
		if e.Name != "" {
			first = e
			break
		}
	}
	if first == nil {
		t.Fatal("无具名 api")
	}
	key := strings.ToLower(first.Name)
	if _, ok := idx.apiByName[key]; !ok {
		t.Errorf("apiByName 缺 lower key %q", key)
	}
}

func TestDeriveCategory(t *testing.T) {
	// link 编码：找「接口」锚点取下一段。
	e := &ApiEntry{Link: "/docs/mcdocs/1-ModAPI/接口/物理/刚体.md"}
	if cat, ok := deriveCategory(e); !ok || cat != "物理" {
		t.Errorf("接口锚点: got (%q,%v) want (物理,true)", cat, ok)
	}
	// 「事件」锚点。
	e2 := &ApiEntry{Link: "/docs/mcdocs/1-ModAPI/事件/世界/天气.md"}
	if cat, ok := deriveCategory(e2); !ok || cat != "世界" {
		t.Errorf("事件锚点: got (%q,%v) want (世界,true)", cat, ok)
	}
	// category 字段优先。
	e3 := &ApiEntry{Category: "显式分类", Link: "/docs/接口/忽略.md"}
	if cat, ok := deriveCategory(e3); !ok || cat != "显式分类" {
		t.Errorf("显式 category: got (%q,%v)", cat, ok)
	}
	// 无 link 无 category → false。
	if _, ok := deriveCategory(&ApiEntry{}); ok {
		t.Error("空条目应无分类")
	}
	// 末段去 .md，锚点在末尾无下一段 → false。
	if _, ok := deriveCategory(&ApiEntry{Link: "/a/接口"}); ok {
		t.Error("锚点在末尾应无分类")
	}
}
