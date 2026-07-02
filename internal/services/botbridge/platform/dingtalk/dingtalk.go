// Package dingtalk 是钉钉（DingTalk）Stream 模式机器人的 botbridge 文本适配器，
// 移植自 cc-connect platform/dingtalk 的右半边（Stream 长连接收消息 + sessionWebhook /
// 主动推送 API 回消息），裁掉 XK-Pane 用不到的部分：图片 / 语音 / 文件的下载与发送、
// 打字状态与表情回应、AI 交互卡片、allow_from 白名单、agent_id 工作通知。
//
// 因不实现 CardSender / InlineButtonSender，botbridge 视其为纯文本平台：AskUserQuestion
// 自动走「文字编号问答」兜底（见 dispatcher.parseTextSelections / timeline.askText）。
//
// 走官方 SDK github.com/open-dingtalk/dingtalk-stream-sdk-go：出站长连接（无需公网回调
// 地址），SDK 内建心跳与断线自动重连。凭据为企业内部应用的 client_id / client_secret，
// robot_code 用于主动推送（未填回退 client_id）。
package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"xkpane/internal/services/botbridge"
	"xkpane/internal/utils/httpx"
	"xkpane/internal/utils/logx"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	dingtalkclient "github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
	dingtalkutils "github.com/open-dingtalk/dingtalk-stream-sdk-go/utils"
)

func init() {
	botbridge.RegisterPlatform("dingtalk", New)
}

const (
	tokenURL     = "https://api.dingtalk.com/v1.0/oauth2/accessToken"
	groupMsgURL  = "https://api.dingtalk.com/v1.0/robot/groupMessages/send"
	directMsgURL = "https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend"

	maxResponseBody       = 1 << 20 // 1 MiB
	tokenExpiryBuffer     = 300     // 提前 5 分钟视为过期
	dedupTTL              = 60 * time.Second
	maxQuotedMessageRunes = 4000

	initialBackoff = 3 * time.Second
	maxBackoff     = 60 * time.Second
)

// replyContext 携带回信所需信息。sessionWebhook 非空走临时会话 webhook 回复，
// proactive（由 scopeKey 还原）走主动推送 API：群用 conversationID，单聊用 senderStaffID。
type replyContext struct {
	sessionWebhook string
	conversationID string
	senderStaffID  string
	messageID      string
	isGroup        bool
	proactive      bool
}

// richTextContent 手工镜像钉钉回调里完整的 "text" 对象——SDK 的 typed 模型
// （BotCallbackDataTextModel 只有 Content）会静默丢掉 isReplyMsg / repliedMsg。
type richTextContent struct {
	Content    string          `json:"content"`
	IsReplyMsg bool            `json:"isReplyMsg"`
	RepliedMsg *repliedMessage `json:"repliedMsg"`
}

type repliedMessage struct {
	MsgType string          `json:"msgType"`
	Content json.RawMessage `json:"content"`
}

type repliedTextContent struct {
	Text string `json:"text"`
}

type Platform struct {
	clientID     string
	clientSecret string
	robotCode    string

	handler     botbridge.MessageHandler
	connHandler botbridge.ConnectionStatusHandler
	connMu      sync.Mutex

	streamClient    *dingtalkclient.StreamClient
	streamCtxCancel context.CancelFunc
	stopped         atomic.Bool

	tokenMu     sync.Mutex
	accessToken string
	tokenExpiry time.Time

	dedupMu sync.Mutex
	dedup   map[string]time.Time
}

func New(opts map[string]any) (botbridge.Platform, error) {
	clientID, _ := opts["client_id"].(string)
	clientID = strings.TrimSpace(clientID)
	clientSecret, _ := opts["client_secret"].(string)
	clientSecret = strings.TrimSpace(clientSecret)
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("dingtalk: client_id and client_secret are required")
	}
	robotCode, _ := opts["robot_code"].(string)
	robotCode = strings.TrimSpace(robotCode)
	if robotCode == "" {
		robotCode = clientID // 未填时回退到 client_id
	}
	return &Platform{
		clientID:     clientID,
		clientSecret: clientSecret,
		robotCode:    robotCode,
		dedup:        map[string]time.Time{},
	}, nil
}

