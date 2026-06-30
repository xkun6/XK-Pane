// Package qqbot 是 QQ 官方机器人（QQ Bot API v2）的 botbridge 文本适配器，移植自 cc-connect
// platform/qqbot 的右半边（网关协议 + 收发消息），裁掉 XK-Pane 用不到的部分：富媒体上传 /
// markdown+keyboard 富按钮（需 QQ 侧模板审核）/ 消息编辑缓存 / sandbox / allow_from。
//
// 因不实现 CardSender / InlineButtonSender，botbridge 视其为纯文本平台：AskUserQuestion 自动
// 走「文字编号问答」兜底（见 dispatcher.parseTextSelections / timeline.askText）。
//
// QQ 平台限制：主动推送（非被动回复）配额很紧，多数消息必须在用户来信后约 5 分钟内以
// passive reply（带 msg_id）发出。为配合 botbridge 轮询器的主动推卡模型，这里按 scopeKey
// 缓存最近一次来信的 msg_id，主动发送时若仍在窗口内则复用为 passive reply 提升送达率。
package qqbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"xkpane/internal/services/botbridge"
	"xkpane/internal/utils/httpx"
	"xkpane/internal/utils/logx"

	"github.com/gorilla/websocket"
)

func init() {
	botbridge.RegisterPlatform("qqbot", New)
}

const (
	// 默认 intents：GROUP_AND_C2C_EVENT (1<<25) | INTERACTION (1<<26)。
	// 文本模式不发按钮、INTERACTION 不会触发，但保留该位以便日后扩展富按钮。
	defaultIntents = (1 << 25) | (1 << 26)

	maxReconnectBackoff  = 60 * time.Second
	maxReconnectAttempts = 30
	tokenRefreshMargin   = 5 * time.Minute
	messageMaxRunes      = 2000
	msgSeqTTL            = 5 * time.Minute
	passiveReplyWindow   = 5 * time.Minute
)

const (
	apiBase  = "https://api.sgroup.qq.com"
	tokenURL = "https://bots.qq.com/app/getAppAccessToken"
)

// QQ Bot 网关 opcode。
const (
	opDispatch       = 0
	opHeartbeat      = 1
	opIdentify       = 2
	opResume         = 6
	opReconnect      = 7
	opInvalidSession = 9
	opHello          = 10
	opHeartbeatACK   = 11
)

type Platform struct {
	appID     string
	appSecret string
	intents   int

	handler botbridge.MessageHandler
	conn    botbridge.ConnectionStatusHandler

	ctx    context.Context
	cancel context.CancelFunc

	tokenMu     sync.RWMutex
	token       string
	tokenExpiry time.Time

	wsMu         sync.Mutex
	wsConn       *websocket.Conn
	sessionID    string
	lastSeq      atomic.Int64
	heartbeatMs  int
	heartbeatOK  atomic.Bool
	reconnecting atomic.Bool
	connCancel   context.CancelFunc

	msgSeqMu  sync.Mutex
	msgSeqMap map[string]*msgSeqEntry

	recentMu sync.Mutex
	recent   map[string]recentMsg
}

type msgSeqEntry struct {
	seq       atomic.Int32
	createdAt time.Time
}

type recentMsg struct {
	id string
	at time.Time
}

// replyContext 携带回信所需信息。messageType=group 用群 openid，c2c 用用户 openid。
type replyContext struct {
	messageType string
	groupOpenID string
	userOpenID  string
	eventMsgID  string
	scopeKey    string
}

func New(opts map[string]any) (botbridge.Platform, error) {
	appID, _ := opts["app_id"].(string)
	appSecret, _ := opts["app_secret"].(string)
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(appSecret) == "" {
		return nil, fmt.Errorf("qqbot: app_id and app_secret are required")
	}
	return &Platform{
		appID:     appID,
		appSecret: appSecret,
		intents:   defaultIntents,
		msgSeqMap: map[string]*msgSeqEntry{},
		recent:    map[string]recentMsg{},
	}, nil
}

func (p *Platform) Name() string { return "qqbot" }

func (p *Platform) SetConnectionHandler(h botbridge.ConnectionStatusHandler) {
	p.wsMu.Lock()
	p.conn = h
	p.wsMu.Unlock()
}

func (p *Platform) Start(handler botbridge.MessageHandler) error {
	p.handler = handler
	ctx, cancel := context.WithCancel(context.Background())
	p.ctx = ctx
	p.cancel = cancel
	if err := p.refreshToken(ctx); err != nil {
		cancel()
		return fmt.Errorf("qqbot: get access token: %w", err)
	}
	if err := p.connectGateway(ctx); err != nil {
		cancel()
		return fmt.Errorf("qqbot: connect gateway: %w", err)
	}
	return nil
}

