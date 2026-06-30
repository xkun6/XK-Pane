package qq

import (
	"encoding/json"
	"testing"
)

func TestReconstructReplyCtx(t *testing.T) {
	cases := []struct {
		name    string
		scope   string
		wantTyp string
		group   int64
		user    int64
		ok      bool
	}{
		{"group", "qq:111:222", "group", 111, 222, true},
		{"private when chat==user", "qq:333:333", "private", 0, 333, true},
		{"wrong platform", "qqbot:1:2", "", 0, 0, false},
		{"too few parts", "qq:1", "", 0, 0, false},
		{"empty chat", "qq::2", "", 0, 0, false},
	}
	for _, c := range cases {
		rctx, ok := reconstructReplyCtx(c.scope)
		if ok != c.ok {
			t.Fatalf("%s: ok=%v want %v", c.name, ok, c.ok)
		}
		if !c.ok {
			continue
		}
		if rctx.messageType != c.wantTyp || rctx.groupID != c.group || rctx.userID != c.user {
			t.Fatalf("%s: rctx=%#v", c.name, rctx)
		}
		if rctx.scopeKey != c.scope {
			t.Fatalf("%s: scopeKey=%q want %q", c.name, rctx.scopeKey, c.scope)
		}
	}
}

func TestStripCQCodes(t *testing.T) {
	cases := map[string]string{
		"hello [CQ:at,qq=123] world": "hello  world",
		"[CQ:image,file=a.jpg]only":  "only",
		"no cq codes":                "no cq codes",
		"[CQ:face,id=1]":             "",
	}
	for in, want := range cases {
		if got := stripCQCodes(in); got != want {
			t.Fatalf("stripCQCodes(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTextSegments(t *testing.T) {
	// 段数组：取 text 段，忽略 image / at。
	payload := map[string]any{
		"message": []any{
			map[string]any{"type": "text", "data": map[string]any{"text": "请 "}},
			map[string]any{"type": "at", "data": map[string]any{"qq": "123"}},
			map[string]any{"type": "text", "data": map[string]any{"text": "部署吗"}},
			map[string]any{"type": "image", "data": map[string]any{"url": "http://x"}},
		},
	}
	if got := parseText(payload); got != "请 部署吗" {
		t.Fatalf("segment parse = %q, want %q", got, "请 部署吗")
	}

	// raw_message 字符串：剥 CQ 码。
	str := map[string]any{"message": "[CQ:at,qq=1]部署吗"}
	if got := parseText(str); got != "部署吗" {
		t.Fatalf("string parse = %q, want %q", got, "部署吗")
	}

	// 纯图片（无文本）→ 空，handleMessage 会忽略。
	imgOnly := map[string]any{
		"message": []any{
			map[string]any{"type": "image", "data": map[string]any{"url": "http://x"}},
		},
	}
	if got := parseText(imgOnly); got != "" {
		t.Fatalf("image-only parse = %q, want empty", got)
	}
}

func TestJSONInt64(t *testing.T) {
	if v := jsonInt64(map[string]any{"id": float64(42)}, "id"); v != 42 {
		t.Fatalf("float64 = %d, want 42", v)
	}
	if v := jsonInt64(map[string]any{"id": json.Number("99")}, "id"); v != 99 {
		t.Fatalf("json.Number = %d, want 99", v)
	}
	if v := jsonInt64(map[string]any{}, "id"); v != 0 {
		t.Fatalf("missing = %d, want 0", v)
	}
}
