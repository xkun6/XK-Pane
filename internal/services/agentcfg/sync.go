package agentcfg

// live 配置同步：启动时把磁盘上的全局配置灌入当前 Provider；运行期用 fsnotify
// 监听 live 文件（settings.json / config.toml / auth.json / CLAUDE.md / AGENTS.md），
// 外部改动后回写 DB 并向前端广播 agentcfg:live-changed，设置页自动刷新。

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xkpane/internal/events"
	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	"gorm.io/gorm"
)

// liveWatchDebounce 合并编辑器/原子写产生的连发事件。
const liveWatchDebounce = 400 * time.Millisecond

// ownWriteSuppressWindow 屏蔽本进程刚写入 live 文件触发的回读（避免自写自同步抖动）。
const ownWriteSuppressWindow = 1500 * time.Millisecond

// SetEmitter 注册事件发射器（app 层传 app.Event.Emit）。未设置时同步仍写库，只是不广播。
func (s *Service) SetEmitter(emit func(name string, payload any)) {
	s.emitMu.Lock()
	s.emit = emit
	s.emitMu.Unlock()
}

func (s *Service) emitLiveChanged(payload events.AgentCfgLiveChangedPayload) {
	s.emitMu.RLock()
	fn := s.emit
	s.emitMu.RUnlock()
	if fn != nil {
		fn(events.AgentCfgLiveChanged, payload)
	}
}

// noteOwnWrite 标记近期由本服务写入 live 文件，watcher 在窗口内忽略事件。
func (s *Service) noteOwnWrite() {
	s.suppressMu.Lock()
	s.suppressUntil = time.Now().Add(ownWriteSuppressWindow)
	s.suppressMu.Unlock()
}

func (s *Service) isOwnWriteSuppressed() bool {
	s.suppressMu.Lock()
	defer s.suppressMu.Unlock()
	return time.Now().Before(s.suppressUntil)
}

// readLiveSettingsJSON 按 appType 读 live 并规范化为 Provider SettingsJSON 形态。
// live 不存在 / 不可读 → 返回 error（调用方应跳过同步，不覆盖已有 Provider）。
func (s *Service) readLiveSettingsJSON(appType string) (string, error) {
	switch appType {
	case appTypeClaude:
		m, _, err := s.readClaudeLive()
		if err != nil {
			return "", err
		}
		data, err := json.Marshal(m)
		if err != nil {
			return "", err
		}
		return string(data), nil
	case appTypeCodex:
		return s.readCodexLive()
	case appTypeGrok:
		return s.readGrokLive()
	default:
		return "", errors.New("agentcfg: 未知 appType")
	}
}

// settingsSemanticallyEqual 比较 DB 中明文与 live 规范化文本是否语义等价。
func settingsSemanticallyEqual(appType, stored, live string) (bool, error) {
	if appType == appTypeClaude {
		var a, b map[string]any
		if err := json.Unmarshal([]byte(stored), &a); err != nil {
			return false, nil // 旧数据坏了 → 视为不等，用 live 覆盖
		}
		if err := json.Unmarshal([]byte(live), &b); err != nil {
			return false, err
		}
		na, err := json.Marshal(a)
		if err != nil {
			return false, err
		}
		nb, err := json.Marshal(b)
		if err != nil {
			return false, err
		}
		return string(na) == string(nb), nil
	}
	// codex / grok：auth+config 结构
	sa, err := parseCodexSettings(stored)
	if err != nil {
		return false, nil
	}
	sb, err := parseCodexSettings(live)
	if err != nil {
		return false, err
	}
	na, err := json.Marshal(sa)
	if err != nil {
		return false, err
	}
	nb, err := json.Marshal(sb)
	if err != nil {
		return false, err
	}
	return string(na) == string(nb), nil
}

