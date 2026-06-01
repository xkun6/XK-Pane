package bedrock

import "strings"

// nameScore 单 token 对单名打分（带可选全文 blob）。0=不匹配，越高越好。对齐 fuzzy.rs:18-44。
//   - 精确（大小写不敏感相等）= 20
//   - name 以 query 开头        = 15
//   - name 含 query             = 10
//   - blob 含 query（name 未命中）= 3
//   - blob 含 query（name 也命中）= +3（加性）
//
// query 空 → 0 早退（否则空串 Contains 恒真污染全表）。blob 传 nil 表示 Rust 的 None。
func nameScore(query, name string, blob *string) int {
	q := strings.ToLower(query)
	if q == "" {
		return 0
	}
	n := strings.ToLower(name)
	var score int
	switch {
	case n == q:
		score = 20
	case strings.HasPrefix(n, q):
		score = 15
	case strings.Contains(n, q):
		score = 10
	default:
		score = 0
	}
	if blob != nil {
		if strings.Contains(strings.ToLower(*blob), q) {
			if score == 0 {
				score = 3
			} else {
				score += 3
			}
		}
	}
	return score
}

// guideScore 多 token 的 guide 字段加权子串打分（cc-pane 自称 "BM25-lite"，实无 k1/b/IDF/TF）。
// 对齐 fuzzy.rs:57-97：title 含 token +5 / breadcrumb +3 / 任一 heading +2 / path +1
// （每字段每 token 最多一次，hit 布尔）；matched=命中≥1 字段的 token 数；matched>1 → +matched*3。
// tokens 须已小写、已切分。
func guideScore(tokens []string, e *GuideEntry) int {
	if len(tokens) == 0 {
		return 0
	}
	title := strings.ToLower(e.Title)
	breadcrumb := strings.ToLower(e.Breadcrumb)
	path := strings.ToLower(e.Path)
	headings := make([]string, len(e.Headings))
	for i, h := range e.Headings {
		headings[i] = strings.ToLower(h)
	}

	score := 0
	matched := 0
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		hit := false
		if strings.Contains(title, tok) {
			score += 5
			hit = true
		}
		if strings.Contains(breadcrumb, tok) {
			score += 3
			hit = true
		}
		for _, h := range headings {
			if strings.Contains(h, tok) {
				score += 2
				hit = true
				break
			}
		}
		if strings.Contains(path, tok) {
			score += 1
			hit = true
		}
		if hit {
			matched++
		}
	}
	if matched > 1 {
		score += matched * 3
	}
	return score
}

// truncateChars 按字符（rune）截断到 max 个，超出追加 "..."（防切断多字节 UTF-8）。对齐 api.rs:43-57。
func truncateChars(s string, max int) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
