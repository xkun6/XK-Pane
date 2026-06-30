package botbridge

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"xkpane/internal/events"
)

const keyDelay = 100 * time.Millisecond
const messageDedupTTL = 60 * time.Second

type PendingAsk struct {
	ToolUseID string
	Questions []events.AskQuestion
}

type sessionWriter interface {
	WriteInput(ctx context.Context, sessionID string, data string) error
}

type dispatchDeps struct {
	writer           sessionWriter
	ask              func(sessionID string) (PendingAsk, bool)
	listSessions     func() []sessionSnapshot
	saveBinding      func(scopeKey, sessionID string)
	sendText         func(ctx context.Context, scopeKey string, replyCtx any, content string) error
	sendCard         func(ctx context.Context, scopeKey string, replyCtx any, card *Card) error
	sessionWriteLock func(sessionID string) *sync.Mutex
	askAnswered      func(sessionID, toolUseID string)
}

type dispatcher struct {
	mu         sync.Mutex
	bindings   map[string]string
	pending    map[string]PendingAsk
	selections map[string]map[int][]int
	seen       map[string]map[string]bool
	messageIDs map[string]time.Time
	deps       dispatchDeps
}

func newDispatcher(deps dispatchDeps) *dispatcher {
	return &dispatcher{
		bindings:   map[string]string{},
		pending:    map[string]PendingAsk{},
		selections: map[string]map[int][]int{},
		seen:       map[string]map[string]bool{},
		messageIDs: map[string]time.Time{},
		deps:       deps,
	}
}

func (d *dispatcher) setBinding(scopeKey, sessionID string) {
	d.mu.Lock()
	d.bindings[scopeKey] = sessionID
	d.mu.Unlock()
}

func (d *dispatcher) replaceBindings(bindings map[string]string) {
	d.mu.Lock()
	d.bindings = map[string]string{}
	for k, v := range bindings {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			d.bindings[k] = v
		}
	}
	d.mu.Unlock()
}

func (d *dispatcher) bindingsForSession(sessionID string) []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := []string{}
	for scope, sid := range d.bindings {
		if sid == sessionID {
			out = append(out, scope)
		}
	}
	return out
}

func (d *dispatcher) boundSession(scopeKey string) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.bindings[scopeKey]
}

func (d *dispatcher) setPending(sessionID string, p PendingAsk) {
	d.mu.Lock()
	d.pending[sessionID] = p
	d.selections[p.ToolUseID] = map[int][]int{}
	d.mu.Unlock()
}

func (d *dispatcher) markToolUseSeen(sessionID, toolUseID string) {
	if toolUseID == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.seen[sessionID] == nil {
		d.seen[sessionID] = map[string]bool{}
	}
	d.seen[sessionID][toolUseID] = true
}

func (d *dispatcher) effectivePending(sessionID string) (PendingAsk, bool) {
	d.mu.Lock()
	p, ok := d.pending[sessionID]
	seen := d.seen[sessionID]
	if ok && p.ToolUseID != "" && seen[p.ToolUseID] {
		delete(d.pending, sessionID)
		delete(d.selections, p.ToolUseID)
		d.mu.Unlock()
		return PendingAsk{}, false
	}
	d.mu.Unlock()
	if ok {
		return p, true
	}
	if d.deps.ask == nil {
		return PendingAsk{}, false
	}
	p, ok = d.deps.ask(sessionID)
	if !ok || p.ToolUseID == "" {
		return PendingAsk{}, false
	}
	d.mu.Lock()
	if d.seen[sessionID] != nil && d.seen[sessionID][p.ToolUseID] {
		d.mu.Unlock()
		return PendingAsk{}, false
	}
	d.pending[sessionID] = p
	if d.selections[p.ToolUseID] == nil {
		d.selections[p.ToolUseID] = map[int][]int{}
	}
	d.mu.Unlock()
	return p, true
}

