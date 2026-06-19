// Package agentusage 统计本机 Claude Code 会话的 token 用量（近5h / 今日 / 本周）。
//
// 移植 fanbox-win server.js 的 parseClaudeFile + claudeUsage：不依赖任何 CLI 在跑、也不依赖
// claude-hud 插件，直接读 Claude Code 落在本机的会话日志 ~/.claude/projects/**/*.jsonl ——
// 每条 assistant 消息带 message.usage（input/output/cache_creation/cache_read 四类 token）。
// 把近一个多月的事件按时间桶聚合成 近5h / 今日 / 本周 / 本月 四个窗口的 token 总量，
// 供左侧栏「设置齿轮上方」竖向显示。
//
// 与 hudstats 的区别：hudstats 读 claude-hud 落盘的账号级 5h/周限「百分比」（需插件 + 配置），
// 本包是自包含的「绝对 token 总量」统计（整机所有 Claude Code 会话之和），口径正是 fanbox 的
// 「token 总量 · 本地会话日志统计」那一行。
//
// 增量解析：每个文件记 offset/lastMsgID/events 缓存，下一轮只读新追加的字节（首轮全量、之后极廉价）。
// 纯只读、best-effort：任何目录/文件缺失或解析失败都静默跳过，返回已聚合到的部分（不报错）。
package agentusage

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xkpane/internal/utils/logx"
)

// 时间窗口常量：事件按 5h / 今日 / 本周 / 本月分桶。文件 mtime 截止必须 ≥ 最宽窗口（本月，
// 最早可回溯到月初 ~31 天前），否则早于截止的文件被整文件跳过 → 本月被截断。取 35 天留余量。
const (
	fileCutoff     = 35 * 24 * time.Hour // 早于此 mtime 的 jsonl 不再扫（也踢出缓存）；须覆盖本月
	resultCacheTTL = 30 * time.Second    // 结果缓存：30s 内重复调用直接返回上次结果
	fiveHourWindow = 5 * time.Hour
	weekWindow     = 7 * 24 * time.Hour
)

// TokenWindow 是单个时间窗口的 token 聚合。Total = Input+Output+CacheCreate+CacheRead（对齐 fanbox tot）。
type TokenWindow struct {
	Total       int64 `json:"total"`
	Input       int64 `json:"input"`
	Output      int64 `json:"output"`
	CacheRead   int64 `json:"cacheRead"`
	CacheCreate int64 `json:"cacheCreate"`
	Msgs        int   `json:"msgs"`
}

// TokenUsage 是三窗口聚合结果。Available=false 表示没找到 ~/.claude/projects（没装/没用过 Claude Code），
// 前端据此不渲染用量条。At 是聚合时刻 unix ms（前端可显示「刚刚更新」之类，可不用）。
type TokenUsage struct {
	Available bool        `json:"available"`
	At        int64       `json:"at"`
	Last5h    TokenWindow `json:"last5h"`
	Today     TokenWindow `json:"today"`
	Week      TokenWindow `json:"week"`
	Month     TokenWindow `json:"month"`
}

// tokenEvent 是一条 assistant 消息的 token 明细（t = 消息时间 unix ms）。
type tokenEvent struct {
	t           int64
	in          int64
	out         int64
	cacheCreate int64
	cacheRead   int64
}

// fileState 是单个 jsonl 的增量解析状态：offset 已消费字节，lastMsgID 去重连续重复行，events 累积事件。
type fileState struct {
	offset    int64
	lastMsgID string
	events    []tokenEvent
}

// Service 持有增量解析缓存与结果缓存。无 DB 依赖、纯文件系统只读。
type Service struct {
	mu      sync.Mutex
	files   map[string]*fileState
	result  *TokenUsage
	resAt   time.Time
	homeDir string // 缓存家目录；空表示取不到（GetTokenUsage 返回 Available=false）
}

// New 创建服务。家目录解析失败不致命（运行期返回空结果）。
func New() *Service {
	home, err := os.UserHomeDir()
	if err != nil {
		logx.For("agentusage").Warn("resolve home dir failed (token 用量将不可用)", "err", err)
	}
	return &Service{
		files:   make(map[string]*fileState),
		homeDir: home,
	}
}

// GetTokenUsage 聚合本机 Claude Code 近5h/今日/本周的 token 总量。30s 内的重复调用返回缓存。
func (s *Service) GetTokenUsage(_ context.Context) (*TokenUsage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.result != nil && time.Since(s.resAt) < resultCacheTTL {
		return s.result, nil
	}

	usage := s.collect()
	s.result = usage
	s.resAt = time.Now()
	return usage, nil
}

