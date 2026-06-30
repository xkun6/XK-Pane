// Package weixin 是微信个人号（ilink bot HTTP API，与 OpenClaw openclaw-weixin 同后端）的
// botbridge 文本适配器，移植自 cc-connect platform/weixin 的右半边（长轮询 getUpdates +
// sendMessage），裁掉 XK-Pane 用不到的部分：图片 / 语音 / 文件 CDN 上传下载、打字状态指示、
// allow_from 白名单、代理、磁盘持久化（state_dir）、引擎生命周期回调。
//
// 凭据只需 token（ilink bot Bearer token）。每条来信带 context_token，发送时必须携带；
// context_token 会随每条来信刷新。这里只在内存里按 peer 缓存 context_token 与长轮询游标
//（不落盘）——重启后用户重新发一条消息即可恢复会话令牌，对桌面端实时场景足够。
//
// 因不实现 CardSender / InlineButtonSender，botbridge 视其为纯文本平台：AskUserQuestion
// 自动走「文字编号问答」兜底（见 dispatcher.parseTextSelections / timeline.askText）。
package weixin

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"xkpane/internal/services/botbridge"
	"xkpane/internal/utils/httpx"
	"xkpane/internal/utils/logx"
)

func init() {
	botbridge.RegisterPlatform("weixin", New)
}

const (
	defaultBaseURL         = "https://ilinkai.weixin.qq.com"
	channelVersion         = "xkpane-weixin/1.0"
	defaultLongPollTimeout = 35 * time.Second
	defaultAPITimeout      = 15 * time.Second
	maxResponseBody        = 64 << 20

	scopePrefix    = "weixin:dm:"
	maxWeixinChunk = 3800
	sendMaxRetries = 3
	sendRetryDelay = 500 * time.Millisecond
	chunkSendDelay = 100 * time.Millisecond
	dedupTTL       = 5 * time.Minute
	sessionPause   = 30 * time.Second

	// ilink 协议常量。
	messageTypeUser       = 1
	messageTypeBot        = 2
	messageItemText       = 1
	messageItemImage      = 2
	messageItemVoice      = 3
	messageItemFile       = 4
	messageItemVideo      = 5
	messageStateFinish    = 2
	sessionExpiredErrcode = -14
)

type replyContext struct {
	peerUserID   string
	contextToken string
}

type Platform struct {
	token   string
	baseURL string

	handler     botbridge.MessageHandler
	connHandler botbridge.ConnectionStatusHandler
	connected   atomic.Bool

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex

	syncBufMu sync.Mutex
	syncBuf   string

	tokensMu sync.RWMutex
	tokens   map[string]string

	dedupMu sync.Mutex
	dedup   map[string]time.Time
}

func New(opts map[string]any) (botbridge.Platform, error) {
	token, _ := opts["token"].(string)
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("weixin: token is required (ilink bot Bearer token)")
	}
	return &Platform{
		token:   strings.TrimSpace(token),
		baseURL: strings.TrimRight(defaultBaseURL, "/") + "/",
		tokens:  map[string]string{},
		dedup:   map[string]time.Time{},
	}, nil
}

func (p *Platform) Name() string { return "weixin" }

func (p *Platform) SetConnectionHandler(h botbridge.ConnectionStatusHandler) {
	p.mu.Lock()
	p.connHandler = h
	p.mu.Unlock()
}

var _ botbridge.ConnectionReporter = (*Platform)(nil)

func (p *Platform) Start(handler botbridge.MessageHandler) error {
	p.handler = handler
	p.ctx, p.cancel = context.WithCancel(context.Background())
	go p.pollLoop(p.ctx)
	return nil
}

