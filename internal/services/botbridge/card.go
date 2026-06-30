package botbridge

import (
	"fmt"
	"strings"
)

type Card struct {
	Header   *CardHeader
	Elements []CardElement
}

type CardHeader struct {
	Title string
	Color string
}

type CardElement interface{ cardElement() }

type CardMarkdown struct{ Content string }
type CardDivider struct{}
type CardActions struct {
	Buttons []CardButton
	Layout  CardActionLayout
}
type CardNote struct{ Text string }
type CardListItem struct {
	Text     string
	BtnText  string
	BtnType  string
	BtnValue string
	Extra    map[string]string
}

func (CardMarkdown) cardElement() {}
func (CardDivider) cardElement()  {}
func (CardActions) cardElement()  {}
func (CardNote) cardElement()     {}
func (CardListItem) cardElement() {}

type CardButton struct {
	Text  string
	Type  string
	Value string
	Extra map[string]string
}

type CardActionLayout string

const (
	CardActionLayoutRow          CardActionLayout = "row"
	CardActionLayoutEqualColumns CardActionLayout = "equal_columns"
)

func PrimaryBtn(text, value string) CardButton {
	return CardButton{Text: text, Type: "primary", Value: value}
}
func DefaultBtn(text, value string) CardButton {
	return CardButton{Text: text, Type: "default", Value: value}
}
func DangerBtn(text, value string) CardButton {
	return CardButton{Text: text, Type: "danger", Value: value}
}

type CardBuilder struct{ card Card }

func NewCard() *CardBuilder { return &CardBuilder{} }

func (b *CardBuilder) Title(title, color string) *CardBuilder {
	b.card.Header = &CardHeader{Title: title, Color: color}
	return b
}

func (b *CardBuilder) Markdown(content string) *CardBuilder {
	if content != "" {
		b.card.Elements = append(b.card.Elements, CardMarkdown{Content: content})
	}
	return b
}

func (b *CardBuilder) Markdownf(format string, args ...any) *CardBuilder {
	return b.Markdown(fmt.Sprintf(format, args...))
}

func (b *CardBuilder) Divider() *CardBuilder {
	b.card.Elements = append(b.card.Elements, CardDivider{})
	return b
}

func (b *CardBuilder) Buttons(buttons ...CardButton) *CardBuilder {
	if len(buttons) > 0 {
		b.card.Elements = append(b.card.Elements, CardActions{Buttons: buttons, Layout: CardActionLayoutRow})
	}
	return b
}

func (b *CardBuilder) ButtonsEqual(buttons ...CardButton) *CardBuilder {
	if len(buttons) > 0 {
		b.card.Elements = append(b.card.Elements, CardActions{Buttons: buttons, Layout: CardActionLayoutEqualColumns})
	}
	return b
}

func (b *CardBuilder) ListItem(desc, btnText, btnValue string) *CardBuilder {
	b.card.Elements = append(b.card.Elements, CardListItem{
		Text: desc, BtnText: btnText, BtnType: "default", BtnValue: btnValue,
	})
	return b
}

func (b *CardBuilder) Note(text string) *CardBuilder {
	if text != "" {
		b.card.Elements = append(b.card.Elements, CardNote{Text: text})
	}
	return b
}

func (b *CardBuilder) Build() *Card {
	c := b.card
	return &c
}

func (c *Card) RenderText() string {
	if c == nil {
		return ""
	}
	var sb strings.Builder
	if c.Header != nil && c.Header.Title != "" {
		sb.WriteString("**")
		sb.WriteString(c.Header.Title)
		sb.WriteString("**\n\n")
	}
	for _, elem := range c.Elements {
		switch e := elem.(type) {
		case CardMarkdown:
			sb.WriteString(e.Content)
			sb.WriteString("\n\n")
		case CardDivider:
			sb.WriteString("---\n\n")
		case CardActions:
			for i, btn := range e.Buttons {
				if i > 0 {
					sb.WriteString("  ")
				}
				sb.WriteString("[")
				sb.WriteString(btn.Text)
				sb.WriteString("]")
			}
			sb.WriteString("\n\n")
		case CardListItem:
			sb.WriteString(e.Text)
			sb.WriteString("  [")
			sb.WriteString(e.BtnText)
			sb.WriteString("]\n")
		case CardNote:
			sb.WriteString(e.Text)
			sb.WriteString("\n")
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (c *Card) CollectButtons() [][]ButtonOption {
	var rows [][]ButtonOption
	if c == nil {
		return rows
	}
	for _, elem := range c.Elements {
		switch e := elem.(type) {
		case CardActions:
			row := make([]ButtonOption, 0, len(e.Buttons))
			for _, btn := range e.Buttons {
				row = append(row, ButtonOption{Text: btn.Text, Data: btn.Value})
			}
			if len(row) > 0 {
				rows = append(rows, row)
			}
		case CardListItem:
			rows = append(rows, []ButtonOption{{Text: e.BtnText, Data: e.BtnValue}})
		}
	}
	return rows
}