// collect 扫 ~/.claude/projects 下所有 8 天内 jsonl，增量解析后按时间桶聚合。调用方已持锁。
func (s *Service) collect() *TokenUsage {
	now := time.Now()
	out := &TokenUsage{At: now.UnixMilli()}
	if s.homeDir == "" {
		return out
	}

	projRoot := filepath.Join(s.homeDir, ".claude", "projects")
	dirs, err := os.ReadDir(projRoot)
	if err != nil {
		return out // 没装 / 没用过 Claude Code：Available 保持 false
	}
	out.Available = true

	cutoff := now.Add(-fileCutoff)
	live := make(map[string]struct{})
	var events []tokenEvent
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		base := filepath.Join(projRoot, d.Name())
		names, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, n := range names {
			if n.IsDir() || !strings.HasSuffix(n.Name(), ".jsonl") {
				continue
			}
			fp := filepath.Join(base, n.Name())
			info, err := n.Info()
			if err != nil || info.ModTime().Before(cutoff) {
				continue
			}
			live[fp] = struct{}{}
			events = append(events, s.parseFile(fp, info)...)
		}
	}
	// 过期文件踢出缓存（避免无限增长 + 释放 events 内存）。
	for k := range s.files {
		if _, ok := live[k]; !ok {
			delete(s.files, k)
		}
	}

	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UnixMilli()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).UnixMilli()
	from5h := now.Add(-fiveHourWindow).UnixMilli()
	fromWeek := now.Add(-weekWindow).UnixMilli()
	for _, e := range events {
		if e.t >= from5h {
			out.Last5h.add(e)
		}
		if e.t >= dayStart {
			out.Today.add(e)
		}
		if e.t >= fromWeek {
			out.Week.add(e)
		}
		if e.t >= monthStart {
			out.Month.add(e)
		}
	}
	return out
}

func (w *TokenWindow) add(e tokenEvent) {
	w.Input += e.in
	w.Output += e.out
	w.CacheCreate += e.cacheCreate
	w.CacheRead += e.cacheRead
	w.Total += e.in + e.out + e.cacheCreate + e.cacheRead
	w.Msgs++
}

// parseFile 增量解析单个 jsonl，返回该文件累积到的全部事件（含历史已解析的）。调用方已持锁。
func (s *Service) parseFile(fp string, info os.FileInfo) []tokenEvent {
	st := s.files[fp]
	if st == nil {
		st = &fileState{}
		s.files[fp] = st
	}
	size := info.Size()
	if size < st.offset { // 文件被截断重写：重来
		st.offset, st.lastMsgID, st.events = 0, "", nil
	}
	if size == st.offset {
		return st.events
	}

	f, err := os.Open(fp)
	if err != nil {
		return st.events
	}
	defer f.Close()
	if _, err := f.Seek(st.offset, io.SeekStart); err != nil {
		return st.events
	}
	buf := make([]byte, size-st.offset)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return st.events
	}
	buf = buf[:n]
	// 末尾可能是写到一半的行：只消费到最后一个完整换行，剩下留给下一轮。
	lastNL := lastIndexByte(buf, '\n')
	if lastNL < 0 {
		return st.events
	}
	st.offset += int64(lastNL + 1)

	for _, line := range strings.Split(string(buf[:lastNL]), "\n") {
		// 廉价预筛：跳过绝大多数不含 usage 的行（user / tool_result / 系统行）。
		if !strings.Contains(line, "\"usage\"") || !strings.Contains(line, "\"assistant\"") {
			continue
		}
		var d struct {
			Type      string `json:"type"`
			Timestamp string `json:"timestamp"`
			Message   struct {
				ID    string `json:"id"`
				Model string `json:"model"`
				Usage *struct {
					InputTokens              int64 `json:"input_tokens"`
					OutputTokens             int64 `json:"output_tokens"`
					CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
					CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &d); err != nil {
			continue
		}
		if d.Type != "assistant" || d.Message.Usage == nil || d.Message.Model == "<synthetic>" {
			continue
		}
		// 同一条消息可能分多行落盘（流式），usage 重复：仅记第一次（按 message.id 去重连续重复）。
		if d.Message.ID != "" && d.Message.ID == st.lastMsgID {
			continue
		}
		if d.Message.ID != "" {
			st.lastMsgID = d.Message.ID
		}
		t := info.ModTime().UnixMilli()
		if ts, err := time.Parse(time.RFC3339, d.Timestamp); err == nil {
			t = ts.UnixMilli()
		}
		u := d.Message.Usage
		st.events = append(st.events, tokenEvent{
			t:           t,
			in:          u.InputTokens,
			out:         u.OutputTokens,
			cacheCreate: u.CacheCreationInputTokens,
			cacheRead:   u.CacheReadInputTokens,
		})
	}
	return st.events
}

func lastIndexByte(b []byte, c byte) int {
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == c {
			return i
		}
	}
	return -1
}
