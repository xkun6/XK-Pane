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

	fallbackLocksMu sync.Mutex
	fallbackLocks   map[string]*sync.Mutex
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
	// 文本作答兜底：纯文本平台（无按钮）下，待答期内的纯编号回复（如「1」「1,3」「1;2」）
	// 解析为选项选择 → 走 BuildAnswerKeys 提交，与按钮作答同一套键位。
	if p, ok := d.effectivePending(sid); ok {
		if sel, isAnswer := parseTextSelections(content, p.Questions); isAnswer {
			d.applyTextSelections(p.ToolUseID, sel)
			return true, d.submitAsk(ctx, scopeKey, p.ToolUseID)
		}
		// 待答期 PTY 处于选择器表单态：Ink 会忽略普通字符却消费末尾 \r（等价「按当前聚焦项作答」），
		// 透传普通文本会以第 0 项误提交表单。非编号回复一律拦下并提示，不写 PTY。
		if d.deps.sendText != nil {
			_ = d.deps.sendText(ctx, scopeKey, nil, "当前有待答问题，请回复选项编号作答（如「1」或「1,3」）；其它文本已忽略，作答完成后再发。")
		}
		return true, nil
	}
	return true, d.writeText(ctx, sid, content+"\r")
}

// applyTextSelections 把文本作答解析出的各题选择写入 selections，供 submitAsk 统一构造按键。
func (d *dispatcher) applyTextSelections(toolUseID string, sel map[int][]int) {
	d.mu.Lock()
	if d.selections[toolUseID] == nil {
		d.selections[toolUseID] = map[int][]int{}
	}
	for qi, picks := range sel {
		d.selections[toolUseID][qi] = append([]int(nil), picks...)
	}
	d.mu.Unlock()
}

// parseTextSelections 把「纯编号回复」解析为各题选项选择（0-based）。
// 约定：分号 ';' 分隔每题、逗号 ',' 分隔同题多选，编号 1-based；同时兼容中文「，」「；」。
// 仅当回复完全由数字/逗号/分号/空白组成、段数不超过题数、每个编号都落在该题选项范围内、
// 且单选题不含多个编号时，才认定为作答；否则返回 false，由调用方按普通文本透传到 PTY。
func parseTextSelections(content string, questions []events.AskQuestion) (map[int][]int, bool) {
	content = strings.TrimSpace(content)
	if content == "" || len(questions) == 0 {
		return nil, false
	}
	norm := strings.NewReplacer("，", ",", "；", ";").Replace(content)
	for _, r := range norm {
		switch {
		case r >= '0' && r <= '9', r == ',', r == ';', r == ' ', r == '\t':
		default:
			return nil, false
		}
	}
	segs := strings.Split(norm, ";")
	if len(segs) > len(questions) {
		return nil, false
	}
	out := map[int][]int{}
	for i, seg := range segs {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		opts := len(questions[i].Options)
		picks := make([]int, 0, len(seg))
		for _, tk := range strings.Split(seg, ",") {
			tk = strings.TrimSpace(tk)
			if tk == "" {
				continue
			}
			n, err := strconv.Atoi(tk)
			if err != nil || n < 1 || n > opts {
				return nil, false
			}
			picks = append(picks, n-1)
		}
		if len(picks) == 0 {
			continue
		}
		if !questions[i].MultiSelect && len(picks) > 1 {
			return nil, false
		}
		out[i] = picks
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
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
	toolUseID, qIdx, optIdx, ok := parseAskQ(action)
	if !ok {
		return nil
	}
	sid := d.boundSession(scopeKey)
	if sid == "" {
		return nil
	}
	p, ok := d.effectivePending(sid)
	if !ok {
		return nil
	}
	// IM 聊天记录里旧 ask 卡的按钮永远可点：toolUseID 与当前待答不符即静默忽略，
	// 防止历史卡片污染/误提交当前问答。
	if p.ToolUseID != toolUseID {
		return nil
	}
	if qIdx < 0 || qIdx >= len(p.Questions) {
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
	// 必须先拿会话写锁、锁内再复查待答：pending 的删除发生在首个提交者写完整套按键之后，
	// 若检查在锁外（check-then-act），双击提交/并发作答的第二个调用会等锁后把按键序列
	// 再写一遍到已离开表单的 PTY（末尾 Enter 可能盲触后续交互）。
	lock := d.sessionWriteLock(sid)
	lock.Lock()
	defer lock.Unlock()
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
	// fallback 必须按会话缓存：每次返回新 mutex 等于没有互斥。
	d.fallbackLocksMu.Lock()
	defer d.fallbackLocksMu.Unlock()
	if d.fallbackLocks == nil {
		d.fallbackLocks = map[string]*sync.Mutex{}
	}
	if d.fallbackLocks[sessionID] == nil {
		d.fallbackLocks[sessionID] = &sync.Mutex{}
	}
	return d.fallbackLocks[sessionID]
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

// parseAskQ 解析「askq:<toolUseID>:<qIdx>:<optIdx>」，从右往左拆两段数字、剩余为 toolUseID
// （兼容 toolUseID 自身含冒号）。旧版两段格式（无 toolUseID）解析失败返回 false，
// 历史卡片按钮点击即被忽略。
func parseAskQ(action string) (string, int, int, bool) {
	rest := strings.TrimPrefix(action, "askq:")
	i := strings.LastIndex(rest, ":")
	if i < 0 {
		return "", 0, 0, false
	}
	j := strings.LastIndex(rest[:i], ":")
	if j < 0 {
		return "", 0, 0, false
	}
	q, err1 := strconv.Atoi(rest[j+1 : i])
	opt, err2 := strconv.Atoi(rest[i+1:])
	if err1 != nil || err2 != nil {
		return "", 0, 0, false
	}
	return rest[:j], q, opt, true
}

func copySelections(in map[int][]int) map[int][]int {
	out := map[int][]int{}
	for k, v := range in {
		out[k] = append([]int(nil), v...)
	}
	return out
}