func (d *dispatcher) shouldDropMessage(messageID string) bool {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return false
	}
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	for id, expiresAt := range d.messageIDs {
		if !expiresAt.After(now) {
			delete(d.messageIDs, id)
		}
	}
	if expiresAt, ok := d.messageIDs[messageID]; ok && expiresAt.After(now) {
		return true
	}
	d.messageIDs[messageID] = now.Add(messageDedupTTL)
	return false
}

func (d *dispatcher) handleMessage(ctx context.Context, scopeKey, messageID, content string) (bool, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return false, nil
	}
	if d.shouldDropMessage(messageID) {
		return true, nil
	}
	if strings.HasPrefix(content, "/sessions") {
		return true, d.sendSessions(ctx, scopeKey, nil)
	}
	if strings.HasPrefix(content, "/switch ") {
		sid := strings.TrimSpace(strings.TrimPrefix(content, "/switch "))
		return true, d.switchSession(ctx, scopeKey, nil, sid)
	}
	sid := d.boundSession(scopeKey)
	if sid == "" {
		if d.deps.sendText != nil {
			_ = d.deps.sendText(ctx, scopeKey, nil, "未绑定会话。请先发送 /sessions 并选择一个会话。")
		}
		return true, nil
	}
	return true, d.writeText(ctx, sid, content+"\r")
}

func (d *dispatcher) handleAction(ctx context.Context, scopeKey, messageID, action string) (bool, error) {
	action = strings.TrimSpace(action)
	if action == "" {
		return false, nil
	}
	switch {
	case strings.HasPrefix(action, "askq:"):
		if d.shouldDropMessage(messageID) {
			return true, nil
		}
		return true, d.handleAskQ(ctx, scopeKey, action)
	case strings.HasPrefix(action, "asksubmit:"):
		if d.shouldDropMessage(messageID) {
			return true, nil
		}
		return true, d.submitAsk(ctx, scopeKey, strings.TrimPrefix(action, "asksubmit:"))
	case action == "cmd:/sessions":
		if d.shouldDropMessage(messageID) {
			return true, nil
		}
		return true, d.sendSessions(ctx, scopeKey, nil)
	case strings.HasPrefix(action, "cmd:/switch "):
		if d.shouldDropMessage(messageID) {
			return true, nil
		}
		return true, d.switchSession(ctx, scopeKey, nil, strings.TrimSpace(strings.TrimPrefix(action, "cmd:/switch ")))
	default:
		return false, nil
	}
}

func (d *dispatcher) handleAskQ(ctx context.Context, scopeKey, action string) error {
	sid := d.boundSession(scopeKey)
	if sid == "" {
		return nil
	}
	p, ok := d.effectivePending(sid)
	if !ok {
		return nil
	}
	qIdx, optIdx, ok := parseAskQ(action)
	if !ok || qIdx < 0 || qIdx >= len(p.Questions) {
		return nil
	}
	if optIdx < 0 || optIdx >= len(p.Questions[qIdx].Options) {
		return nil
	}
	shouldSubmit := false
	d.mu.Lock()
	if d.selections[p.ToolUseID] == nil {
		d.selections[p.ToolUseID] = map[int][]int{}
	}
	if p.Questions[qIdx].MultiSelect {
		cur := d.selections[p.ToolUseID][qIdx]
		for i, v := range cur {
			if v == optIdx {
				d.selections[p.ToolUseID][qIdx] = append(cur[:i], cur[i+1:]...)
				d.mu.Unlock()
				return nil
			}
		}
		d.selections[p.ToolUseID][qIdx] = append(cur, optIdx)
		d.mu.Unlock()
		return nil
	}
	d.selections[p.ToolUseID][qIdx] = []int{optIdx}
	if len(p.Questions) == 1 {
		shouldSubmit = true
	}
	d.mu.Unlock()
	if shouldSubmit {
		return d.submitAsk(ctx, scopeKey, p.ToolUseID)
	}
	return nil
}

