package bedrock

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// search_guide.go：guide 搜索/读取。对齐 search/guide.rs。

var knownGuideSources = []string{"mcguide", "mconline", "wiki", "ui_templates"}

const (
	guideMaxResults      = 20
	guideMaxLinesDefault = 200
	guideMaxLinesCeiling = 2000
)

// searchGuide ← pe_search_guide。tokens = lower + split whitespace（CJK 连续串=单 token）。
// source 可选 ∈ {mcguide,mconline,wiki,ui_templates}。排序 (score 降, title 升) truncate 20。
func searchGuide(keyword string, source *string) (string, error) {
	tokens := strings.Fields(strings.ToLower(keyword))
	if len(tokens) == 0 {
		return "", fmt.Errorf("keyword must not be empty")
	}
	if source != nil && !containsStr(knownGuideSources, *source) {
		return "", fmt.Errorf("invalid source '%s' (expected one of: %s)", *source, strings.Join(knownGuideSources, ", "))
	}

	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	type sg struct {
		score int
		e     *GuideEntry
	}
	var scored []sg
	for i := range idx.guides {
		e := &idx.guides[i]
		if source != nil && e.Source != *source {
			continue
		}
		s := guideScore(tokens, e)
		if s > 0 {
			scored = append(scored, sg{s, e})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].e.Title < scored[j].e.Title
	})
	if len(scored) > guideMaxResults {
		scored = scored[:guideMaxResults]
	}

	var b strings.Builder
	if len(scored) == 0 {
		suffix := ""
		if source != nil {
			suffix = fmt.Sprintf(" in source '%s'", *source)
		}
		fmt.Fprintf(&b, "No guide matched \"%s\"%s.\n", keyword, suffix)
		return b.String(), nil
	}
	suffix := ""
	if source != nil {
		suffix = fmt.Sprintf(" in %s", *source)
	}
	fmt.Fprintf(&b, "Found %d guide(s) matching \"%s\"%s:\n\n", len(scored), keyword, suffix)
	for _, r := range scored {
		fmt.Fprintf(&b, "- [%s] %s  (score=%d)\n", r.e.Source, r.e.Title, r.score)
		fmt.Fprintf(&b, "  path: %s\n", r.e.Path)
		if r.e.Breadcrumb != "" {
			fmt.Fprintf(&b, "  breadcrumb: %s\n", r.e.Breadcrumb)
		}
	}
	return b.String(), nil
}

// readGuide ← pe_read_guide。path 相对 data/docs/（正斜杠），按 in-memory guide 索引校验防越界。
// 对齐 guide.rs:97-153。
func readGuide(path string, maxLines *uint32) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path must not contain '..'")
	}
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return "", fmt.Errorf("path must be relative (no leading slash)")
	}
	// 拒绝 Windows 绝对路径（如 "C:foo" / "C:\\foo"）。
	if len(path) >= 2 && path[1] == ':' {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	if !strings.HasSuffix(strings.ToLower(path), ".md") {
		return "", fmt.Errorf("path must point to a .md file")
	}

	normalised := strings.ReplaceAll(path, "\\", "/")
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	exists := false
	for i := range idx.guides {
		if idx.guides[i].Path == normalised {
			exists = true
			break
		}
	}
	if !exists {
		return "", fmt.Errorf("path '%s' is not a known guide (use search_guide to discover valid paths)", normalised)
	}

	linesCap := uint32(guideMaxLinesDefault)
	if maxLines != nil {
		linesCap = *maxLines
	}
	if linesCap == 0 {
		linesCap = 1
	}
	if linesCap > guideMaxLinesCeiling {
		linesCap = guideMaxLinesCeiling
	}

	resourcePath := "data/docs/" + normalised
	raw, err := readResource(resourcePath)
	if err != nil {
		return "", fmt.Errorf("guide file missing: %s: %w", resourcePath, err)
	}
	if !utf8.Valid(raw) {
		return "", fmt.Errorf("guide '%s' is not valid UTF-8", normalised)
	}
	allLines := splitLines(string(raw))
	totalLines := len(allLines)
	shown := int(linesCap)
	if shown > totalLines {
		shown = totalLines
	}
	content := strings.Join(allLines[:shown], "\n")

	var b strings.Builder
	fmt.Fprintf(&b, "File: data/docs/%s\n", normalised)
	fmt.Fprintf(&b, "Lines: %d shown of %d\n", shown, totalLines)
	b.WriteByte('\n')
	b.WriteString(content)
	if totalLines > shown {
		b.WriteString("\n\n--- truncated; pass a larger max_lines (up to 2000) to read more ---")
	}
	return b.String(), nil
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