// syncCurrentFromLive 把指定 appType 的当前 Provider 与 live 对齐。
// claude 走两层拆分（syncClaudeCurrentFromLive）；codex/grok 保持整份回灌。
// 无 current / live 不可读 / 已一致 → changed=false 且 err=nil。
func (s *Service) syncCurrentFromLive(ctx context.Context, appType string) (changed bool, err error) {
	// settings.json 是 claude/reclaude 共享文件：owner!=claude 时它此刻是 reclaude 占用期
	// 的内容（或 reclaude 运行时自改的结果），不能回灌进 claude 的 Provider。
	if appType == appTypeClaude && s.getClaudeSettingsOwner(ctx) != appTypeClaude {
		return false, nil
	}
	var current storage.AgentProvider
	qerr := s.db().WithContext(ctx).
		Where("app_type = ? AND is_current = ?", appType, true).
		First(&current).Error
	if errors.Is(qerr, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if qerr != nil {
		return false, qerr
	}

	if appType == appTypeClaude {
		return s.syncClaudeCurrentFromLive(ctx, current)
	}

	// codex / grok：整份回灌（原逻辑，不变）
	liveJSON, err := s.readLiveSettingsJSON(appType)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		s.log.Warn("read live for sync skipped", "appType", appType, "err", err)
		return false, nil
	}
	stored, err := cryptox.DecryptString(current.SettingsEnc)
	if err != nil {
		return false, err
	}
	same, err := settingsSemanticallyEqual(appType, stored, liveJSON)
	if err != nil {
		return false, err
	}
	if same {
		return false, nil
	}
	enc, err := cryptox.EncryptString(liveJSON)
	if err != nil {
		return false, err
	}
	if err := s.db().WithContext(ctx).Model(&storage.AgentProvider{}).
		Where("id = ?", current.ID).
		Update("settings_enc", enc).Error; err != nil {
		return false, err
	}
	return true, nil
}

// syncClaudeCurrentFromLive 是 claude 反向同步：live 是 deepMerge(基底,当前补丁) 的产物，
// 外部编辑器改了它之后，按叶子路径把改动拆回「基底」与「当前供应商补丁」——凭据路径
// （当前补丁已声明的 ∪ 凭据兜底集∩live）→ 回灌补丁；其余 → 回灌基底。storedMerged 与
// live 语义相等则无变化。事务内一起更新补丁与基底，保证 deepMerge(基底',补丁') 重建 live。
func (s *Service) syncClaudeCurrentFromLive(ctx context.Context, current storage.AgentProvider) (bool, error) {
	liveMap, _, err := s.readClaudeLive()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		s.log.Warn("read live for sync skipped", "appType", appTypeClaude, "err", err)
		return false, nil
	}
	stored, err := cryptox.DecryptString(current.SettingsEnc)
	if err != nil {
		return false, err
	}
	var patch map[string]any
	if err := json.Unmarshal([]byte(stored), &patch); err != nil {
		patch = map[string]any{} // 旧数据坏 → 视为空补丁，让 live 内容全部归基底
	}
	base, err := s.readBaseConfig(s.db().WithContext(ctx), appTypeClaude)
	if err != nil {
		return false, err
	}
	sm, err := json.Marshal(deepMergeJSON(base, patch))
	if err != nil {
		return false, err
	}
	lm, err := json.Marshal(liveMap)
	if err != nil {
		return false, err
	}
	if string(sm) == string(lm) {
		return false, nil
	}

	patchPart, basePart := splitByPaths(liveMap, credPathSet(liveMap, patch))
	if err := s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		patchData, mErr := json.Marshal(patchPart)
		if mErr != nil {
			return mErr
		}
		enc, encErr := cryptox.EncryptString(string(patchData))
		if encErr != nil {
			return encErr
		}
		if err := tx.Model(&storage.AgentProvider{}).Where("id = ?", current.ID).
			Update("settings_enc", enc).Error; err != nil {
			return err
		}
		return s.writeBaseConfig(tx, appTypeClaude, basePart)
	}); err != nil {
		return false, err
	}
	return true, nil
}

// syncAllCurrentFromLive 对三个 appType 各自把 current 与 live 对齐（测试 / 手动批量用；
// 正常路径走 ListAgentProviders 的按需同步，不再在启动时调用）。
func (s *Service) syncAllCurrentFromLive(ctx context.Context) {
	for _, appType := range []string{appTypeClaude, appTypeCodex, appTypeGrok} {
		changed, err := s.syncCurrentFromLive(ctx, appType)
		if err != nil {
			s.log.Warn("sync current from live failed", "appType", appType, "err", err)
			continue
		}
		if changed {
			s.log.Info("synced current provider from live", "appType", appType)
		}
	}
}

