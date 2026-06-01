package bedrock

import "testing"

func sp(s string) *string { return &s }

// 直接移植 fuzzy.rs 的单测期望值，保证打分与 cc-pane 逐分一致。

func TestNameScore_ExactPrefixContains(t *testing.T) {
	if got := nameScore("Foo", "foo", nil); got != 20 {
		t.Errorf("exact: got %d want 20", got)
	}
	if got := nameScore("foo", "fooBar", nil); got != 15 {
		t.Errorf("prefix: got %d want 15", got)
	}
	if got := nameScore("Bar", "fooBar", nil); got != 10 {
		t.Errorf("contains: got %d want 10", got)
	}
	if got := nameScore("Baz", "fooBar", nil); got != 0 {
		t.Errorf("none: got %d want 0", got)
	}
}

func TestNameScore_BlobOnly(t *testing.T) {
	if got := nameScore("widget", "Game", sp("Spawns a widget on screen")); got != 3 {
		t.Errorf("blob-only: got %d want 3", got)
	}
}

func TestNameScore_BlobBonusAdditive(t *testing.T) {
	// name contains (10) + blob also contains (+3) = 13.
	if got := nameScore("entity", "SpawnEntity", sp("Entity helper used at runtime")); got != 13 {
		t.Errorf("blob-bonus: got %d want 13", got)
	}
}

func TestNameScore_EmptyQueryZero(t *testing.T) {
	if got := nameScore("", "anything", sp("anything")); got != 0 {
		t.Errorf("empty query: got %d want 0", got)
	}
}

func TestGuideScore_TitleBeatsPath(t *testing.T) {
	e := &GuideEntry{
		Title:      "Spawn entity guide",
		Path:       "mcguide/spawn/something.md",
		Source:     "mcguide",
		Breadcrumb: "mcguide / spawn",
		Headings:   []string{"Overview"},
	}
	// title +5, breadcrumb +3, path +1（单 token 无 bonus）= 9。
	if got := guideScore([]string{"spawn"}, e); got != 9 {
		t.Errorf("got %d want 9", got)
	}
}

func TestGuideScore_MultiTokenBonus(t *testing.T) {
	e := &GuideEntry{
		Title:      "Spawn Entity in Server",
		Path:       "mcguide/spawn/server.md",
		Source:     "mcguide",
		Breadcrumb: "mcguide / spawn / server",
		Headings:   []string{"Spawn API", "Server Code"},
	}
	// spawn: 5+3+2+1=11；server: 5+3+2+1=11；base 22 + matched(2)*3 = 28。
	if got := guideScore([]string{"spawn", "server"}, e); got != 28 {
		t.Errorf("got %d want 28", got)
	}
}

func TestGuideScore_NoMatch(t *testing.T) {
	e := &GuideEntry{Title: "Unrelated", Path: "mcguide/other.md", Source: "mcguide", Breadcrumb: "mcguide / other", Headings: []string{"nothing"}}
	if got := guideScore([]string{"spawn"}, e); got != 0 {
		t.Errorf("got %d want 0", got)
	}
}

func TestTruncateChars(t *testing.T) {
	if got := truncateChars("", 80); got != "" {
		t.Errorf("empty: %q", got)
	}
	if got := truncateChars("abc", 80); got != "abc" {
		t.Errorf("under: %q", got)
	}
	if got := truncateChars("abcde", 5); got != "abcde" {
		t.Errorf("exact: %q want unchanged", got)
	}
	if got := truncateChars("abcdef", 5); got != "abcde..." {
		t.Errorf("over: %q want abcde...", got)
	}
	// CJK：按 rune 截断不切多字节。3 个汉字截到 2 → 2 汉字 + "..."。
	if got := truncateChars("一二三", 2); got != "一二..." {
		t.Errorf("cjk: %q want 一二...", got)
	}
}