func (p *Platform) Stop() error {
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

func (p *Platform) Reply(ctx context.Context, replyCtx any, content string) error {
	return p.sendChunks(ctx, replyCtx, content)
}

func (p *Platform) Send(ctx context.Context, replyCtx any, content string) error {
	return p.sendChunks(ctx, replyCtx, content)
}

// ---------------------------------------------------------------------------
// 长轮询
// ---------------------------------------------------------------------------

func (p *Platform) pollLoop(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		p.syncBufMu.Lock()
		buf := p.syncBuf
		p.syncBufMu.Unlock()

		resp, err := p.getUpdates(ctx, buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			p.setConnected(false, err)
			logx.For("botbridge.weixin").Warn("getUpdates failed", "err", err, "backoff", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < maxBackoff {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
			continue
		}
		backoff = time.Second

		if resp.Errcode == sessionExpiredErrcode {
			p.setConnected(false, fmt.Errorf("weixin: session expired (errcode=%d)", resp.Errcode))
			logx.For("botbridge.weixin").Warn("session expired, pausing", "pause", sessionPause)
			select {
			case <-ctx.Done():
				return
			case <-time.After(sessionPause):
			}
			continue
		}
		// 一次成功的长轮询往返即代表 token 被接受、链路畅通。
		p.setConnected(true, nil)
		if resp.Ret != 0 && resp.Errmsg != "" {
			logx.For("botbridge.weixin").Warn("getUpdates ret", "ret", resp.Ret, "errcode", resp.Errcode, "errmsg", resp.Errmsg)
		}

		var wg sync.WaitGroup
		for i := range resp.Msgs {
			wg.Add(1)
			go func(m *weixinMessage) {
				defer wg.Done()
				p.dispatchInbound(m)
			}(&resp.Msgs[i])
		}
		wg.Wait()

		if ctx.Err() == nil && resp.GetUpdatesBuf != "" {
			p.syncBufMu.Lock()
			p.syncBuf = resp.GetUpdatesBuf
			p.syncBufMu.Unlock()
		}
	}
}

func (p *Platform) dispatchInbound(m *weixinMessage) {
	if m == nil || m.MessageType == messageTypeBot {
		return
	}
	if m.MessageType != 0 && m.MessageType != messageTypeUser {
		return
	}
	from := strings.TrimSpace(m.FromUserID)
	if from == "" {
		return
	}
	dedupKey := fmt.Sprintf("%s|%d|%d|%d|%s", from, m.MessageID, m.Seq, m.CreateTimeMs, strings.TrimSpace(m.ClientID))
	if p.isDuplicate(dedupKey) {
		return
	}
	if tok := strings.TrimSpace(m.ContextToken); tok != "" {
		p.setContextToken(from, tok)
	}

	body := strings.TrimSpace(bodyFromItemList(m.ItemList))
	if body == "" {
		return
	}

	msgID := fmt.Sprintf("%d", m.MessageID)
	if m.MessageID == 0 {
		msgID = randomHex(8)
	}
	p.emit(&botbridge.Message{
		MessageID: msgID,
		ScopeKey:  scopePrefix + from,
		Platform:  "weixin",
		UserID:    from,
		Content:   body,
		ReplyCtx:  &replyContext{peerUserID: from, contextToken: strings.TrimSpace(m.ContextToken)},
	})
}

// ---------------------------------------------------------------------------
// 发送消息
// ---------------------------------------------------------------------------

func (p *Platform) sendChunks(ctx context.Context, replyCtx any, content string) error {
	rc, ok := p.parseReplyContext(replyCtx)
	if !ok {
		return fmt.Errorf("weixin: invalid reply context %T", replyCtx)
	}
	if strings.TrimSpace(rc.contextToken) == "" {
		rc.contextToken = p.getContextToken(rc.peerUserID)
	}
	if strings.TrimSpace(rc.contextToken) == "" {
		return fmt.Errorf("weixin: missing context_token for %q — 用户需先发一条消息刷新会话令牌", rc.peerUserID)
	}
	if strings.TrimSpace(content) == "" {
		return nil
	}
	chunks := splitUTF8(content, maxWeixinChunk)
	for i, chunk := range chunks {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(chunkSendDelay):
			}
		}
		if err := p.sendChunkWithRetry(ctx, rc, chunk); err != nil {
			return fmt.Errorf("weixin: send chunk %d/%d: %w", i+1, len(chunks), err)
		}
	}
	return nil
}

// sendChunkWithRetry 在 ret=-2（令牌过期）时尝试用最新缓存的 context_token 重试。
// 若拿不到更新的令牌（与当前相同或为空），快速失败而不空耗重试次数。
func (p *Platform) sendChunkWithRetry(ctx context.Context, rc *replyContext, chunk string) error {
	var lastErr error
	for attempt := 0; attempt < sendMaxRetries; attempt++ {
		clientID := "xk-" + randomHex(6)
		err := p.sendText(ctx, rc.peerUserID, chunk, rc.contextToken, clientID)
		if err == nil {
			return nil
		}
		lastErr = err
		if !strings.Contains(err.Error(), "ret=-2") {
			return err
		}
		fresh := p.getContextToken(rc.peerUserID)
		if fresh == "" || fresh == rc.contextToken {
			return fmt.Errorf("weixin: sendMessage ret=-2（context_token 过期）— 用户需向 %q 重新发一条消息刷新令牌: %w", rc.peerUserID, lastErr)
		}
		rc.contextToken = fresh
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sendRetryDelay):
		}
	}
	return lastErr
}