func (d *dispatcher) submitAsk(ctx context.Context, scopeKey, toolUseID string) error {
	sid := d.boundSession(scopeKey)
	if sid == "" {
		return nil
	}
	p, ok := d.effectivePending(sid)
	if !ok || p.ToolUseID != toolUseID {
		return nil
	}
	d.mu.Lock()
	sel := copySelections(d.selections[toolUseID])
	d.mu.Unlock()
	questions := make([]AnswerQuestion, 0, len(p.Questions))
	for i, q := range p.Questions {
		if len(sel[i]) == 0 {
			sel[i] = []int{0}
		}
		questions = append(questions, AnswerQuestion{MultiSelect: q.MultiSelect, Options: make([]AnswerOption, len(q.Options))})
	}
	keys := BuildAnswerKeys(questions, sel)
	lock := d.sessionWriteLock(sid)
	lock.Lock()
	defer lock.Unlock()
	for _, key := range keys {
		if err := d.deps.writer.WriteInput(ctx, sid, key); err != nil {
			return err
		}
		time.Sleep(keyDelay)
	}
	d.mu.Lock()
	if d.seen[sid] == nil {
		d.seen[sid] = map[string]bool{}
	}
	d.seen[sid][toolUseID] = true
	delete(d.pending, sid)
	delete(d.selections, toolUseID)
	d.mu.Unlock()
	if d.deps.askAnswered != nil {
		d.deps.askAnswered(sid, toolUseID)
	}
	return nil
}

func (d *dispatcher) writeText(ctx context.Context, sessionID, data string) error {
	if d.deps.writer == nil {
		return fmt.Errorf("botbridge writer unavailable")
	}
	lock := d.sessionWriteLock(sessionID)
	lock.Lock()
	defer lock.Unlock()
	return d.deps.writer.WriteInput(ctx, sessionID, data)
}

func (d *dispatcher) sessionWriteLock(sessionID string) *sync.Mutex {
	if d.deps.sessionWriteLock != nil {
		return d.deps.sessionWriteLock(sessionID)
	}
	return &sync.Mutex{}
}

func (d *dispatcher) sendSessions(ctx context.Context, scopeKey string, replyCtx any) error {
	if d.deps.listSessions == nil || d.deps.sendCard == nil {
		return nil
	}
	sessions := d.deps.listSessions()
	card := NewCard().Title("XK-Pane sessions", "blue")
	if len(sessions) == 0 {
		card.Markdown("当前没有活跃会话。")
		return d.deps.sendCard(ctx, scopeKey, replyCtx, card.Build())
	}
	for _, s := range sessions {
		desc := strings.TrimSpace(s.Cwd)
		if desc == "" {
			desc = s.ID
		}
		card.ListItem(fmt.Sprintf("`%s` · %s · %s", s.ID, s.CliTool, desc), "切换", "cmd:/switch "+s.ID)
	}
	return d.deps.sendCard(ctx, scopeKey, replyCtx, card.Build())
}

func (d *dispatcher) switchSession(ctx context.Context, scopeKey string, replyCtx any, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	d.setBinding(scopeKey, sessionID)
	if d.deps.saveBinding != nil {
		d.deps.saveBinding(scopeKey, sessionID)
	}
	if d.deps.sendText != nil {
		return d.deps.sendText(ctx, scopeKey, replyCtx, "已切换到会话 "+sessionID)
	}
	return nil
}

func parseAskQ(action string) (int, int, bool) {
	rest := strings.TrimPrefix(action, "askq:")
	a, b, ok := strings.Cut(rest, ":")
	if !ok {
		return 0, 0, false
	}
	q, err1 := strconv.Atoi(a)
	opt, err2 := strconv.Atoi(b)
	return q, opt, err1 == nil && err2 == nil
}

func copySelections(in map[int][]int) map[int][]int {
	out := map[int][]int{}
	for k, v := range in {
		out[k] = append([]int(nil), v...)
	}
	return out
}
