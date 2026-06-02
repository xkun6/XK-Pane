package bedrock

import (
	"fmt"
	"sort"
	"strings"
)

// search_api.go：API 搜索/查询/校验/统计 + 与 event 共用的渲染/打分/查找 helper。对齐 search/api.rs。

// scoredEntry 是 (score, *ApiEntry) 对，用于 score_and_rank 结果。
type scoredEntry struct {
	score int
	e     *ApiEntry
}

// buildBlob 把全部可搜索字段拼成全文 blob（name desc remark agent_note cat）。对齐 api.rs:16-41。
// cat：category 字段几乎全空 → 回退 deriveCategory。
func buildBlob(e *ApiEntry) string {
	cat := e.Category
	if cat == "" {
		if dc, ok := deriveCategory(e); ok {
			cat = dc
		}
	}
	return e.Name + " " + e.Description + " " + e.Remark + " " + e.AgentNote + " " + cat
}

// sideLabel：空 side → "any"，否则原样。对齐 api.rs:59-65（category.rs 同义，单包共用）。
func sideLabel(side string) string {
	if side == "" {
		return "any"
	}
	return side
}

// is*TableArtifact：上游 api-index.json 把源 markdown 表格的表头/分隔行也当成了 params/return
// 条目（如 参数名/数据类型/说明 或 ------），渲染时要跳过。对齐 api.rs:71-87。
func isParamTableArtifact(name, ty, desc string) bool {
	n := strings.TrimSpace(name)
	t := strings.TrimSpace(ty)
	d := strings.TrimSpace(desc)
	return n == "参数名" || t == "数据类型" || d == "说明" ||
		strings.HasPrefix(n, "---") || strings.HasPrefix(t, "---") || strings.HasPrefix(d, "---")
}

func isReturnTableArtifact(ty, desc string) bool {
	t := strings.TrimSpace(ty)
	d := strings.TrimSpace(desc)
	return t == "数据类型" || d == "说明" || strings.HasPrefix(t, "---") || strings.HasPrefix(d, "---")
}