func (p *Platform) sendText(ctx context.Context, to, text, contextToken, clientID string) error {
	if strings.TrimSpace(contextToken) == "" {
		return fmt.Errorf("weixin: context_token is required for send")
	}
	msg := sendMessageReq{
		Msg: weixinOutboundMsg{
			ToUserID:     to,
			ClientID:     clientID,
			MessageType:  messageTypeBot,
			MessageState: messageStateFinish,
			ItemList:     []messageItem{{Type: messageItemText, TextItem: &textItem{Text: text}}},
			ContextToken: contextToken,
		},
		BaseInfo: baseInfo{ChannelVersion: channelVersion},
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	reqCtx, cancel := context.WithTimeout(ctx, defaultAPITimeout)
	defer cancel()
	raw, err := p.post(reqCtx, "ilink/bot/sendmessage", payload)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var resp sendMessageResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("weixin: sendMessage response json: %w", err)
	}
	if resp.Ret != 0 {
		return fmt.Errorf("weixin: sendMessage ret=%d errcode=%d errmsg=%s", resp.Ret, resp.Errcode, resp.Errmsg)
	}
	return nil
}

// ---------------------------------------------------------------------------
// HTTP（经 httpx，带 ilink 专用头）
// ---------------------------------------------------------------------------

func (p *Platform) getUpdates(ctx context.Context, buf string) (*getUpdatesResp, error) {
	payload, err := json.Marshal(getUpdatesReq{
		GetUpdatesBuf: buf,
		BaseInfo:      baseInfo{ChannelVersion: channelVersion},
	})
	if err != nil {
		return nil, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, defaultLongPollTimeout+5*time.Second)
	defer cancel()
	raw, err := p.post(reqCtx, "ilink/bot/getupdates", payload)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// 长轮询正常超时（无新消息）：保持游标，返回空成功。
		if errors.Is(err, context.DeadlineExceeded) {
			return &getUpdatesResp{GetUpdatesBuf: buf}, nil
		}
		return nil, err
	}
	var out getUpdatesResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("weixin: getUpdates json: %w", err)
	}
	return &out, nil
}

// post 以 ilink 专用头发起 POST。ctx 须自带 deadline（httpx.NewRequest 不会再叠加默认超时）。
func (p *Platform) post(ctx context.Context, endpoint string, payload []byte) ([]byte, error) {
	req, cancel, err := httpx.NewRequest(ctx, http.MethodPost, p.baseURL+strings.TrimPrefix(endpoint, "/"), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer cancel()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("X-WECHAT-UIN", randomWechatUIN())
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	resp, err := httpx.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("weixin: %s read body: %w", endpoint, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weixin: %s http %d", endpoint, resp.StatusCode)
	}
	return raw, nil
}

// ---------------------------------------------------------------------------
// 上下文 / 状态
// ---------------------------------------------------------------------------

func (p *Platform) parseReplyContext(v any) (*replyContext, bool) {
	switch x := v.(type) {
	case *replyContext:
		return x, true
	case replyContext:
		return &x, true
	case string:
		peer, ok := peerFromScope(x)
		if !ok {
			return nil, false
		}
		return &replyContext{peerUserID: peer, contextToken: p.getContextToken(peer)}, true
	default:
		return nil, false
	}
}

// peerFromScope 从 scopeKey "weixin:dm:<peer>" 取出 peer。
func peerFromScope(scope string) (string, bool) {
	if !strings.HasPrefix(scope, scopePrefix) {
		return "", false
	}
	peer := strings.TrimSpace(strings.TrimPrefix(scope, scopePrefix))
	if peer == "" {
		return "", false
	}
	return peer, true
}

func (p *Platform) setContextToken(peer, tok string) {
	if peer == "" || tok == "" {
		return
	}
	p.tokensMu.Lock()
	p.tokens[peer] = tok
	p.tokensMu.Unlock()
}

func (p *Platform) getContextToken(peer string) string {
	p.tokensMu.RLock()
	defer p.tokensMu.RUnlock()
	return p.tokens[peer]
}

func (p *Platform) isDuplicate(key string) bool {
	p.dedupMu.Lock()
	defer p.dedupMu.Unlock()
	now := time.Now()
	for k, ts := range p.dedup {
		if now.Sub(ts) > dedupTTL {
			delete(p.dedup, k)
		}
	}
	if _, ok := p.dedup[key]; ok {
		return true
	}
	p.dedup[key] = now
	return false
}

func (p *Platform) setConnected(v bool, err error) {
	if p.connected.Swap(v) != v {
		p.reportConnection(v, err)
	}
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

// ---------------------------------------------------------------------------
// 文本抽取 / 切分 / 杂项（移植自 cc-connect parse.go）
// ---------------------------------------------------------------------------

func isMediaItemType(t int) bool {
	switch t {
	case messageItemImage, messageItemVoice, messageItemFile, messageItemVideo:
		return true
	default:
		return false
	}
}

// bodyFromItemList 从 item_list 抽取用户可见文本（文本、引用、语音 ASR）。
func bodyFromItemList(items []messageItem) string {
	if len(items) == 0 {
		return ""
	}
	for _, item := range items {
		switch item.Type {
		case messageItemText:
			if item.TextItem == nil {
				continue
			}
			text := strings.TrimSpace(item.TextItem.Text)
			ref := item.RefMsg
			if ref == nil {
				return text
			}
			if ref.MessageItem != nil && isMediaItemType(ref.MessageItem.Type) {
				return text
			}
			var parts []string
			if ref.Title != "" {
				parts = append(parts, ref.Title)
			}
			if ref.MessageItem != nil {
				if refBody := bodyFromItemList([]messageItem{*ref.MessageItem}); refBody != "" {
					parts = append(parts, refBody)
				}
			}
			if len(parts) == 0 {
				return text
			}
			return fmt.Sprintf("[引用: %s]\n%s", strings.Join(parts, " | "), text)
		case messageItemVoice:
			if item.VoiceItem != nil && strings.TrimSpace(item.VoiceItem.Text) != "" {
				return strings.TrimSpace(item.VoiceItem.Text)
			}
		}
	}
	return ""
}

func splitUTF8(s string, maxRunes int) []string {
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return []string{s}
	}
	var out []string
	runes := []rune(s)
	for len(runes) > 0 {
		n := maxRunes
		if len(runes) < n {
			n = len(runes)
		}
		out = append(out, string(runes[:n]))
		runes = runes[n:]
	}
	return out
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func randomWechatUIN() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return base64.StdEncoding.EncodeToString([]byte("0000"))
	}
	u := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", u)))
}

