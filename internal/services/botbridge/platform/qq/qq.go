// Package qq 是 OneBot v11（NapCat / LLOneBot / go-cqhttp 等）的 botbridge 文本适配器，
// 移植自 cc-connect platform/qq 的右半边（正向 WebSocket 连接 + 收发消息），裁掉 XK-Pane
// 用不到的部分：图片 / 语音 / 文件附件的下载与发送、HTTP API（仅文件用）、allow_from
// 白名单、群内共享会话开关。
//
// 因不实现 CardSender / InlineButtonSender，botbridge 视其为纯文本平台：AskUserQuestion
// 自动走「文字编号问答」兜底（见 dispatcher.parseTextSelections / timeline.askText）。
//
// 连接对象是用户自建的 OneBot 实现（NapCat 等）开放的正向 WebSocket 端点（ws_url），
// token 为可选 access_token（Authorization: Bearer）。
package qq

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"xkpane/internal/services/botbridge"
	"xkpane/internal/utils/logx"

	"github.com/gorilla/websocket"
)

func init() {
	botbridge.RegisterPlatform("qq", New)
}

const (
	apiTimeout           = 15 * time.Second
	maxReconnectAttempts = 30
	maxReconnectBackoff  = 60 * time.Second
)

type Platform struct {
	wsURL string
	token string

	handler     botbridge.MessageHandler
	connHandler botbridge.ConnectionStatusHandler

	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	wsConn *websocket.Conn

	echoSeq atomic.Int64
	echoCh  sync.Map // echo(string) -> chan json.RawMessage

	reconnecting atomic.Bool
	selfID       atomic.Int64
}

// replyContext 携带回信所需信息。messageType=group 用 groupID，private 用 userID。
type replyContext struct {
	messageType string // "group" | "private"
	groupID     int64
	userID      int64
	scopeKey    string
}

func New(opts map[string]any) (botbridge.Platform, error) {
	wsURL, _ := opts["ws_url"].(string)
	wsURL = strings.TrimSpace(wsURL)
	if wsURL == "" {
		return nil, fmt.Errorf("qq: ws_url is required")
	}
	token, _ := opts["token"].(string)
	return &Platform{
		wsURL: wsURL,
		token: strings.TrimSpace(token),
	}, nil
}

func (p *Platform) Name() string { return "qq" }

func (p *Platform) SetConnectionHandler(h botbridge.ConnectionStatusHandler) {
	p.mu.Lock()
	p.connHandler = h
	p.mu.Unlock()
}

var _ botbridge.ConnectionReporter = (*Platform)(nil)

func (p *Platform) Start(handler botbridge.MessageHandler) error {
	p.handler = handler
	ctx, cancel := context.WithCancel(context.Background())
	p.ctx = ctx
	p.cancel = cancel
	if err := p.dial(ctx); err != nil {
		cancel()
		return fmt.Errorf("qq: %w", err)
	}
	return nil
}

func (p *Platform) Stop() error {
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
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
		return fmt.Errorf("qq: invalid reply context %T", replyCtx)
	}
	params := map[string]any{"message": content}
	if rctx.messageType == "group" {
		params["group_id"] = rctx.groupID
		_, err := p.callAPI("send_group_msg", params)
		return err
	}
	params["user_id"] = rctx.userID
	_, err := p.callAPI("send_private_msg", params)
	return err
}

// ---------------------------------------------------------------------------
// WebSocket 连接
// ---------------------------------------------------------------------------

func (p *Platform) dial(ctx context.Context) error {
	conn, err := p.openConn()
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.wsConn = conn
	p.mu.Unlock()

	logx.For("botbridge.qq").Info("connected to OneBot", "url", p.wsURL)
	p.reportConnection(true, nil)

	// readLoop 必须先于 callAPI 启动：callAPI 的响应由 readLoop 按 echo 路由，
	// 否则 get_login_info 必定 15s 超时，selfID 留 0 会关掉自发消息过滤。
	go p.readLoop(ctx, conn)
	p.refreshSelfID()
	return nil
}

