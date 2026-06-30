package botbridge

import (
	"context"
	"fmt"
)

// Platform 是 IM 平台适配器最小契约，裁剪自 cc-connect 的右半边平台层。
type Platform interface {
	Name() string
	Start(handler MessageHandler) error
	Reply(ctx context.Context, replyCtx any, content string) error
	Send(ctx context.Context, replyCtx any, content string) error
	Stop() error
}

type MessageHandler func(p Platform, msg *Message)

type ConnectionStatusHandler func(connected bool, err error)

type ConnectionReporter interface {
	SetConnectionHandler(ConnectionStatusHandler)
}

type Message struct {
	MessageID string
	ScopeKey  string
	Platform  string
	UserID    string
	Content   string
	ReplyCtx  any
}

type ButtonOption struct {
	Text string
	Data string
}

type CardSender interface {
	SendCard(ctx context.Context, replyCtx any, card *Card) error
	ReplyCard(ctx context.Context, replyCtx any, card *Card) error
}

type InlineButtonSender interface {
	SendWithButtons(ctx context.Context, replyCtx any, content string, buttons [][]ButtonOption) error
}

type MessageUpdater interface {
	UpdateMessage(ctx context.Context, replyCtx any, content string) error
}

type PlatformFactory func(opts map[string]any) (Platform, error)

var platformFactories = map[string]PlatformFactory{}

func RegisterPlatform(name string, factory PlatformFactory) {
	platformFactories[name] = factory
}

func CreatePlatform(name string, opts map[string]any) (Platform, error) {
	f, ok := platformFactories[name]
	if !ok {
		return nil, fmt.Errorf("unknown bot platform %q", name)
	}
	return f(opts)
}
