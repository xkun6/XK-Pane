package bedrock

import (
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// indices 是从 loader 输出构建的全部只读查表，驱动 O(1) 的 name_lower→entries 查询。
// 对齐 index.rs:20-41 的 Indices。指针 *ApiEntry 等指向 loader 缓存切片的后备数组（GC 在
// loader+index 都失效后回收，替代 Rust Box::leak）。
type indices struct {
	apiByName      map[string][]*ApiEntry // lower(name) → API 条目
	eventByName    map[string][]*ApiEntry // lower(name) → event 条目
	apisByCategory map[string][]*ApiEntry // 原始大小写 category → API+event 混（名为 apis 实混 event）
	allApis        []*ApiEntry            // kind=="api"
	allEvents      []*ApiEntry            // kind=="event"
	allHeaders     []*ApiEntry            // kind=="header"
	components     map[string]*ComponentEntry
	templates      map[string]*TemplateEntry
	guides         []GuideEntry
	standards      map[string]string // 文件 stem → md 内容
}

var (
	indicesMu    sync.RWMutex
	indicesCache *indices
)

// invalidateIndices 丢弃缓存索引，下次 getIndices 按活跃源重建。对齐 index.rs:47-49。
func invalidateIndices() {
	indicesMu.Lock()
	indicesCache = nil
	indicesMu.Unlock()
}

// getIndices 取单例索引；首调构建，后续 O(1)。对齐 index.rs:52-60。
func getIndices() (*indices, error) {
	indicesMu.RLock()
	if indicesCache != nil {
		v := indicesCache
		indicesMu.RUnlock()
		return v, nil
	}
	indicesMu.RUnlock()

	built, err := buildIndices()
	if err != nil {
		return nil, err
	}
	indicesMu.Lock()
	indicesCache = built
	indicesMu.Unlock()
	return built, nil
}

// deriveCategory 为条目派生分类桶名。对齐 index.rs:67-87。
// 上游 api-index.json 的 category 字段几乎全空，分类编码在 link 路径里，如
// `/docs/mcdocs/1-ModAPI/接口/物理/刚体.md` → `物理`（找锚点「接口」/「事件」按序，取其下一段去 .md）。
// 返回 (cat, true) 命中；("", false) 无。
func deriveCategory(e *ApiEntry) (string, bool) {
	if e.Category != "" {
		return e.Category, true
	}
	if e.Link == "" {
		return "", false
	}
	parts := strings.Split(strings.ReplaceAll(e.Link, "\\", "/"), "/")
	for _, anchor := range []string{"接口", "事件"} {
		pos := -1
		for i, p := range parts {
			if p == anchor {
				pos = i
				break
			}
		}
		if pos >= 0 && pos+1 < len(parts) {
			trimmed := strings.TrimSuffix(parts[pos+1], ".md")
			if trimmed != "" {
				return trimmed, true
			}
		}
	}
	return "", false
}

func buildIndices() (*indices, error) {
	apisVec, err := loadApis()
	if err != nil {
		return nil, err
	}
	compsVec, err := loadComponents()
	if err != nil {
		return nil, err
	}
	tmplsVec, err := loadTemplates()
	if err != nil {
		return nil, err
	}
	standardsMap, err := loadStandards()
	if err != nil {
		return nil, err
	}

	idx := &indices{
		apiByName:      map[string][]*ApiEntry{},
		eventByName:    map[string][]*ApiEntry{},
		apisByCategory: map[string][]*ApiEntry{},
		components:     map[string]*ComponentEntry{},
		templates:      map[string]*TemplateEntry{},
		standards:      standardsMap,
	}

	for i := range apisVec {
		e := &apisVec[i]
		switch e.Kind {
		case "api":
			idx.allApis = append(idx.allApis, e)
			if e.Name != "" {
				k := strings.ToLower(e.Name)
				idx.apiByName[k] = append(idx.apiByName[k], e)
			}
			if cat, ok := deriveCategory(e); ok {
				idx.apisByCategory[cat] = append(idx.apisByCategory[cat], e)
			}
		case "event":
			idx.allEvents = append(idx.allEvents, e)
			if e.Name != "" {
				k := strings.ToLower(e.Name)
				idx.eventByName[k] = append(idx.eventByName[k], e)
			}
			if cat, ok := deriveCategory(e); ok {
				idx.apisByCategory[cat] = append(idx.apisByCategory[cat], e)
			}
		case "header":
			idx.allHeaders = append(idx.allHeaders, e)
		default:
			// 未知 kind 静默忽略（刻意容错，对齐 index.rs:129）。
		}
	}

	for i := range compsVec {
		c := &compsVec[i]
		if c.ID != "" {
			idx.components[c.ID] = c
		}
	}

	for i := range tmplsVec {
		t := &tmplsVec[i]
		if t.ID != "" {
			idx.templates[t.ID] = t
		}
	}

	idx.guides = scanGuides()
	return idx, nil
}

// scanGuides 遍历活跃 data/docs 树构建 GuideEntry。对齐 index.rs:171-193。
// 注：data/docs/mcdocs 下 .md 也会被纳入（source="mcdocs"）；ui_templates 是 .json 不入 guides。
func scanGuides() []GuideEntry {
	entries := walkResourceDir("data/docs", "md")
	out := make([]GuideEntry, 0, len(entries))
	for _, f := range entries {
		rel := f.Path
		source := rel
		if i := strings.IndexByte(rel, '/'); i >= 0 {
			source = rel[:i]
		}
		segs := strings.Split(rel, "/")
		breadcrumb := strings.Join(segs[:len(segs)-1], " / ")
		text := ""
		if utf8.Valid(f.Content) {
			text = string(f.Content)
		}
		title, headings := parseHeadings(text, rel)
		out = append(out, GuideEntry{
			Title:      title,
			Path:       rel,
			Source:     source,
			Breadcrumb: breadcrumb,
			Headings:   headings,
		})
	}
	return out
}

// parseHeadings 提取 title（首个 `# ` 行，缺则文件 stem）与 headings（`## `/`### ` 行）。
// 对齐 index.rs:195-217：前缀判定带尾空格、trim_start 用 Unicode 空白、else-if 同序。
func parseHeadings(text, relPath string) (string, []string) {
	var title string
	titleSet := false
	var headings []string
	for _, line := range splitLines(text) {
		trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
		switch {
		case strings.HasPrefix(trimmed, "# "):
			if !titleSet {
				title = strings.TrimSpace(trimmed[len("# "):])
				titleSet = true
			}
		case strings.HasPrefix(trimmed, "## "):
			headings = append(headings, strings.TrimSpace(trimmed[len("## "):]))
		case strings.HasPrefix(trimmed, "### "):
			headings = append(headings, strings.TrimSpace(trimmed[len("### "):]))
		}
	}
	if !titleSet {
		title = fileStem(relPath)
	}
	return title, headings
}
