package dingtalk

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestReconstructReplyCtx(t *testing.T) {
	cases := []struct {
		scope   string
		ok      bool
		isGroup bool
		convID  string
		staffID string
	}{
		{"dingtalk:g:cidABC:staff1", true, true, "cidABC", "staff1"},
		{"dingtalk:d:cidXYZ:staff2", true, false, "cidXYZ", "staff2"},
		{"dingtalk:g:cidABC", true, true, "cidABC", ""}, // 无 senderStaffId
		{"dingtalk:x:cid:staff", false, false, "", ""},  // 非法 convType
		{"dingtalk:g:", false, false, "", ""},           // 空 conversationId
		{"qq:1:2", false, false, "", ""},                // 非 dingtalk
		{"dingtalk:g", false, false, "", ""},            // 段数不足
	}
	for _, c := range cases {
		rc, ok := reconstructReplyCtx(c.scope)
		if ok != c.ok {
			t.Fatalf("reconstructReplyCtx(%q) ok=%v, want %v", c.scope, ok, c.ok)
		}
		if !ok {
			continue
		}
		if rc.isGroup != c.isGroup || rc.conversationID != c.convID || rc.senderStaffID != c.staffID || !rc.proactive {
			t.Fatalf("reconstructReplyCtx(%q) = %#v", c.scope, rc)
		}
	}
}

func TestParseReplyContext(t *testing.T) {
	p := &Platform{}
	// *replyContext 透传。
	rc := &replyContext{sessionWebhook: "wh", conversationID: "c"}
	if got, ok := p.parseReplyContext(rc); !ok || got != rc {
		t.Fatalf("*replyContext passthrough failed")
	}
	// replyContext 值取地址。
	if got, ok := p.parseReplyContext(replyContext{conversationID: "c2"}); !ok || got.conversationID != "c2" {
		t.Fatalf("replyContext value failed: %#v %v", got, ok)
	}
	// string 走 scope 还原。
	if got, ok := p.parseReplyContext("dingtalk:d:cid:staff"); !ok || got.senderStaffID != "staff" || got.isGroup {
		t.Fatalf("string scope failed: %#v %v", got, ok)
	}
	// 未知类型。
	if _, ok := p.parseReplyContext(123); ok {
		t.Fatalf("unknown type should fail")
	}
}

func TestExtractRichText(t *testing.T) {
	content := map[string]any{
		"richText": []any{
			map[string]any{"text": "你好"},
			map[string]any{"downloadCode": "xxx"}, // 非文本片段跳过
			map[string]any{"text": " 世界 "},
		},
	}
	if got := extractRichText(content); got != "你好 世界" {
		t.Fatalf("extractRichText = %q, want 你好 世界", got)
	}
	if got := extractRichText("not a map"); got != "" {
		t.Fatalf("extractRichText(non-map) = %q, want empty", got)
	}
	if got := extractRichText(map[string]any{"foo": "bar"}); got != "" {
		t.Fatalf("extractRichText(no richText) = %q, want empty", got)
	}
}

func TestFormatReplyContent(t *testing.T) {
	// 引用文本消息：拼成「引用: "..."」+ 正文。
	quoted, _ := json.Marshal(repliedTextContent{Text: "原始问题"})
	rt := &richTextContent{
		Content:    "这是回复",
		IsReplyMsg: true,
		RepliedMsg: &repliedMessage{MsgType: "text", Content: quoted},
	}
	got := formatReplyContent(rt, "")
	want := "引用: \"原始问题\"\n\n这是回复"
	if got != want {
		t.Fatalf("formatReplyContent = %q, want %q", got, want)
	}

	// 无 repliedMsg：原样返回正文。
	if got := formatReplyContent(&richTextContent{Content: "纯文本"}, ""); got != "纯文本" {
		t.Fatalf("no reply = %q, want 纯文本", got)
	}

	// Content 空时回退到 fallback。
	if got := formatReplyContent(&richTextContent{}, "兜底正文"); got != "兜底正文" {
		t.Fatalf("fallback = %q, want 兜底正文", got)
	}

	// 引用非文本类型（如 interactiveCard）：不展开，只返回正文。
	rtCard := &richTextContent{
		Content:    "看这个",
		IsReplyMsg: true,
		RepliedMsg: &repliedMessage{MsgType: "interactiveCard", Content: json.RawMessage(`{}`)},
	}
	if got := formatReplyContent(rtCard, ""); got != "看这个" {
		t.Fatalf("non-text quote = %q, want 看这个", got)
	}
}

