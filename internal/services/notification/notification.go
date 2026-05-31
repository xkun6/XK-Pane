// Package notification 是桌面通知服务（移植 cc-pane notification_service.rs 的显式触发部分）。
//
// 触发链：cli-hook notify → orchestrator POST /api/notifications/trigger → 本服务 Trigger →
// Wails v3 notifications.NotificationService.SendNotification（Windows toast / mac / linux）。
//
// 规则（对齐 cc-pane）：读 app_settings 的 NotificationEnabled / NotificationOnlyWhenUnfocused
//
//	→ enabled=false 跳过 → only_when_unfocused 且窗口聚焦跳过 → dedupeKey 10s 去重 → 发送。
//
// 设置落点（D6d-4）：app_settings 两字段；不进前端 Snapshot（无设置 UI），本服务直接读表（holder，
// 不 capture *DB，遵铁律）。窗口聚焦经注入的 focusFn 回调解耦 Wails 窗口 API。
package notification

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"xkpane/internal/storage"
	"xkpane/internal/utils/logx"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// dedupeWindow 与 cc-pane 一致：同 dedupeKey 10s 内只发一次。
const dedupeWindow = 10 * time.Second

// Request 是一次通知触发输入（与 cli-hook notifyRequest / cc-pane NotificationRequest 对齐）。
type Request struct {
	Kind              string          `json:"kind"`
	Title             string          `json:"title"`
	Body              string          `json:"body"`
	Source            string          `json:"source"`
	Scope             string          `json:"scope"`
	DedupeKey         string          `json:"dedupeKey"`
	OnlyWhenUnfocused *bool           `json:"onlyWhenUnfocused"`
	Metadata          json.RawMessage `json:"metadata"`
}

// Result 是触发结果（sent / skipped + 原因）。
type Result struct {
	Sent    bool   `json:"sent"`
	Skipped bool   `json:"skipped"`
	Reason  string `json:"reason,omitempty"`
}

// sender 是发送桌面通知的窄接口（*notifications.NotificationService 满足；测试可注入 fake）。
type sender interface {
	SendNotification(notifications.NotificationOptions) error
}

// Service 管理桌面通知触发与去重。
type Service struct {
	holder   *storage.Holder
	notifier sender

	mu     sync.Mutex
	recent map[string]time.Time

	focusFn func() bool      // 窗口是否聚焦；nil = 视为未聚焦（保守地发送）
	nowFn   func() time.Time // 便于测试
}

// New 构造通知服务。notifier 为 Wails 通知 service 单例（同时须注册进 application.Services 以跑 Startup）。
func New(holder *storage.Holder, notifier *notifications.NotificationService) *Service {
	return &Service{
		holder:   holder,
		notifier: notifier,
		recent:   make(map[string]time.Time),
		nowFn:    time.Now,
	}
}

// SetFocusCheck 注入「主窗口是否聚焦」回调（app 装配阶段，捕获窗口引用）。
func (s *Service) SetFocusCheck(fn func() bool) { s.focusFn = fn }

// Trigger 按规则发送桌面通知。返回 Result（sent/skipped），error 仅用于参数非法 / 发送失败。
func (s *Service) Trigger(req Request) (Result, error) {
	kind := strings.TrimSpace(req.Kind)
	title := strings.TrimSpace(req.Title)
	body := strings.TrimSpace(req.Body)
	if kind == "" {
		return Result{}, fmt.Errorf("notification kind cannot be empty")
	}
	if title == "" && body == "" {
		return Result{}, fmt.Errorf("notification title or body is required")
	}
	if title == "" {
		title = kind
	}

	enabled, settingOnlyUnfocused := s.readSettings()
	if !enabled {
		return Result{Skipped: true, Reason: "notifications_disabled"}, nil
	}

	onlyWhenUnfocused := settingOnlyUnfocused
	if req.OnlyWhenUnfocused != nil {
		onlyWhenUnfocused = *req.OnlyWhenUnfocused
	}
	if onlyWhenUnfocused && s.isFocused() {
		return Result{Skipped: true, Reason: "window_focused"}, nil
	}

	if req.DedupeKey != "" && !s.checkDedupe(req.DedupeKey) {
		return Result{Skipped: true, Reason: "deduped"}, nil
	}

	logx.For("notification").Info("trigger",
		"kind", kind, "title", title,
		"source", req.Source, "scope", req.Scope)

	opts := notifications.NotificationOptions{
		ID:    s.notificationID(req.DedupeKey),
		Title: title,
		Body:  body,
	}
	if err := s.notifier.SendNotification(opts); err != nil {
		return Result{}, fmt.Errorf("send desktop notification: %w", err)
	}
	return Result{Sent: true}, nil
}

// readSettings 读 app_settings 的通知开关（holder.Current() 取活跃库，不 capture *DB）。
// 读失败保守地按 enabled=true / onlyWhenUnfocused=false（不因读设置失败而吞掉通知）。
func (s *Service) readSettings() (enabled, onlyWhenUnfocused bool) {
	var row storage.AppSettings
	if err := s.holder.Current().GORM.First(&row, "id = ?", 1).Error; err != nil {
		return true, false
	}
	return row.NotificationEnabled, row.NotificationOnlyWhenUnfocused
}

func (s *Service) isFocused() bool {
	if s.focusFn == nil {
		return false
	}
	return s.focusFn()
}

// checkDedupe 返回 true 表示可发送（窗口内首次）；false 表示命中去重。
func (s *Service) checkDedupe(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowFn()
	if last, ok := s.recent[key]; ok && now.Sub(last) < dedupeWindow {
		return false
	}
	s.recent[key] = now
	return true
}

// notificationID 取 dedupeKey 作通知 ID；为空时生成随机 hex（Wails 要求 ID 非空）。
func (s *Service) notificationID(dedupeKey string) string {
	if dedupeKey != "" {
		return dedupeKey
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "xkpane-notify"
	}
	return "xkpane-" + hex.EncodeToString(b)
}