func (p *Platform) Name() string { return "dingtalk" }

func (p *Platform) SetConnectionHandler(h botbridge.ConnectionStatusHandler) {
	p.connMu.Lock()
	p.connHandler = h
	p.connMu.Unlock()
}

var _ botbridge.ConnectionReporter = (*Platform)(nil)

func (p *Platform) Start(handler botbridge.MessageHandler) error {
	p.handler = handler
	p.streamClient = dingtalkclient.NewStreamClient(
		dingtalkclient.WithAppCredential(dingtalkclient.NewAppCredentialConfig(p.clientID, p.clientSecret)),
	)
	// 注册原始帧路由（而非 typed 回调），以便读到 SDK typed 模型会丢弃的
	// text.isReplyMsg / text.repliedMsg 字段。
	p.streamClient.RegisterRouter(dingtalkutils.SubscriptionTypeKCallback, payload.BotMessageCallbackTopic,
		func(_ context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
			// SDK 的 reconnect() 是不检查 AutoReconnect 的无限循环，Stop 管不住在途
			// 重连——已停实例的孤儿连接若对消息 ACK 成功，钉钉即认为已送达、不再重推，
			// 消息永久黑洞。返回错误让钉钉重推给后继连接，并顺手关掉孤儿连接
			// （processLoop 退出时 AutoReconnect 已为 false，不会再重连）。
			if p.stopped.Load() {
				if sc := p.streamClient; sc != nil {
					go sc.Close()
				}
				return nil, fmt.Errorf("dingtalk: adapter stopped")
			}
			p.onRawMessage(df.Data)
			return payload.NewSuccessDataFrameResponse(), nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	p.streamCtxCancel = cancel
	go p.connectLoop(ctx)
	return nil
}

func (p *Platform) Stop() error {
	p.stopped.Store(true)
	if p.streamCtxCancel != nil {
		p.streamCtxCancel()
	}
	if p.streamClient != nil {
		// 关闭前先关掉 SDK 内建自动重连：否则 Close() 触发的断线会让 SDK 用
		// context.Background() 无限重连，泄漏一个仍在向 handler 投递消息的孤儿连接。
		p.streamClient.AutoReconnect = false
		p.streamClient.Close()
	}
	return nil
}

// connectLoop 只负责「首连」：SDK 的 Start 建连后即返回（processLoop 在后台 goroutine
// 跑心跳），首连成功后交由 SDK 内建 AutoReconnect 维持长连接。仅当首连失败（如启动时
// 网络不通）才由本循环退避重试。
func (p *Platform) connectLoop(ctx context.Context) {
	backoff := initialBackoff
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		err := p.streamClient.Start(ctx)
		if err == nil {
			// Stop 可能与在途 Start 竞争完成（彼时 conn 尚为 nil，Stop 的 Close 是
			// no-op）：这里补一次收尾，防止孤儿连接存活并把「已连入」污染回 service。
			if p.stopped.Load() {
				p.streamClient.Close()
				return
			}
			logx.For("botbridge.dingtalk").Info("stream connected", "client_id", p.clientID)
			p.reportConnection(true, nil)
			return
		}
		logx.For("botbridge.dingtalk").Warn("stream connect failed, retrying", "err", err, "backoff", backoff)
		p.reportConnection(false, err)
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
	}
}

// ---------------------------------------------------------------------------
// 收消息
// ---------------------------------------------------------------------------

func (p *Platform) onRawMessage(rawJSON string) {
	var data chatbot.BotCallbackDataModel
	if err := json.Unmarshal([]byte(rawJSON), &data); err != nil {
		logx.For("botbridge.dingtalk").Error("parse callback failed", "err", err)
		return
	}
	// 从原始 JSON 再解一遍 text 对象，恢复 typed 模型丢掉的引用/回复信息。
	var envelope struct {
		Text richTextContent `json:"text"`
	}
	_ = json.Unmarshal([]byte(rawJSON), &envelope)
	p.onMessage(&data, &envelope.Text)
}

func (p *Platform) onMessage(data *chatbot.BotCallbackDataModel, richText *richTextContent) {
	if p.isDuplicate(data.MsgId) {
		return
	}

	convType := "d" // 单聊（1:1）
	if data.ConversationType == "2" {
		convType = "g" // 群
	}
	scopeKey := fmt.Sprintf("dingtalk:%s:%s:%s", convType, data.ConversationId, data.SenderStaffId)

	var content string
	switch data.Msgtype {
	case "text":
		content = data.Text.Content
		if richText != nil && richText.IsReplyMsg && richText.RepliedMsg != nil {
			content = formatReplyContent(richText, content)
		}
	case "richText":
		content = extractRichText(data.Content)
	default:
		// 纯文本适配器只处理 text / richText，忽略图片 / 语音 / 文件等。
		logx.For("botbridge.dingtalk").Debug("ignoring non-text message", "msgtype", data.Msgtype)
		return
	}
	if strings.TrimSpace(content) == "" {
		return
	}

	p.emit(&botbridge.Message{
		MessageID: data.MsgId,
		ScopeKey:  scopeKey,
		Platform:  "dingtalk",
		UserID:    data.SenderStaffId,
		Content:   content,
		ReplyCtx: replyContext{
			sessionWebhook: data.SessionWebhook,
			conversationID: data.ConversationId,
			senderStaffID:  data.SenderStaffId,
			messageID:      data.MsgId,
			isGroup:        data.ConversationType == "2",
		},
	})
}

func (p *Platform) emit(msg *botbridge.Message) {
	if p.stopped.Load() {
		return
	}
	if h := p.handler; h != nil {
		go h(p, msg)
	}
}

// extractRichText 从钉钉 richText content 里抽纯文本，跳过图片等非文本片段。
// 结构：{"richText": [{"text": "..."}, {"text": "...", "attrs": {...}}, ...]}
func extractRichText(content any) string {
	m, ok := content.(map[string]any)
	if !ok {
		return ""
	}
	parts, ok := m["richText"].([]any)
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, part := range parts {
		item, ok := part.(map[string]any)
		if !ok {
			continue
		}
		if text, ok := item["text"].(string); ok {
			b.WriteString(text)
		}
	}
	return strings.TrimSpace(b.String())
}

// formatReplyContent 在用户引用/回复某条消息时，把被引文本拼到正文前。
func formatReplyContent(richText *richTextContent, fallback string) string {
	content := richText.Content
	if content == "" {
		content = fallback
	}
	if richText.RepliedMsg == nil {
		return content
	}
	quoted := extractQuotedText(richText.RepliedMsg)
	if quoted == "" {
		return content
	}
	return fmt.Sprintf("引用: \"%s\"\n\n%s", quoted, content)
}

// extractQuotedText 只解被引的文本消息；交互卡片等其它类型不展开（纯文本适配器不发卡片）。
func extractQuotedText(msg *repliedMessage) string {
	if msg == nil || msg.MsgType != "text" {
		return ""
	}
	var rc repliedTextContent
	if err := json.Unmarshal(msg.Content, &rc); err != nil {
		return ""
	}
	return normalizeQuotedMessageText(rc.Text)
}

func normalizeQuotedMessageText(s string) string {
	text := strings.TrimSpace(s)
	runes := []rune(text)
	if len(runes) <= maxQuotedMessageRunes {
		return text
	}
	return string(runes[:maxQuotedMessageRunes]) + "..."
}

func (p *Platform) isDuplicate(id string) bool {
	if id == "" {
		return false
	}
	now := time.Now()
	p.dedupMu.Lock()
	defer p.dedupMu.Unlock()
	for k, t := range p.dedup {
		if now.Sub(t) > dedupTTL {
			delete(p.dedup, k)
		}
	}
	if _, seen := p.dedup[id]; seen {
		return true
	}
	p.dedup[id] = now
	return false
}

// ---------------------------------------------------------------------------
// 回消息
// ---------------------------------------------------------------------------

func (p *Platform) Reply(ctx context.Context, replyCtx any, content string) error {
	rc, ok := p.parseReplyContext(replyCtx)
	if !ok {
		return fmt.Errorf("dingtalk: invalid reply context %T", replyCtx)
	}
	if rc.proactive || rc.sessionWebhook == "" {
		return p.sendProactiveMessage(ctx, rc, content)
	}

	atUserIDs := extractAtUserIds(content)
	md := preprocessDingTalkMarkdown(content)
	body := map[string]any{
		"msgtype":  "markdown",
		"markdown": map[string]string{"title": "reply", "text": md},
	}
	if len(atUserIDs) > 0 {
		body["at"] = map[string]any{"atUserIds": atUserIDs, "isAtAll": false}
	}
	respBody, status, err := p.postJSON(ctx, rc.sessionWebhook, nil, body)
	if err != nil {
		return fmt.Errorf("dingtalk: send reply: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("dingtalk: reply http %d: %s", status, string(respBody))
	}
	return nil
}

// Send 发新消息。有 sessionWebhook 时等价 Reply；否则（主动推送）走群/单聊 API。
func (p *Platform) Send(ctx context.Context, replyCtx any, content string) error {
	rc, ok := p.parseReplyContext(replyCtx)
	if !ok {
		return fmt.Errorf("dingtalk: invalid reply context %T", replyCtx)
	}
	if rc.proactive || rc.sessionWebhook == "" {
		return p.sendProactiveMessage(ctx, rc, content)
	}
	return p.Reply(ctx, rc, content)
}

// sendProactiveMessage 用钉钉群/单聊消息 API 主动推送（不依赖临时 sessionWebhook），
// 支撑卡片推送 / AskUserQuestion 等无最近来信时的主动发送。
func (p *Platform) sendProactiveMessage(ctx context.Context, rc *replyContext, content string) error {
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("dingtalk: get access token: %w", err)
	}
	md := preprocessDingTalkMarkdown(content)

	var apiURL string
	var body map[string]any
	switch {
	case rc.isGroup && rc.conversationID != "":
		apiURL = groupMsgURL
		msgParam, _ := json.Marshal(map[string]string{"text": md})
		body = map[string]any{
			"robotCode":          p.robotCode,
			"openConversationId": rc.conversationID,
			"msgKey":             "sampleMarkdown",
			"msgParam":           string(msgParam),
		}
	case rc.senderStaffID != "":
		apiURL = directMsgURL
		msgParam, _ := json.Marshal(map[string]string{"title": "reply", "text": md})
		body = map[string]any{
			"robotCode": p.robotCode,
			"userIds":   []string{rc.senderStaffID},
			"msgKey":    "sampleMarkdown",
			"msgParam":  string(msgParam),
		}
	default:
		return fmt.Errorf("dingtalk: proactive send needs conversationId (group) or senderStaffId (direct)")
	}

	respBody, status, err := p.postJSON(ctx, apiURL, map[string]string{"x-acs-dingtalk-access-token": token}, body)
	if err != nil {
		return fmt.Errorf("dingtalk: proactive send: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("dingtalk: proactive send http %d: %s", status, string(respBody))
	}
	return nil
}

// getAccessToken 取（并缓存）企业内部应用 access token。
func (p *Platform) getAccessToken(ctx context.Context) (string, error) {
	p.tokenMu.Lock()
	defer p.tokenMu.Unlock()
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.accessToken, nil
	}

	respBody, status, err := p.postJSON(ctx, tokenURL, nil, map[string]string{
		"appKey":    p.clientID,
		"appSecret": p.clientSecret,
	})
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("dingtalk: token http %d: %s", status, string(respBody))
	}
	var out struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int    `json:"expireIn"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("dingtalk: decode token: %w", err)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("dingtalk: empty accessToken")
	}
	p.accessToken = out.AccessToken
	// 服务端漏传 expireIn 时按文档默认 7200s，避免每次调用都刷新 token。
	expiry := out.ExpireIn
	if expiry <= 0 {
		expiry = 7200
	}
	if expiry > tokenExpiryBuffer {
		expiry -= tokenExpiryBuffer
	}
	p.tokenExpiry = time.Now().Add(time.Duration(expiry) * time.Second)
	return p.accessToken, nil
}

func (p *Platform) postJSON(ctx context.Context, url string, headers map[string]string, body any) ([]byte, int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("dingtalk: marshal request: %w", err)
	}
	req, cancel, err := httpx.NewRequest(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	defer cancel()
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpx.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("dingtalk: read body: %w", err)
	}
	return respBody, resp.StatusCode, nil
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

// reconstructReplyCtx 从 scopeKey "dingtalk:<convType>:<conversationId>:<senderStaffId>"
// 还原主动推送上下文。convType 为 "g"（群）/ "d"（单聊）。
func reconstructReplyCtx(scope string) (*replyContext, bool) {
	if !strings.HasPrefix(scope, "dingtalk:") {
		return nil, false
	}
	parts := strings.SplitN(strings.TrimPrefix(scope, "dingtalk:"), ":", 3)
	if len(parts) < 2 {
		return nil, false
	}
	convType := parts[0]
	if convType != "g" && convType != "d" {
		return nil, false
	}
	conversationID := parts[1]
	if conversationID == "" {
		return nil, false
	}
	var senderStaffID string
	if len(parts) > 2 {
		senderStaffID = parts[2]
	}
	return &replyContext{
		conversationID: conversationID,
		senderStaffID:  senderStaffID,
		isGroup:        convType == "g",
		proactive:      true,
	}, true
}

func (p *Platform) reportConnection(connected bool, err error) {
	// 已停实例的迟到回调（connectLoop 尾声 / SDK 断线）不得覆盖新平台实例的连接状态。
	if p.stopped.Load() {
		return
	}
	p.connMu.Lock()
	h := p.connHandler
	p.connMu.Unlock()
	if h != nil {
		h(connected, err)
	}
}

var atUserIDRegexp = regexp.MustCompile(`@(\d{4,})`)

// extractAtUserIds 从正文里抽 @<数字ID>，填入钉钉 text/markdown 的 atUserIds。
func extractAtUserIds(content string) []string {
	matches := atUserIDRegexp.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	var ids []string
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}
	return ids
}

// preprocessDingTalkMarkdown 适配钉钉 markdown 渲染器：
//   - 行首空格 → 不换行空格（防止被 markdown 吃掉缩进）
//   - 非空行之间的单个 \n → 行尾补两空格强制换行
//   - 代码块内部不动
func preprocessDingTalkMarkdown(s string) string {
	lines := strings.Split(s, "\n")
	inCodeBlock := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		}
		if inCodeBlock {
			continue
		}
		spaceCount := len(line) - len(strings.TrimLeft(line, " "))
		if spaceCount > 0 {
			lines[i] = strings.Repeat(" ", spaceCount) + line[spaceCount:]
		}
	}

	var sb strings.Builder
	for i, line := range lines {
		sb.WriteString(line)
		if i < len(lines)-1 {
			if line != "" && lines[i+1] != "" {
				sb.WriteString("  \n")
			} else {
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}
