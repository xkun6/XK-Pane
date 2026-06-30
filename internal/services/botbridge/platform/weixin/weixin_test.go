package weixin

import (
	"strings"
	"testing"
	"time"
)

func TestPeerFromScope(t *testing.T) {
	cases := []struct {
		scope string
		peer  string
		ok    bool
	}{
		{"weixin:dm:userA", "userA", true},
		{"weixin:dm:wxid_123", "wxid_123", true},
		{"weixin:dm:", "", false},
		{"qq:1:2", "", false},
		{"weixin:userA", "", false},
	}
	for _, c := range cases {
		peer, ok := peerFromScope(c.scope)
		if ok != c.ok || peer != c.peer {
			t.Fatalf("peerFromScope(%q) = (%q, %v), want (%q, %v)", c.scope, peer, ok, c.peer, c.ok)
		}
	}
}

func TestBodyFromItemList(t *testing.T) {
	// 纯文本。
	text := []messageItem{{Type: messageItemText, TextItem: &textItem{Text: "  你好  "}}}
	if got := bodyFromItemList(text); got != "你好" {
		t.Fatalf("text = %q, want 你好", got)
	}

	// 语音 ASR 文本。
	voice := []messageItem{{Type: messageItemVoice, VoiceItem: &voiceItem{Text: "转写内容"}}}
	if got := bodyFromItemList(voice); got != "转写内容" {
		t.Fatalf("voice = %q, want 转写内容", got)
	}

	// 引用文本消息：拼成 [引用: ...] 形式。
	quoteText := []messageItem{{
		Type:     messageItemText,
		TextItem: &textItem{Text: "收到"},
		RefMsg: &refMessage{
			Title:       "张三",
			MessageItem: &messageItem{Type: messageItemText, TextItem: &textItem{Text: "原文"}},
		},
	}}
	if got := bodyFromItemList(quoteText); got != "[引用: 张三 | 原文]\n收到" {
		t.Fatalf("quote = %q", got)
	}

	// 引用媒体消息：只返回正文（媒体不展开）。
	quoteMedia := []messageItem{{
		Type:     messageItemText,
		TextItem: &textItem{Text: "看这个"},
		RefMsg:   &refMessage{MessageItem: &messageItem{Type: messageItemImage}},
	}}
	if got := bodyFromItemList(quoteMedia); got != "看这个" {
		t.Fatalf("quote-media = %q, want 看这个", got)
	}

	// 空。
	if got := bodyFromItemList(nil); got != "" {
		t.Fatalf("empty = %q, want empty", got)
	}
}

func TestSplitUTF8(t *testing.T) {
	if got := splitUTF8("短", 100); len(got) != 1 || got[0] != "短" {
		t.Fatalf("short = %#v", got)
	}
	chunks := splitUTF8("一二三四五", 2)
	if len(chunks) != 3 || chunks[0] != "一二" || chunks[1] != "三四" || chunks[2] != "五" {
		t.Fatalf("chunks = %#v", chunks)
	}
	if strings.Join(chunks, "") != "一二三四五" {
		t.Fatalf("rejoin mismatch")
	}
}

func TestContextTokenRoundTrip(t *testing.T) {
	p := &Platform{tokens: map[string]string{}}
	if got := p.getContextToken("peerA"); got != "" {
		t.Fatalf("unknown peer token = %q, want empty", got)
	}
	p.setContextToken("peerA", "tok-1")
	if got := p.getContextToken("peerA"); got != "tok-1" {
		t.Fatalf("token = %q, want tok-1", got)
	}
	// 空 peer / 空 token 不写入。
	p.setContextToken("", "x")
	p.setContextToken("peerB", "")
	if got := p.getContextToken("peerB"); got != "" {
		t.Fatalf("peerB token = %q, want empty", got)
	}
}

func TestIsDuplicate(t *testing.T) {
	p := &Platform{dedup: map[string]time.Time{}}
	if p.isDuplicate("k1") {
		t.Fatalf("first sighting must not be duplicate")
	}
	if !p.isDuplicate("k1") {
		t.Fatalf("second sighting must be duplicate")
	}
	p.dedup["k1"] = time.Now().Add(-dedupTTL - time.Minute)
	if p.isDuplicate("k1") {
		t.Fatalf("stale entry must not count as duplicate")
	}
}

// parseReplyContext 在主动推送（scopeKey 字符串）时应回填该 peer 缓存的 context_token。
func TestParseReplyContextFillsToken(t *testing.T) {
	p := &Platform{tokens: map[string]string{}}
	p.setContextToken("peerA", "tok-9")
	rc, ok := p.parseReplyContext("weixin:dm:peerA")
	if !ok {
		t.Fatalf("parseReplyContext ok=false")
	}
	if rc.peerUserID != "peerA" || rc.contextToken != "tok-9" {
		t.Fatalf("rc = %#v", rc)
	}
}