func (p *Platform) openConn() (*websocket.Conn, error) {
	header := http.Header{}
	if p.token != "" {
		header.Set("Authorization", "Bearer "+p.token)
	}
	conn, _, err := websocket.DefaultDialer.Dial(p.wsURL, header)
	if err != nil {
		return nil, fmt.Errorf("ws connect failed (%s): %w", p.wsURL, err)
	}
	return conn, nil
}

// refreshSelfID 拉取机器人自身 QQ 号，用于过滤自发消息（避免自问自答）。失败不致命。
func (p *Platform) refreshSelfID() {
	info, err := p.callAPI("get_login_info", nil)
	if err != nil {
		logx.For("botbridge.qq").Warn("get_login_info failed; self-message filter disabled", "err", err)
		return
	}
	if uid, ok := info["user_id"].(float64); ok {
		p.selfID.Store(int64(uid))
	}
	nick, _ := info["nickname"].(string)
	logx.For("botbridge.qq").Info("logged in", "qq", p.selfID.Load(), "nickname", nick)
}

func (p *Platform) readLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logx.For("botbridge.qq").Warn("ws read error, reconnecting", "err", err)
			p.reportConnection(false, err)
			p.triggerReconnect(conn)
			return
		}
		var payload map[string]any
		if json.Unmarshal(raw, &payload) != nil {
			continue
		}
		// API 响应（带 echo）→ 路由给对应的 callAPI 等待者。
		if echo, ok := payload["echo"].(string); ok {
			if ch, loaded := p.echoCh.LoadAndDelete(echo); loaded {
				if dataCh, ok := ch.(chan json.RawMessage); ok {
					dataCh <- raw
				}
			}
			continue
		}
		// 否则是事件，只处理消息事件。
		if pt, _ := payload["post_type"].(string); pt == "message" {
			p.handleMessage(payload)
		}
	}
}

func (p *Platform) triggerReconnect(stale *websocket.Conn) {
	if !p.reconnecting.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer p.reconnecting.Store(false)
		p.reconnectLoop(p.ctx, stale)
	}()
}

func (p *Platform) reconnectLoop(ctx context.Context, stale *websocket.Conn) {
	p.mu.Lock()
	if p.wsConn == stale {
		p.wsConn = nil
	}
	p.mu.Unlock()
	if stale != nil {
		stale.Close()
	}

	backoff := time.Second
	for attempt := 1; attempt <= maxReconnectAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		conn, err := p.openConn()
		if err != nil {
			logx.For("botbridge.qq").Warn("reconnect attempt failed", "attempt", attempt, "err", err)
			backoff = min(backoff*2, maxReconnectBackoff)
			continue
		}
		p.mu.Lock()
		p.wsConn = conn
		p.mu.Unlock()
		logx.For("botbridge.qq").Info("reconnected")
		p.reportConnection(true, nil)
		go p.readLoop(ctx, conn)
		p.refreshSelfID()
		return
	}
	logx.For("botbridge.qq").Error("reconnect failed after max attempts")
	p.reportConnection(false, fmt.Errorf("qq: reconnect failed"))
}

// ---------------------------------------------------------------------------
// 收消息
// ---------------------------------------------------------------------------

func (p *Platform) handleMessage(payload map[string]any) {
	msgType, _ := payload["message_type"].(string)
	userID := jsonInt64(payload, "user_id")
	groupID := jsonInt64(payload, "group_id")
	messageID := jsonInt64(payload, "message_id")

	if userID == 0 {
		return
	}
	if self := p.selfID.Load(); self != 0 && userID == self {
		return
	}

	text := parseText(payload)
	if text == "" {
		return
	}

	var scope string
	rctx := &replyContext{userID: userID}
	if msgType == "group" {
		scope = fmt.Sprintf("qq:%d:%d", groupID, userID)
		rctx.messageType = "group"
		rctx.groupID = groupID
	} else {
		scope = fmt.Sprintf("qq:%d:%d", userID, userID)
		rctx.messageType = "private"
	}
	rctx.scopeKey = scope

	p.emit(&botbridge.Message{
		MessageID: strconv.FormatInt(messageID, 10),
		ScopeKey:  scope,
		Platform:  "qq",
		UserID:    strconv.FormatInt(userID, 10),
		Content:   text,
		ReplyCtx:  rctx,
	})
}