// scoreAndRank 对给定池打分排序，返回前 limit 个。排序 (score 降, name 升)。对齐 api.rs:90-110。
func scoreAndRank(pool []*ApiEntry, query string, limit int) []scoredEntry {
	var scored []scoredEntry
	for _, e := range pool {
		blob := buildBlob(e)
		s := nameScore(query, e.Name, &blob)
		if s > 0 {
			scored = append(scored, scoredEntry{s, e})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].e.Name < scored[j].e.Name
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored
}

func renderSearchResults(label, query string, ranked []scoredEntry) string {
	if len(ranked) == 0 {
		return fmt.Sprintf("No %s matched \"%s\". Try a shorter or partial name.", label, query)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d %s(s) matching \"%s\":\n\n", len(ranked), label, query)
	for _, r := range ranked {
		preview := truncateChars(r.e.Description, 80)
		fmt.Fprintf(&b, "- %s [%s]  (score=%d)\n", r.e.Name, sideLabel(r.e.Side), r.score)
		if preview != "" {
			fmt.Fprintf(&b, "  %s\n", preview)
		}
	}
	return b.String()
}

// renderEntryMarkdown 把单条 ApiEntry 渲染成 Markdown 卡片。对齐 api.rs:144-216。
func renderEntryMarkdown(e *ApiEntry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s  [%s]\n", e.Name, sideLabel(e.Side))
	b.WriteByte('\n')

	cat := e.Category
	if cat == "" {
		if dc, ok := deriveCategory(e); ok {
			cat = dc
		}
	}
	if cat != "" {
		fmt.Fprintf(&b, "Category: %s\n", cat)
		b.WriteByte('\n')
	}
	if e.Description != "" {
		b.WriteString("## Description\n")
		b.WriteString(e.Description)
		b.WriteString("\n\n")
	}
	if e.Remark != "" {
		b.WriteString("## Remark\n")
		b.WriteString(e.Remark)
		b.WriteString("\n\n")
	}

	var realParams []Param
	for _, p := range e.Params {
		if !isParamTableArtifact(p.Name, p.Ty, p.Description) {
			realParams = append(realParams, p)
		}
	}
	if len(realParams) > 0 {
		b.WriteString("## Parameters\n")
		for _, p := range realParams {
			if p.Description == "" {
				fmt.Fprintf(&b, "- `%s` (%s)\n", p.Name, p.Ty)
			} else {
				fmt.Fprintf(&b, "- `%s` (%s) — %s\n", p.Name, p.Ty, p.Description)
			}
		}
		b.WriteByte('\n')
	}

	var realReturns []ReturnSpec
	for _, r := range e.Returns {
		if !isReturnTableArtifact(r.Ty, r.Description) {
			realReturns = append(realReturns, r)
		}
	}
	if len(realReturns) > 0 {
		b.WriteString("## Returns\n")
		for _, r := range realReturns {
			if r.Description == "" {
				fmt.Fprintf(&b, "- %s\n", r.Ty)
			} else {
				fmt.Fprintf(&b, "- %s — %s\n", r.Ty, r.Description)
			}
		}
		b.WriteByte('\n')
	}

	if e.AgentNote != "" {
		b.WriteString("## Agent Note\n")
		b.WriteString(e.AgentNote)
		b.WriteString("\n\n")
	}
	if e.Example != "" {
		b.WriteString("## Example\n")
		b.WriteString("```python\n")
		b.WriteString(e.Example)
		b.WriteString("\n```\n\n")
	}
	if e.Link != "" {
		fmt.Fprintf(&b, "Link: %s\n", e.Link)
	}
	return b.String()
}

// renderEntriesMarkdown 排序（server→client→其他，再 name 升）后用 "\n---\n\n" 连接。对齐 api.rs:218-239。
func renderEntriesMarkdown(entries []*ApiEntry) string {
	sideRank := func(s string) int {
		switch s {
		case "server":
			return 0
		case "client":
			return 1
		default:
			return 2
		}
	}
	sorted := make([]*ApiEntry, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool {
		ri, rj := sideRank(sorted[i].Side), sideRank(sorted[j].Side)
		if ri != rj {
			return ri < rj
		}
		return sorted[i].Name < sorted[j].Name
	})
	pieces := make([]string, len(sorted))
	for i, e := range sorted {
		pieces[i] = renderEntryMarkdown(e)
	}
	return strings.Join(pieces, "\n---\n\n")
}

func validateSide(side string) error {
	if side != "server" && side != "client" {
		return fmt.Errorf("invalid side: %s (expected 'server' or 'client')", side)
	}
	return nil
}

// lookupFiltered：lower(name) 查表，side 非 nil 则按 side 过滤。对齐 api.rs:382-394。
func lookupFiltered(m map[string][]*ApiEntry, name string, side *string) []*ApiEntry {
	key := strings.ToLower(name)
	src := m[key]
	out := make([]*ApiEntry, len(src))
	copy(out, src)
	if side != nil {
		var filtered []*ApiEntry
		for _, e := range out {
			if e.Side == *side {
				filtered = append(filtered, e)
			}
		}
		out = filtered
	}
	return out
}

// notFoundWithSuggestions：未命中时给 5 条建议。对齐 api.rs:396-419。
func notFoundWithSuggestions(idx *indices, label, name string, side *string) string {
	pool := idx.allApis
	if label == "event" {
		pool = idx.allEvents
	}
	suggestions := scoreAndRank(pool, name, 5)
	var b strings.Builder
	fmt.Fprintf(&b, "No %s named \"%s\"", label, name)
	if side != nil {
		fmt.Fprintf(&b, " (side='%s')", *side)
	}
	b.WriteByte('.')
	if len(suggestions) > 0 {
		b.WriteString(" Did you mean: ")
		names := make([]string, len(suggestions))
		for i, s := range suggestions {
			names[i] = s.e.Name
		}
		b.WriteString(strings.Join(names, ", "))
	}
	return b.String()
}

func dedupSorted(s []string) []string {
	if len(s) == 0 {
		return s
	}
	out := s[:1]
	for _, v := range s[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}

// ── 工具入口 ─────────────────────────────────────────────────────────────

// searchAPI ← pe_search_api。
func searchAPI(name string) (string, error) {
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	return renderSearchResults("API", name, scoreAndRank(idx.allApis, name, 20)), nil
}

// getAPI ← pe_get_api。side 给则须 server/client（否则返 error）。
func getAPI(name string, side *string) (string, error) {
	if side != nil {
		if err := validateSide(*side); err != nil {
			return "", err
		}
	}
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	entries := lookupFiltered(idx.apiByName, name, side)
	if len(entries) == 0 {
		return notFoundWithSuggestions(idx, "API", name, side), nil
	}
	return renderEntriesMarkdown(entries), nil
}

// validateCall ← pe_validate_call。side 非法返 "INVALID:" 字符串（非 error），合并查 api+event。
func validateCall(name, side string) (string, error) {
	if side != "server" && side != "client" {
		return fmt.Sprintf("INVALID: side must be 'server' or 'client', got '%s'", side), nil
	}
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	key := strings.ToLower(name)
	var all []*ApiEntry
	all = append(all, idx.apiByName[key]...)
	all = append(all, idx.eventByName[key]...)

	if len(all) == 0 {
		suggestions := scoreAndRank(idx.allApis, name, 3)
		if len(suggestions) == 0 {
			return fmt.Sprintf("INVALID: no API/event named '%s'", name), nil
		}
		names := make([]string, len(suggestions))
		for i, s := range suggestions {
			names[i] = s.e.Name
		}
		return fmt.Sprintf("INVALID: no API/event named '%s'. Did you mean: %s?", name, strings.Join(names, ", ")), nil
	}

	var matchesSide []*ApiEntry
	for _, e := range all {
		if e.Side == side {
			matchesSide = append(matchesSide, e)
		}
	}
	if len(matchesSide) == 0 {
		sides := make([]string, 0, len(all))
		for _, e := range all {
			sides = append(sides, sideLabel(e.Side))
		}
		sort.Strings(sides)
		sides = dedupSorted(sides)
		return fmt.Sprintf("INVALID: '%s' exists but only on side(s) [%s], not '%s'", name, strings.Join(sides, ", "), side), nil
	}
	return fmt.Sprintf("VALID: '%s' on side '%s'", name, side), nil
}

// apiStats ← pe_api_stats。KB 总量 + by side + top 10 分类。对齐 api.rs:324-376。
func apiStats() (string, error) {
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	var apiServer, apiClient, apiUnspec int
	for _, e := range idx.allApis {
		switch e.Side {
		case "server":
			apiServer++
		case "client":
			apiClient++
		default:
			apiUnspec++
		}
	}
	var evServer, evClient, evUnspec int
	for _, e := range idx.allEvents {
		switch e.Side {
		case "server":
			evServer++
		case "client":
			evClient++
		default:
			evUnspec++
		}
	}
	type catCount struct {
		cat string
		n   int
	}
	cats := make([]catCount, 0, len(idx.apisByCategory))
	for k, v := range idx.apisByCategory {
		cats = append(cats, catCount{k, len(v)})
	}
	sort.SliceStable(cats, func(i, j int) bool {
		if cats[i].n != cats[j].n {
			return cats[i].n > cats[j].n
		}
		return cats[i].cat < cats[j].cat
	})

	var b strings.Builder
	b.WriteString("Bedrock Knowledge Base Stats\n")
	b.WriteString("=========================\n")
	fmt.Fprintf(&b, "APIs:    %d\n", len(idx.allApis))
	fmt.Fprintf(&b, "Events:  %d\n", len(idx.allEvents))
	fmt.Fprintf(&b, "Headers: %d\n", len(idx.allHeaders))
	b.WriteByte('\n')
	b.WriteString("By side:\n")
	fmt.Fprintf(&b, "  server: %d APIs, %d events\n", apiServer, evServer)
	fmt.Fprintf(&b, "  client: %d APIs, %d events\n", apiClient, evClient)
	fmt.Fprintf(&b, "  unspecified: %d APIs, %d events\n", apiUnspec, evUnspec)
	b.WriteByte('\n')
	b.WriteString("Top 10 categories by API count:\n")
	for i, c := range cats {
		if i >= 10 {
			break
		}
		fmt.Fprintf(&b, "  %d. %s: %d\n", i+1, c.cat, c.n)
	}
	return b.String(), nil
}
