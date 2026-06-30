// Package wecom 是企业微信「智能机器人长连接」（aibot WebSocket）的 botbridge 文本适配器，
// 移植自 cc-connect platform/wecom 的 websocket.go 右半边，裁掉 XK-Pane 用不到的部分：
// 自建应用 HTTP 回调模式（需公网回调 URL + AES 加密）、图片 / 语音 / 文件附件的下载与上传、
// allow_from 白名单、用户名解析。
//
// 选用长连接模式（而非自建应用回调）是因为它对桌面端友好：主动外连 wss，无需公网 URL、
// 无消息加密、无 IP 白名单；凭据只需 bot_id + bot_secret。
//
// 因不实现 CardSender / InlineButtonSender，botbridge 视其为纯文本平台：AskUserQuestion
// 自动走「文字编号问答」兜底（见 dispatcher.parseTextSelections / timeline.askText）。
package wecom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"xkpane/internal/services/botbridge"
	"xkpane/internal/utils/logx"

	"github.com/gorilla/websocket"
)

func init() {
	botbridge.RegisterPlatform("wecom", New)
}

const (
	wsEndpoint      = "wss://openws.work.weixin.qq.com"
	wsPingInterval  = 30 * time.Second
	wsMaxBackoff    = 30 * time.Second
	wsMaxMissedPong = 2
	wsAckTimeout    = 5 * time.Second
	messageMaxBytes = 2000
	dedupTTL        = 60 * time.Second
)

var errWSAckTimeout = errors.New("wecom: ack timeout")

type Platform struct {
	botID  string
	secret string

	handler     botbridge.MessageHandler
	connHandler botbridge.ConnectionStatusHandler

	ctx    context.Context
	cancel context.CancelFunc

	mu   sync.Mutex // 保护 conn 写 + connHandler
	conn *websocket.Conn

	reqSeq      atomic.Int64
	missedPong  atomic.Int32
	pendingAcks sync.Map // req_id -> chan wsAckResult

	dedupMu sync.Mutex
	dedup   map[string]time.Time
}

// replyContext 携带回信所需信息。reqID 来自 aibot_msg_callback，用于 aibot_respond_msg 流式回复；
// chatID 用于 aibot_send_msg 主动发送。
type replyContext struct {
	reqID    string
	chatID   string
	chatType string // "single" | "group"
	userID   string
	scopeKey string
}

func New(opts map[string]any) (botbridge.Platform, error) {
	botID, _ := opts["bot_id"].(string)
	secret, _ := opts["bot_secret"].(string)
	if strings.TrimSpace(botID) == "" || strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("wecom: bot_id and bot_secret are required")
	}
	return &Platform{
		botID:  strings.TrimSpace(botID),
		secret: strings.TrimSpace(secret),
		dedup:  map[string]time.Time{},
	}, nil
}

func (p *Platform) Name() string { return "wecom" }

func (p *Platform) SetConnectionHandler(h botbridge.ConnectionStatusHandler) {
	p.mu.Lock()
	p.connHandler = h
	p.mu.Unlock()
}

var _ botbridge.ConnectionReporter = (*Platform)(nil)

func (p *Platform) Start(handler botbridge.MessageHandler) error {
	p.handler = handler
	p.ctx, p.cancel = context.WithCancel(context.Background())
	go p.connectLoop()
	return nil
}

func (p *Platform) Stop() error {
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Lock()
	conn := p.conn
	p.mu.Unlock()
	if conn != nil {
		return conn.Close()
	}
	return nil
}

// ---------------------------------------------------------------------------
// 连接循环
// ---------------------------------------------------------------------------

func (p *Platform) connectLoop() {
	backoff := time.Second
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		start := time.Now()
		err := p.runConnection()
		if p.ctx.Err() != nil {
			return
		}
		p.reportConnection(false, err)
		// 连接存活足够久则重置退避。
		if time.Since(start) > 2*wsPingInterval {
			backoff = time.Second
		}
		logx.For("botbridge.wecom").Warn("connection lost, reconnecting", "err", err, "backoff", backoff)
		select {
		case <-time.After(backoff):
		case <-p.ctx.Done():
			return
		}
		backoff *= 2
		if backoff > wsMaxBackoff {
			backoff = wsMaxBackoff
		}
	}
}

