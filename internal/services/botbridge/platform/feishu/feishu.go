package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"xkpane/internal/services/botbridge"
	"xkpane/internal/utils/logx"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

func init() {
	botbridge.RegisterPlatform("feishu", New)
}

type replyContext struct {
	messageID string
	chatID    string
	scopeKey  string
}

type Platform struct {
	mu        sync.RWMutex
	appID     string
	appSecret string
	client    *lark.Client
	wsClient  *larkws.Client
	cancel    context.CancelFunc
	handler   botbridge.MessageHandler
	conn      botbridge.ConnectionStatusHandler
}

func New(opts map[string]any) (botbridge.Platform, error) {
	appID, _ := opts["app_id"].(string)
	appSecret, _ := opts["app_secret"].(string)
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(appSecret) == "" {
		return nil, fmt.Errorf("feishu: app_id and app_secret are required")
	}
	return &Platform{
		appID:     appID,
		appSecret: appSecret,
		client:    lark.NewClient(appID, appSecret),
	}, nil
}

func (p *Platform) Name() string { return "feishu" }

func (p *Platform) SetConnectionHandler(handler botbridge.ConnectionStatusHandler) {
	p.mu.Lock()
	p.conn = handler
	p.mu.Unlock()
}

func (p *Platform) Start(handler botbridge.MessageHandler) error {
	p.mu.Lock()
	p.handler = handler
	p.mu.Unlock()

	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			p.onMessage(event)
			return nil
		}).
		OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
			return p.onCardAction(event)
		})

	wsClient := larkws.NewClient(p.appID, p.appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
		larkws.WithOnReady(func() {
			p.reportConnection(true, nil)
		}),
		larkws.WithOnReconnected(func() {
			p.reportConnection(true, nil)
		}),
		larkws.WithOnError(func(err error) {
			p.reportConnection(false, err)
		}),
		larkws.WithOnDisconnected(func() {
			p.reportConnection(false, nil)
		}),
	)
	ctx, cancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.wsClient = wsClient
	p.cancel = cancel
	p.mu.Unlock()
	go func() {
		if err := wsClient.Start(ctx); err != nil && ctx.Err() == nil {
			logx.For("botbridge.feishu").Warn("websocket stopped", "err", err)
			p.reportConnection(false, err)
		}
	}()
	return nil
}