// startLiveWatcher 监听全局配置目录；ctx 取消时退出。失败只记日志。
func (s *Service) startLiveWatcher(ctx context.Context) {
	if s.home == "" {
		s.log.Warn("live watcher skipped: home dir empty")
		return
	}
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		s.log.Warn("create live fsnotify watcher failed", "err", err)
		return
	}

	dirs := []string{
		filepath.Join(s.home, ".claude"),
		filepath.Join(s.home, ".codex"),
		filepath.Join(s.home, ".grok"),
	}
	watched := 0
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		if err := fsw.Add(dir); err != nil {
			s.log.Warn("watch live dir failed", "dir", dir, "err", err)
			continue
		}
		watched++
	}
	if watched == 0 {
		_ = fsw.Close()
		s.log.Info("live watcher: no global config dirs to watch yet")
		return
	}

	debounced := debounce.New(liveWatchDebounce)
	var mu sync.Mutex
	pending := map[string]struct{}{} // key = "kind|appType"

	go func() {
		defer fsw.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-fsw.Errors:
				if !ok {
					return
				}
				s.log.Warn("live watcher error", "err", err)
			case ev, ok := <-fsw.Events:
				if !ok {
					return
				}
				// 只关心写/创建/改名（原子写常表现为 rename）
				if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
				kind, appType, matched := s.classifyLivePath(ev.Name)
				if !matched {
					continue
				}
				key := kind + "|" + appType
				mu.Lock()
				pending[key] = struct{}{}
				mu.Unlock()
				debounced(func() {
					if ctx.Err() != nil {
						return
					}
					if s.isOwnWriteSuppressed() {
						mu.Lock()
						pending = map[string]struct{}{}
						mu.Unlock()
						return
					}
					mu.Lock()
					batch := pending
					pending = map[string]struct{}{}
					mu.Unlock()
					for k := range batch {
						parts := strings.SplitN(k, "|", 2)
						if len(parts) != 2 {
							continue
						}
						s.handleLiveChange(ctx, parts[0], parts[1])
					}
				})
			}
		}
	}()
	s.log.Info("live config watcher started", "dirs", watched)
}

// classifyLivePath 判断绝对路径是否是我们关心的全局配置/指令文件。
// kind: "provider" | "instruction"
func (s *Service) classifyLivePath(path string) (kind, appType string, ok bool) {
	path = filepath.Clean(path)
	base := filepath.Base(path)
	dir := filepath.Dir(path)

	claudeDir := filepath.Clean(filepath.Join(s.home, ".claude"))
	codexDir := filepath.Clean(filepath.Join(s.home, ".codex"))
	grokDir := filepath.Clean(filepath.Join(s.home, ".grok"))

	switch {
	case samePath(dir, claudeDir) && equalFoldBase(base, "settings.json"):
		return "provider", appTypeClaude, true
	case samePath(dir, claudeDir) && equalFoldBase(base, "CLAUDE.md"):
		return "instruction", appTypeClaude, true
	case samePath(dir, codexDir) && (equalFoldBase(base, "config.toml") || equalFoldBase(base, "auth.json")):
		return "provider", appTypeCodex, true
	case samePath(dir, codexDir) && equalFoldBase(base, "AGENTS.md"):
		return "instruction", appTypeCodex, true
	case samePath(dir, grokDir) && (equalFoldBase(base, "config.toml") || equalFoldBase(base, "auth.json")):
		return "provider", appTypeGrok, true
	default:
		return "", "", false
	}
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func equalFoldBase(a, b string) bool {
	return strings.EqualFold(a, b)
}

// handleLiveChange 处理一批去抖后的变更：provider 写回 current；instruction 只广播。
func (s *Service) handleLiveChange(ctx context.Context, kind, appType string) {
	switch kind {
	case "provider":
		changed, err := s.syncCurrentFromLive(ctx, appType)
		if err != nil {
			s.log.Warn("live change sync failed", "appType", appType, "err", err)
			return
		}
		// 仅在内容确实变化时广播，减少噪音。
		if changed {
			s.emitLiveChanged(events.AgentCfgLiveChangedPayload{
				Kind:    "provider",
				AppType: appType,
			})
		}
	case "instruction":
		s.emitLiveChanged(events.AgentCfgLiveChangedPayload{
			Kind:    "instruction",
			AppType: appType,
		})
	}
}