func (p *Platform) runConnection() error {
	logx.For("botbridge.wecom").Info("connecting", "endpoint", wsEndpoint)
	conn, _, err := websocket.DefaultDialer.DialContext(p.ctx, wsEndpoint, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	p.mu.Lock()
	p.conn = conn
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.conn = nil
		p.mu.Unlock()
		conn.Close()
		// 排空挂起的 ack 通道，唤醒等待者并清理陈旧条目。
		var staleKeys []any
		p.pendingAcks.Range(func(key, value any) bool {
			if ch, ok := value.(chan wsAckResult); ok {
				select {
				case ch <- wsAckResult{err: fmt.Errorf("wecom: connection closed")}:
				default:
				}
			}
			staleKeys = append(staleKeys, key)
			return true
		})
		for _, k := range staleKeys {
			p.pendingAcks.Delete(k)
		}
	}()

	// 订阅（鉴权）帧。
	subReqID := p.generateReqID("aibot_subscribe")
	subFrame := map[string]any{
		"cmd":     "aibot_subscribe",
		"headers": map[string]string{"req_id": subReqID},
		"body":    map[string]string{"bot_id": p.botID, "secret": p.secret},
	}
	if err := p.writeJSON(subFrame); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	var subResp wsFrame
	if err := conn.ReadJSON(&subResp); err != nil {
		return fmt.Errorf("subscribe response: %w", err)
	}
	if subResp.ErrCode == nil || *subResp.ErrCode != 0 {
		code := 0
		if subResp.ErrCode != nil {
			code = *subResp.ErrCode
		}
		return fmt.Errorf("subscribe failed: errcode=%d errmsg=%s", code, subResp.ErrMsg)
	}
	logx.For("botbridge.wecom").Info("subscribed", "bot_id", p.botID)
	p.missedPong.Store(0)
	p.reportConnection(true, nil)

	heartCtx, heartCancel := context.WithCancel(p.ctx)
	defer heartCancel()
	go p.heartbeat(heartCtx, conn)

	for {
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		default:
		}
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		var frame wsFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			logx.For("botbridge.wecom").Warn("invalid json frame", "err", err)
			continue
		}
		p.handleFrame(frame)
	}
}

