// Package hudstats 复用 claude-hud 落盘的产物，给「结构化时间线」卡片底部状态行提供
// 上下文用量 / 5h / 周限三项指标（与 claude-hud statusline 显示的数字完全一致）。
//
// 为什么读 claude-hud 的产物而非自算：这三项数据在磁盘别处都不存在 ——
//   - 5h / 周限：账号级限额窗口，只活在 Claude Code 每次渲染 statusLine 时实时喂给插件的
//     stdin（rate_limits.{five_hour,seven_day}）里，不落转录、不落任何持久化文件。
//   - 上下文窗口大小（200k / 1M）：同样只在实时 stdin 的 context_window.context_window_size 里，
//     转录的 message.usage 拿不到，无法可靠区分 200k 与 1M。
// 用户已全局启用 claude-hud 作为 statusLine（每个 claude 会话都会跑它）。claude-hud 把两类数据落盘：
//   1. 每会话 context 快照：<pluginDir>/context-cache/<sha256(transcriptPath)>.json
//      （含 used_percentage / context_window_size / current_usage 分项 token）。
//   2. 账号级 5h/周限 快照：<pluginDir>/external-usage.json（需在 claude-hud 配置开启
//      display.externalUsageWritePath；含 five_hour/seven_day 的 used_percentage + ISO resets_at）。
// 故 XK 直接读这两份文件即可，无需自建 statusline 探针、不改 claude 启动方式、镜像终端里的
// claude-hud 不受影响。<pluginDir> = <CLAUDE_CONFIG_DIR 或 ~/.claude>/plugins/claude-hud（复刻
// claude-hud getHudPluginDir）；context 缓存键 = sha256(转录文件原生绝对路径)（复刻其 getCachePath
// 的 sha256(path.resolve(transcript_path))，Windows 下即 filepath.Join 产出的反斜杠路径，已实测对齐）。
//
// 纯只读、best-effort：任何文件缺失 / 解析失败 / 过期都静默返回缺省，卡片优雅降级、不报错。
package hudstats

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"xkpane/internal/utils/filex"
)

// 文件读取上限：两类快照都很小（各约数百字节）。
const readCap = 1 << 20

// usageStaleAfter：external-usage 快照超过此时长未刷新视为过期（数字必失真，整体丢弃）。
// claude-hud 仅在有 claude 会话渲染 statusLine 时刷新（节流 30s），全部会话关闭后会停更。
const usageStaleAfter = 6 * time.Hour

// contextStaleAfter：context-cache 超过此时长未刷新视为过期（claude-hud 自身保留上限 7 天）。
const contextStaleAfter = 24 * time.Hour

// Metrics 是卡片状态行的三项指标。任一为 nil 表示该项暂不可用（前端不渲染对应文字）。
type Metrics struct {
	Context  *ContextMetric `json:"context,omitempty"`
	FiveHour *UsageWindow   `json:"fiveHour,omitempty"`
	SevenDay *UsageWindow   `json:"sevenDay,omitempty"`
}

// ContextMetric 是当前会话的上下文用量。UsedTokens/WindowSize 渲染成 "216k/1.0M"，UsedPercentage 取色。
type ContextMetric struct {
	UsedPercentage int `json:"usedPercentage"` // 原生 used_percentage（取色阈值用）
	UsedTokens     int `json:"usedTokens"`     // input + cache_creation + cache_read（对齐 claude-hud getTotalTokens，不含 output）
	WindowSize     int `json:"windowSize"`     // context_window_size（200000 / 1000000）
}

// UsageWindow 是一个限额窗口（5h 或 周）。ResetsAtMs=0 表示未知/已过 → 前端不显示剩余时间。
type UsageWindow struct {
	UsedPercentage int   `json:"usedPercentage"`
	ResetsAtMs     int64 `json:"resetsAtMs,omitempty"` // 重置时刻 unix ms
}

