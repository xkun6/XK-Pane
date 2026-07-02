package botbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"xkpane/internal/events"
	"xkpane/internal/services/sessionscan"
	"xkpane/internal/services/terminal"
	"xkpane/internal/services/transcript"
)

// optionsDescBlock 渲染选项的「编号 + 标签 + 描述」markdown 列表（飞书按钮只能显示标签，
// 描述靠这段文字补上）。任一选项无描述则整体省略，避免无意义的编号噪声。
func optionsDescBlock(opts []events.AskOption) string {
	has := false
	for _, o := range opts {
		if strings.TrimSpace(o.Description) != "" {
			has = true
			break
		}
	}
	if !has {
		return ""
	}
	var sb strings.Builder
	for i, o := range opts {
		label := o.Label
		if label == "" {
			label = fmt.Sprintf("Option %d", i+1)
		}
		sb.WriteString(fmt.Sprintf("%d. **%s**", i+1, label))
		if d := strings.TrimSpace(o.Description); d != "" {
			sb.WriteString(" — ")
			sb.WriteString(d)
		}
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

type sessionSnapshot struct {
	ID              string
	PID             int
	Cwd             string
	CliTool         string
	ClaudeSessionID string
	Status          string
	LastOutputAt    int64
}

func fromTerminalSnapshot(s terminal.SessionSnapshot) sessionSnapshot {
	return sessionSnapshot{
		ID:              s.ID,
		PID:             s.PID,
		Cwd:             s.Cwd,
		CliTool:         s.CliTool,
		ClaudeSessionID: s.ClaudeSessionID,
		Status:          s.Status,
		LastOutputAt:    s.LastOutputAt,
	}
}

func eventToCard(ev transcript.Event) (*Card, bool) {
	switch ev.Kind {
	case transcript.KindUser:
		return NewCard().Title("User", "grey").Markdown(ev.Text).Build(), true
	case transcript.KindAssistant:
		return NewCard().Title("Assistant", "blue").Markdown(ev.Text).Build(), true
	case transcript.KindThinking:
		return NewCard().Title("Thinking", "grey").Markdown(ev.Text).Build(), true
	case transcript.KindToolUse:
		if ev.ToolName == "AskUserQuestion" {
			return nil, false
		}
		text := ev.ToolName
		if ev.ToolInput != "" {
			text += "\n```json\n" + ev.ToolInput + "\n```"
		}
		return NewCard().Title("Tool", "orange").Markdown(text).Build(), true
	case transcript.KindToolResult:
		return toolResultCard(ev), true
	default:
		return nil, false
	}
}

func toolResultCard(ev transcript.Event) *Card {
	title := "Tool result"
	color := "green"
	if ev.IsError {
		color = "red"
	}
	b := NewCard().Title(title, color)
	if ev.Detail == nil {
		return b.Markdown(ev.Text).Build()
	}
	switch ev.Detail.Kind {
	case "edit":
		b.Markdownf("`%s` · +%d -%d", ev.Detail.FilePath, ev.Detail.Added, ev.Detail.Removed)
		if len(ev.Detail.Diff) > 0 {
			b.Markdown("```diff\n" + strings.Join(ev.Detail.Diff, "\n") + "\n```")
		}
	case "bash":
		if ev.Detail.Stdout != "" {
			b.Markdown("```text\n" + ev.Detail.Stdout + "\n```")
		}
		if ev.Detail.Stderr != "" {
			b.Markdown("```text\n" + ev.Detail.Stderr + "\n```")
		}
	default:
		b.Markdown(ev.Text)
	}
	return b.Build()
}

func askCard(p PendingAsk) *Card {
	b := NewCard().Title("AskUserQuestion", "orange")
	for qi, q := range p.Questions {
		if q.Header != "" {
			b.Markdown("**" + q.Header + "**")
		}
		b.Markdown(q.Question)
		if desc := optionsDescBlock(q.Options); desc != "" {
			b.Markdown(desc)
		}
		buttons := make([]CardButton, 0, len(q.Options))
		for oi, opt := range q.Options {
			label := opt.Label
			if label == "" {
				label = fmt.Sprintf("Option %d", oi+1)
			}
			buttons = append(buttons, CardButton{
				Text: label,
				Type: "default",
				// value 携带 toolUseID：IM 里历史卡片按钮永远可点，dispatcher 靠它拒绝
				// 与当前待答不符的点击（见 handleAskQ）。
				Value: fmt.Sprintf("askq:%s:%d:%d", p.ToolUseID, qi, oi),
				Extra: map[string]string{
					"askq_label":    label,
					"askq_question": q.Question,
				},
			})
		}
		if len(buttons) > 0 {
			b.Buttons(buttons...)
		}
	}
	if p.ToolUseID != "" {
		b.Buttons(PrimaryBtn("提交", "asksubmit:"+p.ToolUseID))
	}
	return b.Build()
}

// askCards 把待答渲染成「一题一卡」。单问题沿用 askCard（已验证可在飞书渲染）；
// 多问题时拆成 N 张卡（每张只含单个问题的选项行，末张附「提交全部」按钮）。
// 原因：飞书对单张卡内多个 action 交互模块存在渲染降级——3+ 个 action 模块会令整卡
// 退化成纯文字、所有按钮消失（实测复现）。拆成一题一卡后每张 ≤2 个 action 模块即可正常渲染。
// 作答按全局 qIdx 累积（见 dispatcher.handleAskQ），与卡片如何分布无关，故 answerKeys 不受影响。
func askCards(p PendingAsk) []*Card {
	total := len(p.Questions)
	if total <= 1 {
		return []*Card{askCard(p)}
	}
	cards := make([]*Card, 0, total)
	for qi, q := range p.Questions {
		b := NewCard().Title(fmt.Sprintf("AskUserQuestion (%d/%d)", qi+1, total), "orange")
		if q.Header != "" {
			b.Markdown("**" + q.Header + "**")
		}
		b.Markdown(q.Question)
		if q.MultiSelect {
			b.Markdown("_多选：可点多个选项，选完点末尾「提交全部」_")
		}
		if desc := optionsDescBlock(q.Options); desc != "" {
			b.Markdown(desc)
		}
		buttons := make([]CardButton, 0, len(q.Options))
		for oi, opt := range q.Options {
			label := opt.Label
			if label == "" {
				label = fmt.Sprintf("Option %d", oi+1)
			}
			buttons = append(buttons, CardButton{
				Text:  label,
				Type:  "default",
				Value: fmt.Sprintf("askq:%s:%d:%d", p.ToolUseID, qi, oi),
				Extra: map[string]string{
					"askq_label":    label,
					"askq_question": q.Question,
				},
			})
		}
		if len(buttons) > 0 {
			b.Buttons(buttons...)
		}
		if qi == total-1 && p.ToolUseID != "" {
			b.Buttons(PrimaryBtn("提交全部", "asksubmit:"+p.ToolUseID))
		}
		cards = append(cards, b.Build())
	}
	return cards
}

// askText 把待答渲染成纯文本作答提示，供无按钮的纯文本平台（企业微信 / QQ OneBot / 微信）使用：
// 每题选项 1-based 编号 + 描述，末尾说明回复格式。对应 dispatcher.parseTextSelections 的解析约定
// （分号分隔每题、逗号分隔同题多选）。按钮平台不走这里，仍用 askCards 渲染富卡。
func askText(p PendingAsk) string {
	var sb strings.Builder
	sb.WriteString("**需要你作答（AskUserQuestion）**\n")
	multi := false
	single := len(p.Questions) == 1
	for qi, q := range p.Questions {
		sb.WriteByte('\n')
		label := "问题"
		if !single {
			label = fmt.Sprintf("问题 %d", qi+1)
		}
		if q.MultiSelect {
			multi = true
			label += "（多选）"
		}
		sb.WriteString(label)
		sb.WriteString("：")
		if head := strings.TrimSpace(q.Header); head != "" {
			sb.WriteString(head)
			sb.WriteString(" — ")
		}
		sb.WriteString(strings.TrimSpace(q.Question))
		sb.WriteByte('\n')
		for oi, opt := range q.Options {
			label := opt.Label
			if label == "" {
				label = fmt.Sprintf("Option %d", oi+1)
			}
			sb.WriteString(fmt.Sprintf("  %d) %s", oi+1, label))
			if d := strings.TrimSpace(opt.Description); d != "" {
				sb.WriteString(" — ")
				sb.WriteString(d)
			}
			sb.WriteByte('\n')
		}
	}
	sb.WriteByte('\n')
	switch {
	case !single:
		sb.WriteString("回复编号作答：多题用分号「;」分隔每题")
		if multi {
			sb.WriteString("、多选用逗号「,」")
		}
		sb.WriteString("，例如「1;2」。未答的题取第 1 项。")
	case multi:
		sb.WriteString("回复编号作答：多选用逗号「,」分隔，例如「1,3」。")
	default:
		sb.WriteString("回复编号作答，例如「1」。")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func locateTranscript(ctx context.Context, scanner sessionScanner, snap sessionSnapshot) (string, bool) {
	if (snap.CliTool == "claude" || snap.CliTool == "reclaude") && snap.ClaudeSessionID != "" {
		return transcript.ResolveClaudePath(snap.Cwd, snap.ClaudeSessionID)
	}
	if scanner == nil {
		return "", false
	}
	sessions, err := scanner.ListProjectSessions(ctx, snap.Cwd, 0)
	if err != nil || len(sessions) == 0 {
		return "", false
	}
	want := "claude"
	if snap.CliTool == "codex" {
		want = "codex"
	}
	for _, sess := range sessions {
		if sess.Source == want && sess.FilePath != "" {
			return sess.FilePath, true
		}
	}
	return "", false
}

func parseTimeline(path string, snap sessionSnapshot, after int) transcript.Result {
	if snap.CliTool == "codex" || strings.Contains(filepath.ToSlash(path), "/.codex/") {
		return transcript.ParseCodex(path, after)
	}
	return transcript.Parse(path, after)
}

func compactJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

type sessionScanner interface {
	ListProjectSessions(ctx context.Context, projectPath string, limit int) ([]sessionscan.ResumeSession, error)
}
