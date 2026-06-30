package feishu

import (
	"encoding/json"

	"xkpane/internal/services/botbridge"
	"xkpane/internal/utils/logx"
)

func plainText(content string) map[string]any {
	return map[string]any{"tag": "plain_text", "content": content}
}

func renderCard(card *botbridge.Card, scopeKey string) string {
	b, err := json.Marshal(renderCardMap(card, scopeKey))
	if err != nil {
		logx.For("botbridge.feishu").Error("render card failed", "err", err)
		return `{"config":{"wide_screen_mode":true},"elements":[]}`
	}
	return string(b)
}

func renderCardMap(card *botbridge.Card, scopeKey string) map[string]any {
	result := map[string]any{"config": map[string]any{"wide_screen_mode": true}}
	if card == nil {
		return result
	}
	if card.Header != nil && card.Header.Title != "" {
		color := card.Header.Color
		if color == "" {
			color = "blue"
		}
		result["header"] = map[string]any{
			"title":    plainText(card.Header.Title),
			"template": color,
		}
	}
	elements := []map[string]any{}
	for _, elem := range card.Elements {
		switch e := elem.(type) {
		case botbridge.CardMarkdown:
			elements = append(elements, map[string]any{"tag": "markdown", "content": e.Content})
		case botbridge.CardDivider:
			elements = append(elements, map[string]any{"tag": "hr"})
		case botbridge.CardActions:
			actions := []map[string]any{}
			for _, btn := range e.Buttons {
				btnType := btn.Type
				if btnType == "" {
					btnType = "default"
				}
				value := map[string]string{"action": btn.Value}
				if scopeKey != "" {
					value["scope_key"] = scopeKey
				}
				for k, v := range btn.Extra {
					value[k] = v
				}
				action := map[string]any{
					"tag":   "button",
					"text":  plainText(btn.Text),
					"type":  btnType,
					"value": value,
				}
				if e.Layout == botbridge.CardActionLayoutEqualColumns {
					action["width"] = "fill"
				}
				actions = append(actions, action)
			}
			if len(actions) > 0 {
				elements = append(elements, map[string]any{"tag": "action", "actions": actions})
			}
		case botbridge.CardListItem:
			btnType := e.BtnType
			if btnType == "" {
				btnType = "default"
			}
			value := map[string]string{"action": e.BtnValue}
			if scopeKey != "" {
				value["scope_key"] = scopeKey
			}
			for k, v := range e.Extra {
				value[k] = v
			}
			elements = append(elements, map[string]any{
				"tag":       "column_set",
				"flex_mode": "none",
				"columns": []map[string]any{
					{
						"tag":            "column",
						"width":          "weighted",
						"weight":         5,
						"vertical_align": "center",
						"elements":       []map[string]any{{"tag": "markdown", "content": e.Text}},
					},
					{
						"tag":            "column",
						"width":          "auto",
						"vertical_align": "center",
						"elements": []map[string]any{{
							"tag":   "button",
							"text":  plainText(e.BtnText),
							"type":  btnType,
							"value": value,
						}},
					},
				},
			})
		case botbridge.CardNote:
			elements = append(elements, map[string]any{
				"tag":      "note",
				"elements": []map[string]any{plainText(e.Text)},
			})
		}
	}
	if len(elements) == 0 {
		elements = append(elements, map[string]any{"tag": "markdown", "content": " "})
	}
	result["elements"] = elements
	return result
}