func (p *Platform) heartbeat(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if int(p.missedPong.Load()) >= wsMaxMissedPong {
				logx.For("botbridge.wecom").Warn("no heartbeat ack, closing connection")
				conn.Close()
				return
			}
			p.missedPong.Add(1)
			ping := map[string]any{
				"cmd":     "ping",
				"headers": map[string]string{"req_id": p.generateReqID("ping")},
			}
			if err := p.writeJSON(ping); err != nil {
				logx.For("botbridge.wecom").Warn("ping failed", "err", err)
				return
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 帧分发
// ---------------------------------------------------------------------------

func (p *Platform) handleFrame(frame wsFrame) {
	switch frame.Cmd {
	case "aibot_msg_callback":
		p.handleMsgCallback(frame)
	case "aibot_event_callback":
		// 忽略事件回调。
	case "":
		// 响应帧（无 cmd）：按 req_id 前缀识别。
		reqID := frame.Headers.ReqID
		switch {
		case strings.HasPrefix(reqID, "ping"):
			p.missedPong.Store(0)
		case strings.HasPrefix(reqID, "aibot_subscribe"):
			// 迟到的订阅 ack，忽略。
		default:
			var ackErr error
			if frame.ErrCode != nil && *frame.ErrCode != 0 {
				ackErr = fmt.Errorf("wecom: ack error errcode=%d errmsg=%s", *frame.ErrCode, frame.ErrMsg)
			}
			p.dispatchAck(reqID, wsAckResult{frame: frame, err: ackErr})
		}
	}
}

func (p *Platform) dispatchAck(reqID string, result wsAckResult) {
	ch, ok := p.pendingAcks.LoadAndDelete(reqID)
	if !ok {
		return
	}
	if resultCh, ok := ch.(chan wsAckResult); ok {
		select {
		case resultCh <- result:
		default:
		}
	}
}

func (p *Platform) handleMsgCallback(frame wsFrame) {
	var body wsMsgCallbackBody
	if err := json.Unmarshal(frame.Body, &body); err != nil {
		logx.For("botbridge.wecom").Warn("parse msg_callback body failed", "err", err)
		return
	}
	if p.isDuplicate(body.MsgID) {
		return
	}

	chatID := body.ChatID
	if chatID == "" {
		chatID = body.From.UserID
	}
	if chatID == "" {
		return
	}
	content := stripWeComAtMentions(collectText(&body), p.botID, body.AibotID)
	if content == "" {
		return
	}

	scope := fmt.Sprintf("wecom:%s:%s", chatID, body.From.UserID)
	rctx := &replyContext{
		reqID:    frame.Headers.ReqID,
		chatID:   chatID,
		chatType: body.ChatType,
		userID:   body.From.UserID,
		scopeKey: scope,
	}
	p.emit(&botbridge.Message{
		MessageID: body.MsgID,
		ScopeKey:  scope,
		Platform:  "wecom",
		UserID:    body.From.UserID,
		Content:   content,
		ReplyCtx:  rctx,
	})
}

// ---------------------------------------------------------------------------
// 发送消息
// ---------------------------------------------------------------------------

func (p *Platform) Reply(ctx context.Context, replyCtx any, content string) error {
	rc, ok := p.parseReplyContext(replyCtx)
	if !ok {
		return fmt.Errorf("wecom: invalid reply context %T", replyCtx)
	}
	if content == "" {
		return nil
	}
	// 有 req_id 时走 aibot_respond_msg 流式回复（一次性整体替换，finish=true）。
	if rc.reqID != "" {
		streamID := p.generateReqID("stream")
		frame := map[string]any{
			"cmd":     "aibot_respond_msg",
			"headers": map[string]string{"req_id": rc.reqID},
			"body": map[string]any{
				"msgtype": "stream",
				"stream":  map[string]any{"id": streamID, "finish": true, "content": content},
			},
		}
		return p.writeJSON(frame)
	}
	return p.Send(ctx, rc, content)
}

func (p *Platform) Send(ctx context.Context, replyCtx any, content string) error {
	rc, ok := p.parseReplyContext(replyCtx)
	if !ok {
		return fmt.Errorf("wecom: invalid reply context %T", replyCtx)
	}
	if content == "" {
		return nil
	}
	if rc.chatID == "" {
		return fmt.Errorf("wecom: empty chatID, cannot send")
	}
	for i, chunk := range splitByBytes(content, messageMaxBytes) {
		reqID := p.generateReqID("aibot_send_msg")
		frame := map[string]any{
			"cmd":     "aibot_send_msg",
			"headers": map[string]string{"req_id": reqID},
			"body": map[string]any{
				"chatid":   rc.chatID,
				"msgtype":  "markdown",
				"markdown": map[string]string{"content": chunk},
			},
		}
		if err := p.writeAndWaitAck(ctx, frame, reqID); err != nil {
			return fmt.Errorf("wecom: send chunk %d: %w", i, err)
		}
	}
	return nil
}

// writeAndWaitAck 发送帧并等待服务端 ack；超时则放行（返回 nil）避免死锁。
func (p *Platform) writeAndWaitAck(ctx context.Context, frame map[string]any, reqID string) error {
	ch := make(chan wsAckResult, 1)
	p.pendingAcks.Store(reqID, ch)
	defer p.pendingAcks.Delete(reqID)
	if err := p.writeJSON(frame); err != nil {
		return err
	}
	select {
	case res := <-ch:
		return res.err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wsAckTimeout):
		logx.For("botbridge.wecom").Debug("ack timeout, proceeding", "req_id", reqID)
		return nil
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

// reconstructReplyCtx 从 scopeKey "wecom:<chatID>:<userID>" 还原回信上下文。无 req_id，
// 只能走 Send（aibot_send_msg 主动发送），不能走 Reply 的流式回复。
func reconstructReplyCtx(scope string) (*replyContext, bool) {
	parts := strings.SplitN(scope, ":", 3)
	if len(parts) < 3 || parts[0] != "wecom" || parts[1] == "" {
		return nil, false
	}
	return &replyContext{chatID: parts[1], userID: parts[2], scopeKey: scope}, true
}

func (p *Platform) generateReqID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, p.reqSeq.Add(1))
}

func (p *Platform) writeJSON(v any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn == nil {
		return fmt.Errorf("wecom: not connected")
	}
	return p.conn.WriteJSON(v)
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

// isDuplicate 按 MsgID 去重（企业微信可能重投），60s 滚动窗口驱逐旧条目。
func (p *Platform) isDuplicate(msgID string) bool {
	if msgID == "" {
		return false
	}
	p.dedupMu.Lock()
	defer p.dedupMu.Unlock()
	now := time.Now()
	for k, t := range p.dedup {
		if now.Sub(t) > dedupTTL {
			delete(p.dedup, k)
		}
	}
	if _, ok := p.dedup[msgID]; ok {
		return true
	}
	p.dedup[msgID] = now
	return false
}

// ---------------------------------------------------------------------------
// 帧类型 / 文本抽取
// ---------------------------------------------------------------------------

type wsFrame struct {
	Cmd     string          `json:"cmd,omitempty"`
	Headers wsFrameHeaders  `json:"headers"`
	Body    json.RawMessage `json:"body,omitempty"`
	ErrCode *int            `json:"errcode,omitempty"`
	ErrMsg  string          `json:"errmsg,omitempty"`
}

type wsFrameHeaders struct {
	ReqID string `json:"req_id"`
}

type wsAckResult struct {
	frame wsFrame
	err   error
}

// wsMsgCallbackBody 仅保留文本相关字段（图片 / 文件已裁掉）。
type wsMsgCallbackBody struct {
	MsgID    string `json:"msgid"`
	AibotID  string `json:"aibotid"`
	ChatID   string `json:"chatid"`
	ChatType string `json:"chattype"`
	From     struct {
		UserID string `json:"userid"`
	} `json:"from"`
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
	Voice struct {
		Text    string `json:"text,omitempty"`
		Content string `json:"content,omitempty"`
	} `json:"voice"`
	Mixed *wsMixedBlock `json:"mixed,omitempty"`
	Quote *wsQuoteBlock `json:"quote,omitempty"`
}

type wsMixedItem struct {
	MsgType string `json:"msgtype"`
	Text    *struct {
		Content string `json:"content"`
	} `json:"text,omitempty"`
}

type wsMixedBlock struct {
	MsgItem []wsMixedItem `json:"msg_item"`
}

type wsQuoteBlock struct {
	MsgType string `json:"msgtype"`
	Text    *struct {
		Content string `json:"content"`
	} `json:"text,omitempty"`
	Voice *struct {
		Content string `json:"content"`
	} `json:"voice,omitempty"`
}

// collectText 抽取一条回调里的全部文本：语音转写 + 主体文本 / mixed 文本段 + quote 文本。
func collectText(body *wsMsgCallbackBody) string {
	var parts []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			parts = append(parts, s)
		}
	}
	if body.MsgType == "voice" {
		add(wsVoiceText(body.Voice))
	}
	if body.Mixed != nil && len(body.Mixed.MsgItem) > 0 {
		for _, item := range body.Mixed.MsgItem {
			if item.MsgType == "text" && item.Text != nil {
				add(item.Text.Content)
			}
		}
	} else {
		add(body.Text.Content)
	}
	if q := body.Quote; q != nil {
		switch q.MsgType {
		case "text":
			if q.Text != nil {
				add(q.Text.Content)
			}
		case "voice":
			if q.Voice != nil {
				add(q.Voice.Content)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func wsVoiceText(v struct {
	Text    string `json:"text,omitempty"`
	Content string `json:"content,omitempty"`
}) string {
	if s := strings.TrimSpace(v.Content); s != "" {
		return s
	}
	return strings.TrimSpace(v.Text)
}

// ---------------------------------------------------------------------------
// 文本切分 / @提及剥离（移植自 cc-connect message_split.go / mention_strip.go，做了精简）
// ---------------------------------------------------------------------------

// splitByBytes 按 UTF-8 字节上限切分，优先在窗口内换行处断开，否则按 rune 边界硬切，
// 避免切坏多字节字符。
func splitByBytes(s string, maxBytes int) []string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return []string{s}
	}
	var out []string
	for len(s) > maxBytes {
		cut := maxBytes
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		if nl := strings.LastIndexByte(s[:cut], '\n'); nl > maxBytes/2 {
			cut = nl + 1
		}
		if cut == 0 {
			_, size := utf8.DecodeRuneInString(s)
			cut = size
		}
		out = append(out, s[:cut])
		s = s[cut:]
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}

// stripWeComAtMentions 去掉 @<botId> / ＠<botId> 段，使群里「@机器人 命令」仍能匹配。
func stripWeComAtMentions(s string, botIDs ...string) string {
	s = strings.TrimSpace(s)
	for _, id := range botIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		s = strings.TrimSpace(stripOneWeComAtMention(s, id))
	}
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return stripLeadingDisplayMentionCommand(strings.TrimSpace(s))
}

func stripOneWeComAtMention(s, botID string) string {
	if s == "" || botID == "" {
		return s
	}
	s = removeAllEqualFold(s, "＠"+botID)
	needleLower := "@" + strings.ToLower(botID)
	for {
		lower := strings.ToLower(s)
		idx := strings.Index(lower, needleLower)
		if idx < 0 {
			return s
		}
		end := idx + len(needleLower)
		if end > len(s) {
			return s
		}
		s = s[:idx] + s[end:]
	}
}

func removeAllEqualFold(s, sub string) string {
	if sub == "" {
		return s
	}
	subLower := strings.ToLower(sub)
	for {
		idx := strings.Index(strings.ToLower(s), subLower)
		if idx < 0 {
			return s
		}
		s = s[:idx] + s[idx+len(sub):]
	}
}

// stripLeadingDisplayMentionCommand 去掉 / 或 ! 命令前的前导 @提及（WS 回调不带结构化提及元数据，
// 显示名直接混在文本里）。在 token 边界（前面是空白）匹配，避免误伤 https:// 或 1/2。
func stripLeadingDisplayMentionCommand(s string) string {
	if s == "" || (!strings.HasPrefix(s, "@") && !strings.HasPrefix(s, "＠")) {
		return s
	}
	for i, r := range s {
		if r != '/' && r != '!' {
			continue
		}
		if i == 0 {
			return s
		}
		prev, _ := utf8.DecodeLastRuneInString(s[:i])
		if unicode.IsSpace(prev) {
			return strings.TrimSpace(s[i:])
		}
	}
	return s
}
