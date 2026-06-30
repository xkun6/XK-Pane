package qqbot

import (
	"testing"
	"time"
)

func TestReconstructReplyCtx(t *testing.T) {
	cases := []struct {
		name    string
		scope   string
		wantTyp string
		group   string
		user    string
		ok      bool
	}{
		{"group", "qqbot:grp1:mem1", "group", "grp1", "mem1", true},
		{"c2c when chat==user", "qqbot:usr1:usr1", "c2c", "", "usr1", true},
		{"wrong platform", "feishu:a:b", "", "", "", false},
		{"too few parts", "qqbot:a", "", "", "", false},
		{"empty chat", "qqbot::b", "", "", "", false},
	}
	for _, c := range cases {
		rctx, ok := reconstructReplyCtx(c.scope)
		if ok != c.ok {
			t.Fatalf("%s: ok=%v want %v", c.name, ok, c.ok)
		}
		if !c.ok {
			continue
		}
		if rctx.messageType != c.wantTyp || rctx.groupOpenID != c.group || rctx.userOpenID != c.user {
			t.Fatalf("%s: rctx=%#v", c.name, rctx)
		}
		if rctx.scopeKey != c.scope {
			t.Fatalf("%s: scopeKey=%q want %q", c.name, rctx.scopeKey, c.scope)
		}
	}
}

func TestStripAtMention(t *testing.T) {
	cases := map[string]string{
		"<@!123456> hello":   "hello",
		"  <@!9> 部署吗 ":       "部署吗",
		"no mention here":    "no mention here",
		"  trimmed  ":        "trimmed",
		"<@!1>":              "",
	}
	for in, want := range cases {
		if got := stripAtMention(in); got != want {
			t.Fatalf("stripAtMention(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestChunkRunes(t *testing.T) {
	if got := chunkRunes("abc", 10); len(got) != 1 || got[0] != "abc" {
		t.Fatalf("short chunk = %#v", got)
	}
	// 多字节按 rune 切分，不会切坏字符。
	chunks := chunkRunes("一二三四五", 2)
	if len(chunks) != 3 || chunks[0] != "一二" || chunks[1] != "三四" || chunks[2] != "五" {
		t.Fatalf("rune chunks = %#v", chunks)
	}
}

func TestNextMsgSeqIncrementsPerEvent(t *testing.T) {
	p := &Platform{msgSeqMap: map[string]*msgSeqEntry{}}
	if s := p.nextMsgSeq("evt-1"); s != 1 {
		t.Fatalf("first seq = %d, want 1", s)
	}
	if s := p.nextMsgSeq("evt-1"); s != 2 {
		t.Fatalf("second seq = %d, want 2", s)
	}
	if s := p.nextMsgSeq("evt-2"); s != 1 {
		t.Fatalf("new event seq = %d, want 1", s)
	}
}

func TestRecentMsgIDWindow(t *testing.T) {
	p := &Platform{recent: map[string]recentMsg{}}
	if id := p.recentMsgID("scope-a"); id != "" {
		t.Fatalf("unknown scope id = %q, want empty", id)
	}
	p.rememberMsg("scope-a", "msg-1")
	if id := p.recentMsgID("scope-a"); id != "msg-1" {
		t.Fatalf("fresh id = %q, want msg-1", id)
	}
	// 过期则不再复用为 passive reply。
	p.recent["scope-a"] = recentMsg{id: "msg-old", at: time.Now().Add(-passiveReplyWindow - time.Minute)}
	if id := p.recentMsgID("scope-a"); id != "" {
		t.Fatalf("stale id = %q, want empty", id)
	}
}

// parseReplyContext 在主动推送（scopeKey 字符串）时，应把窗口内的最近来信 msg_id 回填为
// passive reply，提升 QQ 主动推送的送达率。
func TestParseReplyContextFillsRecentMsgID(t *testing.T) {
	p := &Platform{recent: map[string]recentMsg{}}
	p.rememberMsg("qqbot:grp1:mem1", "msg-9")
	rctx, ok := p.parseReplyContext("qqbot:grp1:mem1")
	if !ok {
		t.Fatalf("parseReplyContext ok=false")
	}
	if rctx.eventMsgID != "msg-9" {
		t.Fatalf("eventMsgID = %q, want msg-9 (passive reply 回填)", rctx.eventMsgID)
	}
}
