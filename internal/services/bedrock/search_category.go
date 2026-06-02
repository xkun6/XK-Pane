package bedrock

import (
	"fmt"
	"sort"
	"strings"
)

// search_category.go：分类枚举。对齐 search/category.rs。

// listCategories ← pe_list_categories。全分类字母序 `cat (N APIs)`。
func listCategories() (string, error) {
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	if len(idx.apisByCategory) == 0 {
		return "No categories indexed.", nil
	}
	type cc struct {
		cat string
		n   int
	}
	cats := make([]cc, 0, len(idx.apisByCategory))
	for k, v := range idx.apisByCategory {
		cats = append(cats, cc{k, len(v)})
	}
	sort.SliceStable(cats, func(i, j int) bool { return cats[i].cat < cats[j].cat })

	var b strings.Builder
	for _, c := range cats {
		fmt.Fprintf(&b, "%s (%d APIs)\n", c.cat, c.n)
	}
	return b.String(), nil
}

// listApisByCategory ← pe_list_apis_by_category。先原始大小写 key 精确命中，miss 则 lower
// 全表子串过滤、key 升序逐个 render（与 by_name 全 lower 不同，刻意保留）。对齐 category.rs:42-69。
func listApisByCategory(category string) (string, error) {
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	if entries, ok := idx.apisByCategory[category]; ok {
		return renderCategory(category, entries), nil
	}

	query := strings.ToLower(category)
	type m struct {
		cat     string
		entries []*ApiEntry
	}
	var matches []m
	for k, v := range idx.apisByCategory {
		if strings.Contains(strings.ToLower(k), query) {
			matches = append(matches, m{k, v})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].cat < matches[j].cat })

	if len(matches) == 0 {
		return fmt.Sprintf("No category matched \"%s\". Use list_categories() to see all.", category), nil
	}
	var b strings.Builder
	for _, mm := range matches {
		b.WriteString(renderCategory(mm.cat, mm.entries))
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func renderCategory(cat string, entries []*ApiEntry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Category: %s\n", cat)
	fmt.Fprintf(&b, "APIs (%d):\n", len(entries))
	sorted := make([]*ApiEntry, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	for _, e := range sorted {
		fmt.Fprintf(&b, "  - %s [%s]\n", e.Name, sideLabel(e.Side))
	}
	return b.String()
}
