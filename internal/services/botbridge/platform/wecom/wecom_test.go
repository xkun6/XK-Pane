package wecom

import (
	"strings"
	"testing"
	"time"
)

func TestReconstructReplyCtx(t *testing.T) {
	cases := []struct {
		name   string
		scope  string
		chatID string
		userID string
		ok     bool
	}{
		{"group", "wecom:chat1:user1", "chat1", "user1", true},
		{"single chat==user", "wecom:user1:user1", "user1", "user1", true},
		{"wrong platform", "qq:1:2", "", "", false},
		{"too few parts", "wecom:only", "", "", false},
		{"empty chat", "wecom::user1", "", "", false},
	}
	for _, c := range cases {
		rc, ok := reconstructReplyCtx(c.scope)
		if ok != c.ok {
			t.Fatalf("%s: ok=%v want %v", c.name, ok, c.ok)
		}
		if !c.ok {
			continue
		}
		if rc.chatID != c.chatID || rc.userID != c.userID || rc.scopeKey != c.scope {
			t.Fatalf("%s: rc=%#v", c.name, rc)
		}
		if rc.reqID != "" {
			t.Fatalf("%s: reconstructed ctx must have empty reqID, got %q", c.name, rc.reqID)
		}
	}
}

func TestStripWeComAtMentions(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"@bot123 部署吗", "部署吗"},
		{"＠bot123 重启服务", "重启服务"},
		{"@张三 @bot123 /list", "/list"},
		{"没有提及", "没有提及"},
		{"  多余   空格  ", "多余 空格"},
	}
	for _, c := range cases {
		if got := stripWeComAtMentions(c.in, "bot123"); got != c.want {
			t.Fatalf("stripWeComAtMentions(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCollectText(t *testing.T) {
	// 主体文本。
	b := &wsMsgCallbackBody{MsgType: "text"}
	b.Text.Content = "你好"
	if got := collectText(b); got != "你好" {
		t.Fatalf("text body = %q, want 你好", got)
	}

	// mixed 文本段拼接，忽略非文本段。
	mixed := &wsMsgCallbackBody{
		MsgType: "mixed",
		Mixed: &wsMixedBlock{MsgItem: []wsMixedItem{
			{MsgType: "text", Text: &struct {
				Content string `json:"content"`
			}{Content: "第一段"}},
			{MsgType: "image"},
			{MsgType: "text", Text: &struct {
				Content string `json:"content"`
			}{Content: "第二段"}},
		}},
	}
	if got := collectText(mixed); got != "第一段\n第二段" {
		t.Fatalf("mixed body = %q, want 第一段\\n第二段", got)
	}

	// 语音转写当作文本。
	voice := &wsMsgCallbackBody{MsgType: "voice"}
	voice.Voice.Content = "转写文本"
	if got := collectText(voice); got != "转写文本" {
		t.Fatalf("voice body = %q, want 转写文本", got)
	}
}

func TestSplitByBytes(t *testing.T) {
	// 不超限不切。
	if got := splitByBytes("短文本", 2000); len(got) != 1 || got[0] != "短文本" {
		t.Fatalf("short = %#v", got)
	}
	// 切分后拼接还原，且每片不超字节上限。
	long := strings.Repeat("一二三四五\n", 500) // 每个汉字 3 字节
	chunks := splitByBytes(long, 100)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > 100 {
			t.Fatalf("chunk %d exceeds limit: %d bytes", i, len(c))
		}
	}
	if strings.Join(chunks, "") != long {
		t.Fatalf("rejoined chunks != original")
	}
}

func TestIsDuplicate(t *testing.T) {
	p := &Platform{dedup: map[string]time.Time{}}
	if p.isDuplicate("") {
		t.Fatalf("empty msgID must not be duplicate")
	}
	if p.isDuplicate("m1") {
		t.Fatalf("first sighting must not be duplicate")
	}
	if !p.isDuplicate("m1") {
		t.Fatalf("second sighting must be duplicate")
	}
	// 过期条目不再算重复。
	p.dedup["m1"] = time.Now().Add(-dedupTTL - time.Minute)
	if p.isDuplicate("m1") {
		t.Fatalf("stale entry must not count as duplicate")
	}
}
