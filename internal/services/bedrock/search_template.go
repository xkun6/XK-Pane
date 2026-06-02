package bedrock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// search_template.go：模板 match/info/file/list_mod_packs。对齐 search/template.rs。
// TemplateEntry 的 apis/systems/file_count 是 json.RawMessage（serde_json::Value），按需解码。

// matchTemplate ← pe_match_template。tags ⊕ keyword 至少给一。
// tag：模板某 tag == +2 否则 Contains +1（else-if 互斥，每查询 tag 一次）；
// keyword：id Contains +3，模板每个 tag Contains +1（可累加）。排序 (score 降, id 升) truncate 20。
func matchTemplate(tags []string, keyword string) (string, error) {
	hasTags := len(tags) > 0
	hasKeyword := strings.TrimSpace(keyword) != ""
	if !hasTags && !hasKeyword {
		return "", fmt.Errorf("provide tags or keyword")
	}
	kwLower := strings.ToLower(keyword)
	tagsLower := make([]string, len(tags))
	for i, t := range tags {
		tagsLower[i] = strings.ToLower(t)
	}
	idx, err := getIndices()
	if err != nil {
		return "", err
	}

	type st struct {
		score int
		t     *TemplateEntry
	}
	var scored []st
	for _, tmpl := range idx.templates {
		entryTagsLower := make([]string, len(tmpl.Tags))
		for i, t := range tmpl.Tags {
			entryTagsLower[i] = strings.ToLower(t)
		}
		score := 0
		for _, tag := range tagsLower {
			if containsStr(entryTagsLower, tag) {
				score += 2
			} else if anyContains(entryTagsLower, tag) {
				score += 1
			}
		}
		if hasKeyword {
			if strings.Contains(strings.ToLower(tmpl.ID), kwLower) {
				score += 3
			}
			for _, et := range entryTagsLower {
				if strings.Contains(et, kwLower) {
					score += 1
				}
			}
		}
		if score > 0 {
			scored = append(scored, st{score, tmpl})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].t.ID < scored[j].t.ID
	})
	if len(scored) > 20 {
		scored = scored[:20]
	}

	if len(scored) == 0 {
		return "No template matched the given tags/keyword.", nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d template(s):\n\n", len(scored))
	for _, r := range scored {
		modName := r.t.ModName
		if modName == "" {
			modName = "(none)"
		}
		fmt.Fprintf(&b, "- %s  (score=%d)\n", r.t.ID, r.score)
		fmt.Fprintf(&b, "  mod: %s  tags: [%s]\n", modName, strings.Join(r.t.Tags, ", "))
		fmt.Fprintf(&b, "  path: %s\n", r.t.Path)
	}
	return b.String(), nil
}

// templateInfo ← pe_get_template_info。对齐 template.rs:88-196。
func templateInfo(templateID string) (string, error) {
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	tmpl, ok := idx.templates[templateID]
	if !ok {
		return fmt.Sprintf("No template \"%s\". Use match_template to discover candidates.", templateID), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", tmpl.ID)
	b.WriteByte('\n')
	modName := tmpl.ModName
	if modName == "" {
		modName = "(none)"
	}
	fmt.Fprintf(&b, "Mod: %s\n", modName)
	fmt.Fprintf(&b, "Path: %s (source not embedded)\n", tmpl.Path)
	fmt.Fprintf(&b, "Tags: %s\n", strings.Join(tmpl.Tags, ", "))
	fmt.Fprintf(&b, "Flags: has_client=%t has_ui=%t has_custom_entity=%t\n", tmpl.HasClient, tmpl.HasUI, tmpl.HasCustomEntity)
	eventsLine := "(none)"
	if len(tmpl.EventsUsed) > 0 {
		eventsLine = strings.Join(tmpl.EventsUsed, ", ")
	}
	fmt.Fprintf(&b, "Events used: %s\n", eventsLine)

	if fc, ok := rawObject(tmpl.FileCount); ok {
		py := i64OrZero(fc["py"])
		jsonN := i64OrZero(fc["json"])
		other := i64OrZero(fc["other"])
		fmt.Fprintf(&b, "File count: py=%d json=%d other=%d\n", py, jsonN, other)
	}

	if arr, ok := rawArray(tmpl.Systems); ok && len(arr) > 0 {
		b.WriteByte('\n')
		b.WriteString("Systems:\n")
		for _, sys := range arr {
			if obj, ok := rawObject(sys); ok {
				ns, _ := rawString(obj["namespace"])
				nm, _ := rawString(obj["name"])
				fmt.Fprintf(&b, "  - %s/%s\n", ns, nm)
			} else if s, ok := rawString(sys); ok {
				fmt.Fprintf(&b, "  - %s\n", s)
			}
		}
	}

	if apis, ok := rawObject(tmpl.Apis); ok {
		comps := joinStrArray(apis["components"])
		methods := joinStrArray(apis["methods"])
		if comps != "" || methods != "" {
			b.WriteByte('\n')
			cs := comps
			if cs == "" {
				cs = "(none)"
			}
			ms := methods
			if ms == "" {
				ms = "(none)"
			}
			fmt.Fprintf(&b, "Components: %s\n", cs)
			fmt.Fprintf(&b, "Methods: %s\n", ms)
		}
	}
	return b.String(), nil
}

func langHintFor(filePath string) string {
	lower := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(lower, ".py"):
		return "python"
	case strings.HasSuffix(lower, ".json"):
		return "json"
	case strings.HasSuffix(lower, ".md"):
		return "markdown"
	default:
		return "text"
	}
}

