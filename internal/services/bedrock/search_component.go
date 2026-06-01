package bedrock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// search_component.go：组件搜索/查询。对齐 search/component.rs。

var validComponentTypes = []string{"all", "item", "block", "entity", "netease_item", "netease_block"}

func validateComponentType(t string) error {
	for _, v := range validComponentTypes {
		if v == t {
			return nil
		}
	}
	return fmt.Errorf("invalid component_type: %s (expected one of %s)", t, strings.Join(validComponentTypes, ", "))
}

// scoreComponent：id Contains +3 / name +2 / description +1（可叠加，最高 6）。对齐 component.rs:29-44。
func scoreComponent(queryLC string, c *ComponentEntry) int {
	if queryLC == "" {
		return 0
	}
	score := 0
	if strings.Contains(strings.ToLower(c.ID), queryLC) {
		score += 3
	}
	if strings.Contains(strings.ToLower(c.Name), queryLC) {
		score += 2
	}
	if strings.Contains(strings.ToLower(c.Description), queryLC) {
		score += 1
	}
	return score
}

// searchComponent ← bedrock_search_component。componentType 默认 all 由调用方（工具层）解析后传入。
func searchComponent(query, componentType string) (string, error) {
	if err := validateComponentType(componentType); err != nil {
		return "", err
	}
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	q := strings.ToLower(query)
	type sc struct {
		score int
		c     *ComponentEntry
	}
	var scored []sc
	for _, c := range idx.components {
		if componentType != "all" && c.Kind != componentType {
			continue
		}
		s := scoreComponent(q, c)
		if s > 0 {
			scored = append(scored, sc{s, c})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].c.ID < scored[j].c.ID
	})
	if len(scored) > 20 {
		scored = scored[:20]
	}

	if len(scored) == 0 {
		return fmt.Sprintf("No component matched \"%s\" (type filter: %s).", query, componentType), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d component(s) matching \"%s\":\n\n", len(scored), query)
	for _, r := range scored {
		kind := r.c.Kind
		if kind == "" {
			kind = "?"
		}
		name := ""
		if r.c.Name != "" {
			name = " " + r.c.Name
		}
		fmt.Fprintf(&b, "- %s [%s]%s\n", r.c.ID, kind, name)
	}
	return b.String(), nil
}

// getComponent ← bedrock_get_component。按 id 精确查，渲染卡片（含 ## Definition JSON，用 Extra）。
// 对齐 component.rs:92-132。
func getComponent(componentID string) (string, error) {
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	c, ok := idx.components[componentID]
	if !ok {
		return fmt.Sprintf("No component with id \"%s\". Use search_component to find candidates.", componentID), nil
	}

	var b strings.Builder
	kind := c.Kind
	if kind == "" {
		kind = "?"
	}
	fmt.Fprintf(&b, "# %s  [%s]\n", c.ID, kind)
	b.WriteByte('\n')
	if c.Name != "" {
		fmt.Fprintf(&b, "Name: %s\n", c.Name)
	}
	if c.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", c.Description)
	}
	if len(c.Extra) > 0 {
		b.WriteByte('\n')
		b.WriteString("## Definition\n")
		b.WriteString("```json\n")
		b.WriteString(renderExtraJSON(c.Extra))
		b.WriteByte('\n')
		b.WriteString("```\n")
	}
	return b.String(), nil
}

// renderExtraJSON 复刻 serde_json::to_string_pretty(Value::Object(BTreeMap))：键升序（Go json
// 对 map 键天然排序）、2 空格缩进、**不转义 HTML**（serde 不转义 <>&，Go 默认转义 → 须关）。
// 对齐 component.rs:112-130。
func renderExtraJSON(extra map[string]json.RawMessage) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(extra); err != nil {
		return "{}"
	}
	return strings.TrimSuffix(buf.String(), "\n")
}
