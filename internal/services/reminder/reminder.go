// Package reminder 周期性扫描到期的待办提醒并发桌面通知。
//
// 移植 cc-pane web/hooks/useTodoReminders 的「轮询 + 去重」语义，但**落在后端**而非前端：
//   - 前端 setInterval 在窗口最小化/后台时会被浏览器节流（恰是最需要提醒的时刻），后端 ticker 不受影响；
//   - 复用既有 internal/services/notification（读 app_settings 开关 + 聚焦判定 + 10s 去重 + 原生 toast），
//     与 cli-hook → orchestrator → Trigger 的 XK 既有通知架构一致；
//   - 到期判定走 todo.Service.CheckReminders（reminder_at <= now AND status != 'done'，纯查询不回写）。
//
// 数值对齐 cc-pane：初始延迟 5s、轮询 60s、同一待办 10min 重提醒窗口（notifiedRef 等价物落后端）。
package reminder

import (
	"context"
	"strings"
	"sync"
	"time"

	"xkpane/internal/services/notification"
	"xkpane/internal/services/todo"
	"xkpane/internal/utils/logx"
)

const (
	pollInterval   = 60 * time.Second
	initialDelay   = 5 * time.Second
	renotifyWindow = 10 * time.Minute
)

// dueLister 取到期提醒待办（*todo.Service 满足）。
type dueLister interface {
	CheckReminders(ctx context.Context) ([]todo.TodoDTO, error)
}

// notifier 发桌面通知（*notification.Service 满足）。
type notifier interface {
	Trigger(notification.Request) (notification.Result, error)
}

// Dispatcher 周期性扫描到期提醒并发桌面通知，同一待办在 renotifyWindow 内只提醒一次。
type Dispatcher struct {
	lister   dueLister
	notifier notifier
	localeFn func() string // 返回当前 locale（'zh-CN'/'en-US'/'auto'），决定通知标题语言；nil 视为 zh

	mu       sync.Mutex
	notified map[string]time.Time // todoID → 上次「已发送」通知时刻
	nowFn    func() time.Time     // 便于测试
}

// New 构造提醒分发器。lister 取到期待办，notifier 发通知，localeFn 提供通知文案语言。
func New(lister dueLister, notifier notifier, localeFn func() string) *Dispatcher {
	return &Dispatcher{
		lister:   lister,
		notifier: notifier,
		localeFn: localeFn,
		notified: make(map[string]time.Time),
		nowFn:    time.Now,
	}
}

// Start 启动后台轮询 goroutine：初始延迟 5s，之后每 60s 一轮（对齐 cc-pane）。ctx 取消即停。
func (d *Dispatcher) Start(ctx context.Context) {
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(initialDelay):
		}
		d.runOnce(ctx)
		t := time.NewTicker(pollInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				d.runOnce(ctx)
			}
		}
	}()
}

// runOnce 扫一轮到期提醒：对每条未在重提醒窗口内的待办发通知。查询错误仅 warn，不中断轮询。
// 只有【真正发送】（Result.Sent）才记入去重窗口——通知被禁用/窗口聚焦跳过时不消耗窗口，
// 下一轮条件变化（启用/失焦）即可补发（比 cc-pane 无条件标记更准确）。
func (d *Dispatcher) runOnce(ctx context.Context) {
	due, err := d.lister.CheckReminders(ctx)
	if err != nil {
		logx.For("reminder").Warn("check reminders failed", "err", err)
		return
	}
	d.prune()
	title := reminderLabel(d.locale())
	for _, td := range due {
		if d.recentlyNotified(td.ID) {
			continue
		}
		res, err := d.notifier.Trigger(notification.Request{
			Kind:      "todo_reminder",
			Title:     title,
			Body:      td.Title,
			Source:    "todo",
			DedupeKey: "todo-reminder-" + td.ID,
		})
		if err != nil {
			logx.For("reminder").Warn("trigger notification failed", "todo", td.ID, "err", err)
			continue
		}
		if res.Sent {
			d.record(td.ID)
		}
	}
}

// recentlyNotified 报告该待办是否在重提醒窗口内已发送过。
func (d *Dispatcher) recentlyNotified(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	last, ok := d.notified[id]
	return ok && d.nowFn().Sub(last) < renotifyWindow
}

// record 记录该待办本次发送时刻。
func (d *Dispatcher) record(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.notified[id] = d.nowFn()
}

// prune 清掉超过窗口的旧记录，避免 map 无界增长。
func (d *Dispatcher) prune() {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.nowFn()
	for k, ts := range d.notified {
		if now.Sub(ts) >= renotifyWindow {
			delete(d.notified, k)
		}
	}
}

func (d *Dispatcher) locale() string {
	if d.localeFn == nil {
		return ""
	}
	return d.localeFn()
}

// reminderLabel 桌面通知标题（待办标题进 body）。后端无 i18n 系统，按 locale 取两种语言之一，
// 'auto' / 'zh-*' / 未知 → 中文（应用默认语言）。
func reminderLabel(locale string) string {
	if strings.HasPrefix(locale, "en") {
		return "Todo Reminder"
	}
	return "待办提醒"
}