// parseText 从 OneBot 消息里抽纯文本：段数组取 text 段，字符串则剥 CQ 码。
func parseText(payload map[string]any) string {
	switch msg := payload["message"].(type) {
	case []any:
		var parts []string
		for _, seg := range msg {
			s, ok := seg.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := s["type"].(string); t != "text" {
				continue
			}
			data, _ := s["data"].(map[string]any)
			if data == nil {
				continue
			}
			if txt, ok := data["text"].(string); ok {
				parts = append(parts, txt)
			}
		}
		return strings.TrimSpace(strings.Join(parts, ""))
	case string:
		return strings.TrimSpace(stripCQCodes(msg))
	default:
		if raw, ok := payload["raw_message"].(string); ok {
			return strings.TrimSpace(stripCQCodes(raw))
		}
		return ""
	}
}

// ---------------------------------------------------------------------------
// OneBot API（经 WebSocket，echo 路由）
// ---------------------------------------------------------------------------

func (p *Platform) callAPI(action string, params map[string]any) (map[string]any, error) {
	seq := p.echoSeq.Add(1)
	echo := strconv.FormatInt(seq, 10)
	req := map[string]any{"action": action, "echo": echo}
	if params != nil {
		req["params"] = params
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	ch := make(chan json.RawMessage, 1)
	p.echoCh.Store(echo, ch)
	defer p.echoCh.Delete(echo)

	p.mu.Lock()
	conn := p.wsConn
	if conn == nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("qq: not connected")
	}
	err = conn.WriteMessage(websocket.TextMessage, data)
	p.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("qq: ws write: %w", err)
	}

	select {
	case raw := <-ch:
		var resp struct {
			RetCode int             `json:"retcode"`
			Data    json.RawMessage `json:"data"`
		}
		if json.Unmarshal(raw, &resp) != nil {
			return nil, fmt.Errorf("qq: invalid API response")
		}
		if resp.RetCode != 0 {
			return nil, fmt.Errorf("qq: API %s failed (retcode=%d)", action, resp.RetCode)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Data, &result)
		return result, nil
	case <-time.After(apiTimeout):
		return nil, fmt.Errorf("qq: API %s timeout", action)
	}
}

// ---------------------------------------------------------------------------
// 上下文 / 辅助
// ---------------------------------------------------------------------------

func (p *Platform) parseReplyContext(v any) (*replyContext, bool) {
	switch x := v.(type) {
	case *replyContext:
		return x, true
	case replyContext:
		return &x, true
	case string:
		return reconstructReplyCtx(x)
	default:
		return nil, false
	}
}

// reconstructReplyCtx 从 scopeKey "qq:<chatID>:<userID>" 还原回信上下文。
// chatID==userID 视为单聊（private），否则为群（group）。
func reconstructReplyCtx(scope string) (*replyContext, bool) {
	parts := strings.Split(scope, ":")
	if len(parts) != 3 || parts[0] != "qq" || parts[1] == "" {
		return nil, false
	}
	chatID, _ := strconv.ParseInt(parts[1], 10, 64)
	userID, _ := strconv.ParseInt(parts[2], 10, 64)
	if parts[1] == parts[2] {
		return &replyContext{messageType: "private", userID: userID, scopeKey: scope}, true
	}
	return &replyContext{messageType: "group", groupID: chatID, userID: userID, scopeKey: scope}, true
}

func (p *Platform) reportConnection(connected bool, err error) {
	p.mu.Lock()
	h := p.connHandler
	p.mu.Unlock()
	if h != nil {
		h(connected, err)
	}
}

func (p *Platform) emit(msg *botbridge.Message) {
	if p.handler != nil {
		go p.handler(p, msg)
	}
}

func jsonInt64(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	}
	return 0
}

// stripCQCodes 去掉 raw_message 里的 [CQ:...] 段，只留纯文本。
func stripCQCodes(s string) string {
	var b strings.Builder
	for len(s) > 0 {
		idx := strings.Index(s, "[CQ:")
		if idx < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:idx])
		end := strings.Index(s[idx:], "]")
		if end < 0 {
			break
		}
		s = s[idx+end+1:]
	}
	return b.String()
}