// ---------------------------------------------------------------------------
// ilink JSON 形状（仅保留文本相关字段）
// ---------------------------------------------------------------------------

type baseInfo struct {
	ChannelVersion string `json:"channel_version,omitempty"`
}

type getUpdatesReq struct {
	GetUpdatesBuf string   `json:"get_updates_buf"`
	BaseInfo      baseInfo `json:"base_info"`
}

type getUpdatesResp struct {
	Ret           int             `json:"ret"`
	Errcode       int             `json:"errcode"`
	Errmsg        string          `json:"errmsg"`
	Msgs          []weixinMessage `json:"msgs"`
	GetUpdatesBuf string          `json:"get_updates_buf"`
}

type weixinMessage struct {
	Seq          int64         `json:"seq,omitempty"`
	MessageID    int64         `json:"message_id,omitempty"`
	FromUserID   string        `json:"from_user_id,omitempty"`
	ClientID     string        `json:"client_id,omitempty"`
	CreateTimeMs int64         `json:"create_time_ms,omitempty"`
	MessageType  int           `json:"message_type,omitempty"`
	MessageState int           `json:"message_state,omitempty"`
	ItemList     []messageItem `json:"item_list,omitempty"`
	ContextToken string        `json:"context_token,omitempty"`
}

type textItem struct {
	Text string `json:"text,omitempty"`
}

type voiceItem struct {
	Text string `json:"text,omitempty"`
}

type refMessage struct {
	MessageItem *messageItem `json:"message_item,omitempty"`
	Title       string       `json:"title,omitempty"`
}

type messageItem struct {
	Type      int         `json:"type,omitempty"`
	TextItem  *textItem   `json:"text_item,omitempty"`
	VoiceItem *voiceItem  `json:"voice_item,omitempty"`
	RefMsg    *refMessage `json:"ref_msg,omitempty"`
}

type sendMessageReq struct {
	Msg      weixinOutboundMsg `json:"msg"`
	BaseInfo baseInfo          `json:"base_info"`
}

type sendMessageResp struct {
	Ret     int    `json:"ret"`
	Errcode int    `json:"errcode"`
	Errmsg  string `json:"errmsg"`
}

type weixinOutboundMsg struct {
	FromUserID   string        `json:"from_user_id"`
	ToUserID     string        `json:"to_user_id"`
	ClientID     string        `json:"client_id"`
	MessageType  int           `json:"message_type"`
	MessageState int           `json:"message_state"`
	ItemList     []messageItem `json:"item_list,omitempty"`
	ContextToken string        `json:"context_token,omitempty"`
}
