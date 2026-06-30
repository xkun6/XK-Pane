package feishu

import (
	"encoding/json"
	"testing"

	"xkpane/internal/services/botbridge"
)

// TestRenderCardActionSchema 锁住飞书交互卡的 schema：按钮必须渲染成 tag:action 模块，
// 按钮文案走 plain_text，value 带 action/scope_key 并合并 Extra。任一处漂移都会让飞书
// 卡片不出按钮（此前「多问题卡只有文字」即此类问题），故作为回归硬断言。
func TestRenderCardActionSchema(t *testing.T) {
	card := botbridge.NewCard().
		Title("AskUserQuestion (1/3)", "orange").
		Markdown("选一个").
		Buttons(
			botbridge.CardButton{Text: "选项A", Type: "default", Value: "askq:0:0", Extra: map[string]string{"askq_label": "选项A"}},
			botbridge.CardButton{Text: "选项B", Type: "default", Value: "askq:0:1"},
		).
		Build()

	var m map[string]any
	if err := json.Unmarshal([]byte(renderCard(card, "feishu:oc_x:ou_y")), &m); err != nil {
		t.Fatalf("renderCard 输出非法 JSON: %v", err)
	}

	cfg, _ := m["config"].(map[string]any)
	if cfg == nil || cfg["wide_screen_mode"] != true {
		t.Fatalf("config = %v, want wide_screen_mode=true", m["config"])
	}

	header, _ := m["header"].(map[string]any)
	if header == nil {
		t.Fatalf("header 缺失")
	}
	if header["template"] != "orange" {
		t.Fatalf("header.template = %v, want orange", header["template"])
	}
	title, _ := header["title"].(map[string]any)
	if title == nil || title["tag"] != "plain_text" || title["content"] != "AskUserQuestion (1/3)" {
		t.Fatalf("header.title = %v", header["title"])
	}

	elements, _ := m["elements"].([]any)
	var md, action map[string]any
	actionCount := 0
	for _, e := range elements {
		em, _ := e.(map[string]any)
		switch em["tag"] {
		case "markdown":
			md = em
		case "action":
			action = em
			actionCount++
		}
	}
	if md == nil || md["content"] != "选一个" {
		t.Fatalf("markdown 元素 = %v", md)
	}
	if action == nil {
		t.Fatalf("action 元素缺失——按钮没渲染成 tag:action")
	}
	// 同一行的两个按钮应聚成「一个」action 模块（飞书按 action 模块数量降级，多按钮挤一行才安全）。
	if actionCount != 1 {
		t.Fatalf("action 模块数 = %d, want 1（两按钮同属一个 action 行）", actionCount)
	}
	actions, _ := action["actions"].([]any)
	if len(actions) != 2 {
		t.Fatalf("action.actions = %d, want 2 个按钮", len(actions))
	}

	btn0, _ := actions[0].(map[string]any)
	if btn0["tag"] != "button" || btn0["type"] != "default" {
		t.Fatalf("button0 = %v", btn0)
	}
	btnText, _ := btn0["text"].(map[string]any)
	if btnText == nil || btnText["tag"] != "plain_text" || btnText["content"] != "选项A" {
		t.Fatalf("button0.text = %v", btn0["text"])
	}
	val, _ := btn0["value"].(map[string]any)
	if val == nil || val["action"] != "askq:0:0" {
		t.Fatalf("button0.value.action = %v, want askq:0:0", btn0["value"])
	}
	if val["scope_key"] != "feishu:oc_x:ou_y" {
		t.Fatalf("button0.value.scope_key = %v, want 注入的 scopeKey", val["scope_key"])
	}
	if val["askq_label"] != "选项A" {
		t.Fatalf("button0.value.askq_label = %v, want Extra 合并进 value", val["askq_label"])
	}
}

// TestRenderCardEmptyFallback 空卡必须兜底成一个占位 markdown，否则飞书会因 elements 为空报错拒收。
func TestRenderCardEmptyFallback(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal([]byte(renderCard(botbridge.NewCard().Build(), "")), &m); err != nil {
		t.Fatalf("renderCard 输出非法 JSON: %v", err)
	}
	elements, _ := m["elements"].([]any)
	if len(elements) != 1 {
		t.Fatalf("空卡 elements = %d, want 1 个占位", len(elements))
	}
	el0, _ := elements[0].(map[string]any)
	if el0["tag"] != "markdown" {
		t.Fatalf("占位元素 = %v, want markdown", el0)
	}
}