func (p *Platform) Stop() error {
	p.mu.Lock()
	cancel := p.cancel
	p.cancel = nil
	p.wsClient = nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (p *Platform) Reply(ctx context.Context, rctx any, content string) error {
	rc, ok := parseReplyContext(rctx)
	if !ok {
		return fmt.Errorf("feishu: invalid reply context %T", rctx)
	}
	msgType, body := buildTextContent(content)
	if rc.messageID != "" {
		req := larkim.NewReplyMessageReqBuilder().
			MessageId(rc.messageID).
			Body(larkim.NewReplyMessageReqBodyBuilder().MsgType(msgType).Content(body).Build()).
			Build()
		resp, err := p.client.Im.Message.Reply(ctx, req)
		if err != nil {
			return err
		}
		if !resp.Success() {
			return fmt.Errorf("feishu reply failed code=%d msg=%s", resp.Code, resp.Msg)
		}
		return nil
	}
	return p.createMessage(ctx, rc.chatID, msgType, body)
}

func (p *Platform) Send(ctx context.Context, rctx any, content string) error {
	rc, ok := parseReplyContext(rctx)
	if !ok {
		return fmt.Errorf("feishu: invalid send context %T", rctx)
	}
	msgType, body := buildTextContent(content)
	return p.createMessage(ctx, rc.chatID, msgType, body)
}

func (p *Platform) SendCard(ctx context.Context, rctx any, card *botbridge.Card) error {
	rc, ok := parseReplyContext(rctx)
	if !ok {
		return fmt.Errorf("feishu: invalid card context %T", rctx)
	}
	return p.createMessage(ctx, rc.chatID, larkim.MsgTypeInteractive, renderCard(card, rc.scopeKey))
}

func (p *Platform) ReplyCard(ctx context.Context, rctx any, card *botbridge.Card) error {
	rc, ok := parseReplyContext(rctx)
	if !ok {
		return fmt.Errorf("feishu: invalid card context %T", rctx)
	}
	if rc.messageID == "" {
		return p.SendCard(ctx, rctx, card)
	}
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(rc.messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeInteractive).
			Content(renderCard(card, rc.scopeKey)).
			Build()).
		Build()
	resp, err := p.client.Im.Message.Reply(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("feishu reply card failed code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (p *Platform) createMessage(ctx context.Context, chatID, msgType, content string) error {
	if chatID == "" {
		return fmt.Errorf("feishu: empty chat id")
	}
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(msgType).
			Content(content).
			Build()).
		Build()
	resp, err := p.client.Im.Message.Create(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("feishu create message failed code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (p *Platform) onMessage(event *larkim.P2MessageReceiveV1) {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return
	}
	msg := event.Event.Message
	chatID := stringValue(msg.ChatId)
	userID := ""
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
		userID = stringValue(event.Event.Sender.SenderId.OpenId)
	}
	messageID := stringValue(msg.MessageId)
	content := extractText(msg)
	if strings.TrimSpace(content) == "" {
		return
	}
	scope := scopeKey(chatID, userID)
	p.emit(&botbridge.Message{
		MessageID: messageID,
		ScopeKey:  scope,
		Platform:  "feishu",
		UserID:    userID,
		Content:   content,
		ReplyCtx:  replyContext{messageID: messageID, chatID: chatID, scopeKey: scope},
	})
}

func (p *Platform) onCardAction(event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	if event == nil || event.Event == nil || event.Event.Action == nil {
		return nil, nil
	}
	value := event.Event.Action.Value
	action, _ := value["action"].(string)
	if action == "" && event.Event.Action.Option != "" {
		action = event.Event.Action.Option
	}
	if strings.TrimSpace(action) == "" {
		return nil, nil
	}
	userID := ""
	if event.Event.Operator != nil {
		userID = event.Event.Operator.OpenID
	}
	chatID := ""
	messageID := ""
	if event.Event.Context != nil {
		chatID = event.Event.Context.OpenChatID
		messageID = event.Event.Context.OpenMessageID
	}
	if event.EventV2Base != nil && event.EventV2Base.Header != nil && event.EventV2Base.Header.EventID != "" {
		messageID = event.EventV2Base.Header.EventID
	} else if messageID != "" {
		messageID = messageID + ":" + action
	}
	if chatID == "" {
		chatID = userID
	}
	scope, _ := value["scope_key"].(string)
	if scope == "" {
		scope = scopeKey(chatID, userID)
	}
	p.emit(&botbridge.Message{
		MessageID: messageID,
		ScopeKey:  scope,
		Platform:  "feishu",
		UserID:    userID,
		Content:   action,
		ReplyCtx:  replyContext{messageID: messageID, chatID: chatID, scopeKey: scope},
	})
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{Type: "info", Content: "已记录"},
	}, nil
}

func (p *Platform) reportConnection(connected bool, err error) {
	p.mu.RLock()
	h := p.conn
	p.mu.RUnlock()
	if h != nil {
		h(connected, err)
	}
}

func (p *Platform) emit(msg *botbridge.Message) {
	p.mu.RLock()
	h := p.handler
	p.mu.RUnlock()
	if h != nil {
		go h(p, msg)
	}
}

func parseReplyContext(v any) (replyContext, bool) {
	switch x := v.(type) {
	case replyContext:
		return x, true
	case string:
		chatID := chatIDFromScope(x)
		if chatID == "" {
			chatID = x
		}
		return replyContext{chatID: chatID, scopeKey: x}, true
	case nil:
		return replyContext{}, false
	default:
		return replyContext{}, false
	}
}

func scopeKey(chatID, userID string) string {
	if chatID == "" {
		chatID = userID
	}
	return "feishu:" + chatID + ":" + userID
}

func chatIDFromScope(scope string) string {
	parts := strings.Split(scope, ":")
	if len(parts) >= 2 && parts[0] == "feishu" {
		return parts[1]
	}
	return ""
}

func buildTextContent(content string) (string, string) {
	b, _ := json.Marshal(map[string]string{"text": content})
	return larkim.MsgTypeText, string(b)
}

func extractText(msg *larkim.EventMessage) string {
	if msg == nil || msg.Content == nil {
		return ""
	}
	var body map[string]any
	if json.Unmarshal([]byte(*msg.Content), &body) != nil {
		return ""
	}
	if text, _ := body["text"].(string); text != "" {
		return strings.TrimSpace(text)
	}
	if title, _ := body["title"].(string); title != "" {
		return strings.TrimSpace(title)
	}
	return ""
}

func stringValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