// Collect 汇总指定 Claude 转录文件对应的状态行指标：context 来自该会话的 context-cache（按转录文件
// 原生绝对路径 sha256 定位，与 claude-hud 一致），5h/周限来自账号级 external-usage 快照。三项全缺返回 nil。
func Collect(transcriptPath string) *Metrics {
	m := &Metrics{}
	m.Context = readContext(transcriptPath)
	m.FiveHour, m.SevenDay = readUsage()
	if m.Context == nil && m.FiveHour == nil && m.SevenDay == nil {
		return nil
	}
	return m
}

// pluginDir 复刻 claude-hud getHudPluginDir：<CLAUDE_CONFIG_DIR 或 ~/.claude>/plugins/claude-hud。
func pluginDir() (string, bool) {
	base := os.Getenv("CLAUDE_CONFIG_DIR")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		base = filepath.Join(home, ".claude")
	}
	return filepath.Join(base, "plugins", "claude-hud"), true
}

// readContext 按转录文件路径定位并解析 claude-hud 的每会话 context 快照。
func readContext(transcriptPath string) *ContextMetric {
	if transcriptPath == "" {
		return nil
	}
	dir, ok := pluginDir()
	if !ok {
		return nil
	}
	sum := sha256.Sum256([]byte(transcriptPath))
	path := filepath.Join(dir, "context-cache", hex.EncodeToString(sum[:])+".json")
	data, err := filex.ReadLimit(path, readCap)
	if err != nil {
		return nil
	}
	var raw struct {
		UsedPercentage    *float64 `json:"used_percentage"`
		ContextWindowSize *int     `json:"context_window_size"`
		CurrentUsage      *struct {
			InputTokens              int `json:"input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"current_usage"`
		SavedAt int64 `json:"saved_at"` // unix ms
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	if raw.UsedPercentage == nil || raw.ContextWindowSize == nil || *raw.ContextWindowSize <= 0 {
		return nil
	}
	if raw.SavedAt > 0 && time.Since(time.UnixMilli(raw.SavedAt)) > contextStaleAfter {
		return nil
	}
	used := 0
	if raw.CurrentUsage != nil {
		used = raw.CurrentUsage.InputTokens + raw.CurrentUsage.CacheCreationInputTokens + raw.CurrentUsage.CacheReadInputTokens
	}
	return &ContextMetric{
		UsedPercentage: clampPct(*raw.UsedPercentage),
		UsedTokens:     used,
		WindowSize:     *raw.ContextWindowSize,
	}
}

// readUsage 解析账号级 external-usage 快照的 5h / 周限两窗口。快照过期 → 整体丢弃。
func readUsage() (five, seven *UsageWindow) {
	dir, ok := pluginDir()
	if !ok {
		return nil, nil
	}
	data, err := filex.ReadLimit(filepath.Join(dir, "external-usage.json"), readCap)
	if err != nil {
		return nil, nil
	}
	var raw struct {
		UpdatedAt string    `json:"updated_at"`
		FiveHour  rawWindow `json:"five_hour"`
		SevenDay  rawWindow `json:"seven_day"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, raw.UpdatedAt); err != nil || time.Since(t) > usageStaleAfter {
		return nil, nil
	}
	return raw.FiveHour.toWindow(), raw.SevenDay.toWindow()
}

// rawWindow 是 external-usage 快照里单个窗口的原始形态（used_percentage 可为 null，resets_at 为 ISO 串）。
type rawWindow struct {
	UsedPercentage *float64 `json:"used_percentage"`
	ResetsAt       *string  `json:"resets_at"`
}

func (w rawWindow) toWindow() *UsageWindow {
	if w.UsedPercentage == nil {
		return nil
	}
	out := &UsageWindow{UsedPercentage: clampPct(*w.UsedPercentage)}
	if w.ResetsAt != nil {
		// 仅当重置时刻在未来才带 resets：已过 = 窗口已重置，旧百分比无意义，前端不显示剩余时间。
		if t, err := time.Parse(time.RFC3339, *w.ResetsAt); err == nil && t.After(time.Now()) {
			out.ResetsAtMs = t.UnixMilli()
		}
	}
	return out
}

// clampPct 把百分比夹到 [0,100] 并四舍五入（对齐 claude-hud Math.round(clamp)）。
func clampPct(v float64) int {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return int(v + 0.5)
}
