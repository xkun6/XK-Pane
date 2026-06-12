// Package screenshotsvc 实现「截图快捷键」（参照 cc-pane trigger_screenshot 全链路）：
// 系统级全局热键 → 主窗口排除捕获（SetContentProtection，底层 WDA_EXCLUDEFROMCAPTURE）→
// 截光标所在显示器 → Win32 GDI 选区遮罩（拖拽选择 / 松开确认 / ESC 取消）→ 裁剪存 PNG →
// 路径写剪贴板 + Emit screenshot:saved（前端把路径写进活跃终端，闭环"粘贴进 Claude Code"）。
//
// 仅 Windows 实现热键与截屏（对齐 cc-pane #[cfg(target_os = "windows")]）；其余平台 Start
// 为 no-op（fail-open，不阻断应用启动）。
package screenshotsvc

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"xkpane/internal/storage"
	"xkpane/internal/utils/logx"

	"gorm.io/gorm"
)

// preferences 键（前端 PreferencesService 同名读写，Value 为 JSON 字符串）。
const (
	prefKeyShortcut      = "screenshot.shortcut"
	prefKeyRetentionDays = "screenshot.retentionDays"

	// DefaultShortcut 故意避开 cc-pane 的 Ctrl+Shift+S（release）/ Ctrl+Alt+Shift+S（dev）：
	// 两 app 同机并行时 RegisterHotKey 同键互斥，后注册方静默失效。
	DefaultShortcut      = "Ctrl+Alt+S"
	defaultRetentionDays = 7
)

// Service 是截图快捷键的 Wails service。
// 持有 *storage.Holder 而非 *storage.DB（切换数据路径会替换句柄）。
type Service struct {
	holder *storage.Holder

	emit    func(name string, payload any) // 装配处注入（mirrorEmit）
	protect func(on bool)                  // 主窗口 SetContentProtection（截图期间排除自身）

	capturing atomic.Bool // 防重入（对齐 cc-pane CAPTURING）

	// mu 守护 hk 的全部读写：Start（主 goroutine）/ Stop（退出 defer）/ UpdateShortcut
	// （Wails binding，每次调用独立 goroutine）三方并发，check-then-act 须在同一临界区。
	mu sync.Mutex
	hk *hotkeyLoop // Windows 热键消息循环；注册失败 / 非 Windows 时为 nil
}

func New(holder *storage.Holder) *Service {
	return &Service{holder: holder}
}

// SetEmitter 注入事件发射函数（装配处的 mirrorEmit）。
func (s *Service) SetEmitter(fn func(name string, payload any)) {
	s.emit = fn
}

// SetWindowHooks 注入主窗口捕获排除钩子（解耦 service 与 Wails 窗口对象）。
// 剪贴板不走 Wails（仅文本 API）：截图以 CF_HDROP 文件对象自接 Win32 写入。
func (s *Service) SetWindowHooks(protect func(on bool)) {
	s.protect = protect
}

// Start 读取持久化的快捷键并注册全局热键。注册失败（键位冲突等）返回 error，
// 调用方 fail-open（用户仍可在设置页改键后经 UpdateShortcut 重试）。
func (s *Service) Start() error {
	return s.platformStart(s.shortcutPref())
}

// Stop 注销热键并退出消息循环。
func (s *Service) Stop() {
	s.platformStop()
}

// UpdateShortcut 运行时改键：注销旧热键、注册新热键（对齐 cc-pane screenshot_update_shortcut）。
// 仅负责注册，不持久化——前端在成功后经 PreferencesService 落库。
func (s *Service) UpdateShortcut(_ context.Context, shortcut string) error {
	mods, vk, err := parseShortcut(shortcut)
	if err != nil {
		return err
	}
	return s.platformReregister(mods, vk)
}

// shortcutPref 读取持久化键位；缺失 / 解析失败回默认值。
func (s *Service) shortcutPref() string {
	return s.prefString(prefKeyShortcut, DefaultShortcut)
}

// retentionDays 读取持久化保留天数；0 = 永久保留。
func (s *Service) retentionDays() int {
	return s.prefInt(prefKeyRetentionDays, defaultRetentionDays)
}

func (s *Service) prefString(key, def string) string {
	var p storage.Preference
	if err := s.holder.Current().GORM.First(&p, "key = ?", key).Error; err != nil {
		return def
	}
	var v string
	if err := json.Unmarshal([]byte(p.Value), &v); err != nil || strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func (s *Service) prefInt(key string, def int) int {
	var p storage.Preference
	if err := s.holder.Current().GORM.First(&p, "key = ?", key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return def
		}
		// 读库异常（如切换数据路径的瞬断窗口）：fail-safe 返回 0（= 永久保留，跳过清理），
		// 绝不能拿默认 7 天去删用户可能设了"永久保留"的旧截图。
		return 0
	}
	var v float64
	if err := json.Unmarshal([]byte(p.Value), &v); err != nil || v < 0 {
		return def
	}
	return int(v)
}

func logFor() *slog.Logger { return logx.For("screenshot") }