func (p *Platform) Stop() error {
	if p.cancel != nil {
		p.cancel()
	}
	p.wsMu.Lock()
	defer p.wsMu.Unlock()
	if p.wsConn != nil {
		err := p.wsConn.Close()
		p.wsConn = nil
		return err
	}
	return nil
}

func (p *Platform) Reply(ctx context.Context, replyCtx any, content string) error {
	return p.Send(ctx, replyCtx, content)
}

func (p *Platform) Send(ctx context.Context, replyCtx any, content string) error {
	rctx, ok := p.parseReplyContext(replyCtx)
	if !ok {
		return fmt.Errorf("qqbot: invalid reply context %T", replyCtx)
	}
	for _, chunk := range chunkRunes(content, messageMaxRunes) {
		if err := p.sendMessage(ctx, rctx, chunk); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// OAuth2 Token
// ---------------------------------------------------------------------------

func (p *Platform) refreshToken(ctx context.Context) error {
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   string `json:"expires_in"`
	}
	in := map[string]string{"appId": p.appID, "clientSecret": p.appSecret}
	if err := httpx.PostJSON(ctx, tokenURL, in, &result); err != nil {
		return err
	}
	if result.AccessToken == "" {
		return fmt.Errorf("qqbot: empty access_token")
	}
	expiresSec := 0
	_, _ = fmt.Sscanf(result.ExpiresIn, "%d", &expiresSec)
	if expiresSec <= 0 {
		expiresSec = 7200
	}
	p.tokenMu.Lock()
	p.token = result.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(expiresSec) * time.Second)
	p.tokenMu.Unlock()
	return nil
}

func (p *Platform) getAccessToken(ctx context.Context) (string, error) {
	p.tokenMu.RLock()
	token := p.token
	expiry := p.tokenExpiry
	p.tokenMu.RUnlock()
	if token != "" && time.Now().Before(expiry.Add(-tokenRefreshMargin)) {
		return token, nil
	}
	if err := p.refreshToken(ctx); err != nil {
		return "", err
	}
	p.tokenMu.RLock()
	defer p.tokenMu.RUnlock()
	return p.token, nil
}

// ---------------------------------------------------------------------------
// WebSocket 网关
// ---------------------------------------------------------------------------

func (p *Platform) connectGateway(ctx context.Context) error {
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return err
	}
	gatewayURL, err := p.getGatewayURL(ctx, token)
	if err != nil {
		return err
	}
	conn, _, err := websocket.DefaultDialer.Dial(gatewayURL, nil)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	p.wsMu.Lock()
	p.wsConn = conn
	p.wsMu.Unlock()
	if err := p.waitForHello(conn); err != nil {
		conn.Close()
		return err
	}
	if err := p.sendIdentify(conn, token); err != nil {
		conn.Close()
		return err
	}
	if err := p.waitForReady(conn); err != nil {
		conn.Close()
		return err
	}
	p.reportConnection(true, nil)
	connCtx, connCancel := context.WithCancel(ctx)
	p.connCancel = connCancel
	go p.heartbeatLoop(connCtx)
	go p.readLoop(connCtx)
	return nil
}

func (p *Platform) getGatewayURL(ctx context.Context, token string) (string, error) {
	var result struct {
		URL string `json:"url"`
	}
	if err := p.apiDo(ctx, http.MethodGet, apiBase+"/gateway/bot", token, nil, &result); err != nil {
		return "", err
	}
	if result.URL == "" {
		return "", fmt.Errorf("qqbot: empty gateway url")
	}
	return result.URL, nil
}

type wsPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  *int64          `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

func (p *Platform) waitForHello(conn *websocket.Conn) error {
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()
	var msg wsPayload
	if err := conn.ReadJSON(&msg); err != nil {
		return fmt.Errorf("waiting for hello: %w", err)
	}
	if msg.Op != opHello {
		return fmt.Errorf("expected op 10 (Hello), got %d", msg.Op)
	}
	var hello struct {
		HeartbeatInterval int `json:"heartbeat_interval"`
	}
	_ = json.Unmarshal(msg.D, &hello)
	if hello.HeartbeatInterval > 0 {
		p.heartbeatMs = hello.HeartbeatInterval
	} else {
		p.heartbeatMs = 41250
	}
	p.heartbeatOK.Store(true)
	return nil
}

func (p *Platform) sendIdentify(conn *websocket.Conn, token string) error {
	identify := map[string]any{
		"op": opIdentify,
		"d": map[string]any{
			"token":   "QQBot " + token,
			"intents": p.intents,
			"shard":   [2]int{0, 1},
		},
	}
	p.wsMu.Lock()
	err := conn.WriteJSON(identify)
	p.wsMu.Unlock()
	return err
}

func (p *Platform) waitForReady(conn *websocket.Conn) error {
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()
	var msg wsPayload
	if err := conn.ReadJSON(&msg); err != nil {
		return fmt.Errorf("waiting for ready: %w", err)
	}
	if msg.Op != opDispatch || msg.T != "READY" {
		return fmt.Errorf("expected READY, got op=%d t=%s", msg.Op, msg.T)
	}
	var ready struct {
		SessionID string `json:"session_id"`
	}
	_ = json.Unmarshal(msg.D, &ready)
	p.sessionID = ready.SessionID
	if msg.S != nil {
		p.lastSeq.Store(*msg.S)
	}
	return nil
}

func (p *Platform) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(p.heartbeatMs) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !p.heartbeatOK.Load() {
				logx.For("botbridge.qqbot").Warn("no heartbeat ACK, reconnecting")
				p.triggerReconnect()
				return
			}
			p.heartbeatOK.Store(false)
			p.sendHeartbeat()
		}
	}
}

func (p *Platform) sendHeartbeat() {
	seq := p.lastSeq.Load()
	var d json.RawMessage
	if seq > 0 {
		d, _ = json.Marshal(seq)
	} else {
		d = json.RawMessage("null")
	}
	p.wsMu.Lock()
	defer p.wsMu.Unlock()
	if p.wsConn != nil {
		_ = p.wsConn.WriteJSON(wsPayload{Op: opHeartbeat, D: d})
	}
}

func (p *Platform) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		p.wsMu.Lock()
		conn := p.wsConn
		p.wsMu.Unlock()
		if conn == nil {
			return
		}
		var msg wsPayload
		if err := conn.ReadJSON(&msg); err != nil {
			if ctx.Err() != nil {
				return
			}
			logx.For("botbridge.qqbot").Warn("ws read error, reconnecting", "err", err)
			p.reportConnection(false, err)
			p.triggerReconnect()
			return
		}
		if msg.S != nil {
			p.lastSeq.Store(*msg.S)
		}
		switch msg.Op {
		case opDispatch:
			p.handleDispatch(msg.T, msg.D)
		case opHeartbeat:
			p.sendHeartbeat()
		case opReconnect:
			p.triggerReconnect()
			return
		case opInvalidSession:
			var resumable bool
			_ = json.Unmarshal(msg.D, &resumable)
			if !resumable {
				p.sessionID = ""
			}
			p.triggerReconnect()
			return
		case opHeartbeatACK:
			p.heartbeatOK.Store(true)
		}
	}
}

func (p *Platform) triggerReconnect() {
	if p.reconnecting.CompareAndSwap(false, true) {
		go func() {
			defer p.reconnecting.Store(false)
			p.reconnectLoop(p.ctx)
		}()
	}
}

func (p *Platform) reconnectLoop(ctx context.Context) {
	if p.connCancel != nil {
		p.connCancel()
	}
	p.wsMu.Lock()
	if p.wsConn != nil {
		p.wsConn.Close()
		p.wsConn = nil
	}
	p.wsMu.Unlock()

	backoff := time.Second
	for attempt := 1; attempt <= maxReconnectAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if err := p.refreshToken(ctx); err != nil {
			backoff = min(backoff*2, maxReconnectBackoff)
			continue
		}
		token, _ := p.getAccessToken(ctx)
		gatewayURL, err := p.getGatewayURL(ctx, token)
		if err != nil {
			backoff = min(backoff*2, maxReconnectBackoff)
			continue
		}
		conn, _, err := websocket.DefaultDialer.Dial(gatewayURL, nil)
		if err != nil {
			backoff = min(backoff*2, maxReconnectBackoff)
			continue
		}
		p.wsMu.Lock()
		p.wsConn = conn
		p.wsMu.Unlock()
		if err := p.waitForHello(conn); err != nil {
			p.clearConn(conn)
			backoff = min(backoff*2, maxReconnectBackoff)
			continue
		}
		// 有 session_id 先尝试 resume，失败再 identify。
		ok := false
		if p.sessionID != "" {
			if err := p.sendResume(conn, token); err == nil {
				ok = true
			} else {
				p.sessionID = ""
			}
		}
		if !ok {
			if err := p.sendIdentify(conn, token); err != nil {
				p.clearConn(conn)
				backoff = min(backoff*2, maxReconnectBackoff)
				continue
			}
			if err := p.waitForReady(conn); err != nil {
				p.clearConn(conn)
				backoff = min(backoff*2, maxReconnectBackoff)
				continue
			}
		}
		logx.For("botbridge.qqbot").Info("reconnected")
		p.reportConnection(true, nil)
		connCtx, connCancel := context.WithCancel(ctx)
		p.connCancel = connCancel
		go p.heartbeatLoop(connCtx)
		go p.readLoop(connCtx)
		return
	}
	logx.For("botbridge.qqbot").Error("reconnect failed after max attempts")
	p.reportConnection(false, fmt.Errorf("qqbot: reconnect failed"))
}

func (p *Platform) clearConn(conn *websocket.Conn) {
	p.wsMu.Lock()
	p.wsConn = nil
	p.wsMu.Unlock()
	conn.Close()
}

func (p *Platform) sendResume(conn *websocket.Conn, token string) error {
	resume := map[string]any{
		"op": opResume,
		"d": map[string]any{
			"token":      "QQBot " + token,
			"session_id": p.sessionID,
			"seq":        p.lastSeq.Load(),
		},
	}
	p.wsMu.Lock()
	err := conn.WriteJSON(resume)
	p.wsMu.Unlock()
	return err
}

// ---------------------------------------------------------------------------
// 事件分发
// ---------------------------------------------------------------------------

func (p *Platform) handleDispatch(eventType string, data json.RawMessage) {
	switch eventType {
	case "GROUP_AT_MESSAGE_CREATE":
		p.handleGroupMessage(data)
	case "C2C_MESSAGE_CREATE":
		p.handleC2CMessage(data)
	}
}

func (p *Platform) handleGroupMessage(data json.RawMessage) {
	var d struct {
		ID          string `json:"id"`
		GroupOpenID string `json:"group_openid"`
		Content     string `json:"content"`
		Author      struct {
			MemberOpenID string `json:"member_openid"`
		} `json:"author"`
	}
	if err := json.Unmarshal(data, &d); err != nil || d.ID == "" || d.GroupOpenID == "" {
		return
	}
	content := stripAtMention(d.Content)
	if content == "" {
		return
	}
	scope := "qqbot:" + d.GroupOpenID + ":" + d.Author.MemberOpenID
	p.rememberMsg(scope, d.ID)
	p.emit(&botbridge.Message{
		MessageID: d.ID,
		ScopeKey:  scope,
		Platform:  "qqbot",
		UserID:    d.Author.MemberOpenID,
		Content:   content,
		ReplyCtx: &replyContext{
			messageType: "group",
			groupOpenID: d.GroupOpenID,
			userOpenID:  d.Author.MemberOpenID,
			eventMsgID:  d.ID,
			scopeKey:    scope,
		},
	})
}

func (p *Platform) handleC2CMessage(data json.RawMessage) {
	var d struct {
		ID      string `json:"id"`
		Content string `json:"content"`
		Author  struct {
			UserOpenID string `json:"user_openid"`
		} `json:"author"`
	}
	if err := json.Unmarshal(data, &d); err != nil || d.ID == "" || d.Author.UserOpenID == "" {
		return
	}
	content := strings.TrimSpace(d.Content)
	if content == "" {
		return
	}
	scope := "qqbot:" + d.Author.UserOpenID + ":" + d.Author.UserOpenID
	p.rememberMsg(scope, d.ID)
	p.emit(&botbridge.Message{
		MessageID: d.ID,
		ScopeKey:  scope,
		Platform:  "qqbot",
		UserID:    d.Author.UserOpenID,
		Content:   content,
		ReplyCtx: &replyContext{
			messageType: "c2c",
			userOpenID:  d.Author.UserOpenID,
			eventMsgID:  d.ID,
			scopeKey:    scope,
		},
	})
}

// ---------------------------------------------------------------------------
// 发送消息
// ---------------------------------------------------------------------------

func (p *Platform) sendMessage(ctx context.Context, rctx *replyContext, content string) error {
	var url string
	switch rctx.messageType {
	case "group":
		url = fmt.Sprintf("%s/v2/groups/%s/messages", apiBase, rctx.groupOpenID)
	case "c2c":
		url = fmt.Sprintf("%s/v2/users/%s/messages", apiBase, rctx.userOpenID)
	default:
		return fmt.Errorf("qqbot: unknown message type %q", rctx.messageType)
	}
	body := map[string]any{"content": content, "msg_type": 0}
	if rctx.eventMsgID != "" {
		body["msg_id"] = rctx.eventMsgID
		body["msg_seq"] = p.nextMsgSeq(rctx.eventMsgID)
	}
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return err
	}
	return p.apiDo(ctx, http.MethodPost, url, token, body, nil)
}

func (p *Platform) nextMsgSeq(eventMsgID string) int32 {
	p.msgSeqMu.Lock()
	defer p.msgSeqMu.Unlock()
	now := time.Now()
	for k, v := range p.msgSeqMap {
		if now.Sub(v.createdAt) > msgSeqTTL {
			delete(p.msgSeqMap, k)
		}
	}
	entry, ok := p.msgSeqMap[eventMsgID]
	if !ok {
		entry = &msgSeqEntry{createdAt: now}
		p.msgSeqMap[eventMsgID] = entry
	}
	return entry.seq.Add(1)
}

func (p *Platform) rememberMsg(scope, msgID string) {
	p.recentMu.Lock()
	p.recent[scope] = recentMsg{id: msgID, at: time.Now()}
	p.recentMu.Unlock()
}

func (p *Platform) recentMsgID(scope string) string {
	p.recentMu.Lock()
	defer p.recentMu.Unlock()
	r, ok := p.recent[scope]
	if !ok || time.Since(r.at) > passiveReplyWindow {
		return ""
	}
	return r.id
}

// ---------------------------------------------------------------------------
// HTTP / 上下文
// ---------------------------------------------------------------------------

// apiDo 发起带 QQBot 授权头的请求；401 自动刷新 token 重试一次。out 为 nil 表示丢弃响应体。
func (p *Platform) apiDo(ctx context.Context, method, url, token string, body any, out any) error {
	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("qqbot: marshal body: %w", err)
		}
		raw = b
	}
	resp, err := p.doOnce(ctx, method, url, token, raw)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if err := p.refreshToken(ctx); err != nil {
			return err
		}
		token, _ = p.getAccessToken(ctx)
		resp, err = p.doOnce(ctx, method, url, token, raw)
		if err != nil {
			return err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		snip, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("qqbot: %s %s -> %d: %s", method, url, resp.StatusCode, snip)
	}
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (p *Platform) doOnce(ctx context.Context, method, url, token string, raw []byte) (*http.Response, error) {
	var reader io.Reader
	if raw != nil {
		reader = bytes.NewReader(raw)
	}
	req, cancel, err := httpx.NewRequest(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	defer cancel()
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json")
	return httpx.Do(req)
}

func (p *Platform) parseReplyContext(v any) (*replyContext, bool) {
	switch x := v.(type) {
	case *replyContext:
		return x, true
	case replyContext:
		return &x, true
	case string:
		rctx, ok := reconstructReplyCtx(x)
		if !ok {
			return nil, false
		}
		// 主动推送时无 eventMsgID：若该 scope 最近来信仍在 passive reply 窗口内，复用其 msg_id。
		if id := p.recentMsgID(x); id != "" {
			rctx.eventMsgID = id
		}
		return rctx, true
	default:
		return nil, false
	}
}

// reconstructReplyCtx 从 scopeKey "qqbot:<chatID>:<userID>" 还原回信上下文。
// chatID==userID 视为单聊（c2c），否则为群（group）。
func reconstructReplyCtx(scope string) (*replyContext, bool) {
	parts := strings.Split(scope, ":")
	if len(parts) != 3 || parts[0] != "qqbot" || parts[1] == "" {
		return nil, false
	}
	chatID, userID := parts[1], parts[2]
	if chatID == userID {
		return &replyContext{messageType: "c2c", userOpenID: userID, scopeKey: scope}, true
	}
	return &replyContext{messageType: "group", groupOpenID: chatID, userOpenID: userID, scopeKey: scope}, true
}

func (p *Platform) reportConnection(connected bool, err error) {
	p.wsMu.Lock()
	h := p.conn
	p.wsMu.Unlock()
	if h != nil {
		h(connected, err)
	}
}

func (p *Platform) emit(msg *botbridge.Message) {
	if p.handler != nil {
		go p.handler(p, msg)
	}
}

func stripAtMention(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "<@!") {
		if idx := strings.Index(content, ">"); idx >= 0 {
			content = strings.TrimSpace(content[idx+1:])
		}
	}
	return content
}

func chunkRunes(s string, max int) []string {
	r := []rune(s)
	if len(r) <= max {
		return []string{s}
	}
	var out []string
	for len(r) > max {
		out = append(out, string(r[:max]))
		r = r[max:]
	}
	if len(r) > 0 {
		out = append(out, string(r))
	}
	return out
}