// templateFile ← pe_get_template_file。模板源文件当前未内嵌 → 路径校验后 read_resource 失败，
// 返回“未内嵌”说明（指向 get_template_info）。对齐 template.rs:219-263。
func templateFile(templateID, filePath string) (string, error) {
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	tmpl, ok := idx.templates[templateID]
	if !ok {
		return "", fmt.Errorf("no template named '%s'", templateID)
	}
	if filePath == "" {
		return "", fmt.Errorf("file_path must not be empty")
	}
	if strings.Contains(filePath, "..") {
		return "", fmt.Errorf("file_path must not contain '..'")
	}
	if strings.HasPrefix(filePath, "/") || strings.HasPrefix(filePath, "\\") {
		return "", fmt.Errorf("file_path must be relative (no leading slash)")
	}
	if len(filePath) >= 2 && filePath[1] == ':' {
		return "", fmt.Errorf("absolute paths are not allowed")
	}

	relPath := tmpl.Path + "/" + strings.ReplaceAll(filePath, "\\", "/")
	if raw, rerr := readResource(relPath); rerr == nil {
		if !utf8.Valid(raw) {
			return "", fmt.Errorf("file is not valid UTF-8: %s", relPath)
		}
		text := string(raw)
		var b strings.Builder
		fmt.Fprintf(&b, "File: %s/%s\n", tmpl.Path, filePath)
		b.WriteByte('\n')
		fmt.Fprintf(&b, "```%s\n", langHintFor(filePath))
		b.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
		return b.String(), nil
	}

	return "", fmt.Errorf("template files are not embedded in this build (template '%s' lives outside the embedded knowledge base). Use get_template_info to see the template's metadata.", templateID)
}

// listModPacks ← pe_list_mod_packs。按 mod_name 分组（空→"(unspecified)"），key 升序，组内 id 升序。
// 对齐 template.rs:267-305。注意：tags 用 "," 连接（非 match_ 的 ", "）。
func listModPacks() (string, error) {
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	groups := map[string][]*TemplateEntry{}
	total := len(idx.templates)
	for _, t := range idx.templates {
		key := t.ModName
		if key == "" {
			key = "(unspecified)"
		}
		groups[key] = append(groups[key], t)
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "Mod Packs (%d total, %d templates)\n\n", len(keys), total)
	for _, k := range keys {
		fmt.Fprintf(&b, "## %s\n", k)
		entries := groups[k]
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
		for _, t := range entries {
			tags := "(none)"
			if len(t.Tags) > 0 {
				tags = strings.Join(t.Tags, ",")
			}
			fmt.Fprintf(&b, "  - %s  tags=[%s]\n", t.ID, tags)
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// ── json.RawMessage 解码 helper（复刻 serde_json::Value 的 as_object/as_array/as_str/as_i64）──

func anyContains(haystack []string, sub string) bool {
	for _, h := range haystack {
		if strings.Contains(h, sub) {
			return true
		}
	}
	return false
}

func rawObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, false
	}
	return m, true
}

func rawArray(raw json.RawMessage) ([]json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var a []json.RawMessage
	if err := json.Unmarshal(raw, &a); err != nil || a == nil {
		return nil, false
	}
	return a, true
}

func rawString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func i64OrZero(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var n json.Number
	if err := dec.Decode(&n); err != nil {
		return 0
	}
	i, err := n.Int64()
	if err != nil {
		return 0
	}
	return i
}

func joinStrArray(raw json.RawMessage) string {
	arr, ok := rawArray(raw)
	if !ok {
		return ""
	}
	var parts []string
	for _, e := range arr {
		if s, ok := rawString(e); ok {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}