func TestNormalizeQuotedMessageText(t *testing.T) {
	if got := normalizeQuotedMessageText("  hi  "); got != "hi" {
		t.Fatalf("trim = %q", got)
	}
	long := strings.Repeat("字", maxQuotedMessageRunes+10)
	got := normalizeQuotedMessageText(long)
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("long text not truncated")
	}
	if len([]rune(got)) != maxQuotedMessageRunes+3 {
		t.Fatalf("truncated rune count = %d", len([]rune(got)))
	}
}

func TestExtractAtUserIds(t *testing.T) {
	ids := extractAtUserIds("你好 @194252073827812352 和 @123 再 @194252073827812352 忽略 @ab")
	// 4 位以上数字才算；去重；@ab 不匹配；@123 太短不匹配。
	if len(ids) != 1 || ids[0] != "194252073827812352" {
		t.Fatalf("extractAtUserIds = %#v", ids)
	}
	if got := extractAtUserIds("无提及"); len(got) != 0 {
		t.Fatalf("no mention = %#v", got)
	}
}

func TestPreprocessDingTalkMarkdown(t *testing.T) {
	// 非空行之间的单个 \n 应补两空格强制换行。
	got := preprocessDingTalkMarkdown("第一行\n第二行")
	if got != "第一行  \n第二行" {
		t.Fatalf("line break = %q", got)
	}
	// 行首空格转不换行空格（ ）。
	got = preprocessDingTalkMarkdown("  缩进")
	if !strings.HasPrefix(got, "  ") {
		t.Fatalf("indent not converted to nbsp: %q", got)
	}
	// 空行相邻不补两空格。
	got = preprocessDingTalkMarkdown("行\n\n段")
	if strings.Contains(got, "行  \n") {
		t.Fatalf("empty-line neighbor should not get forced break: %q", got)
	}
}

func TestIsDuplicate(t *testing.T) {
	p := &Platform{dedup: map[string]time.Time{}}
	if p.isDuplicate("") {
		t.Fatalf("empty id must not be duplicate")
	}
	if p.isDuplicate("m1") {
		t.Fatalf("first sighting must not be duplicate")
	}
	if !p.isDuplicate("m1") {
		t.Fatalf("second sighting must be duplicate")
	}
	// 过期项不再算重复。
	p.dedup["m1"] = time.Now().Add(-dedupTTL - time.Minute)
	if p.isDuplicate("m1") {
		t.Fatalf("stale entry must not count as duplicate")
	}
}

// New 校验：缺 client_id/client_secret 报错；robot_code 缺省回退 client_id。
func TestNew(t *testing.T) {
	if _, err := New(map[string]any{"client_id": "cid"}); err == nil {
		t.Fatalf("missing client_secret should error")
	}
	pIface, err := New(map[string]any{"client_id": "cid", "client_secret": "sec"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	p := pIface.(*Platform)
	if p.robotCode != "cid" {
		t.Fatalf("robot_code fallback = %q, want cid", p.robotCode)
	}
	pIface2, _ := New(map[string]any{"client_id": "cid", "client_secret": "sec", "robot_code": "rc"})
	if pIface2.(*Platform).robotCode != "rc" {
		t.Fatalf("explicit robot_code lost")
	}
}
