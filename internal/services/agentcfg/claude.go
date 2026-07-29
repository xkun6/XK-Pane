package agentcfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
	"xkpane/internal/utils/filex"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// claudeCredLeafPaths 是 claude 供应商「凭据」叶子路径兜底集(用 leafPathSep 连接)。
// 反向同步/回填时,这些路径连同当前补丁已声明的路径一起被排除在系统配置基底之外,
// 防止用户手加的 token/base_url 漏进公共基底、进而在切到无凭据的供应商时串味。
//
// model 刻意不在此集合:模型选择属于公共配置。早期把 model 当凭据,导致 claude CLI
// 自己改写 settings.json 的 model(/model 切模型)被反向同步写进当前供应商补丁,
// 供应商配置被替换成 {"model":"opus[1m]"}(历史数据由 migrate_model_to_base.go 归位)。
var claudeCredLeafPaths = []string{
	"env" + leafPathSep + "ANTHROPIC_AUTH_TOKEN",
	"env" + leafPathSep + "ANTHROPIC_API_KEY",
	"env" + leafPathSep + "ANTHROPIC_BASE_URL",
}

// credPathSet 组装「不进入系统配置基底」的叶子路径集 = 当前补丁已声明的所有叶子路径
// ∪ (claude 凭据兜底路径 ∩ live 中确实存在的路径)。这些路径的值由供应商补丁提供,
// 存进基底会在切到未声明该路径的供应商时串味。
func credPathSet(live, oldPatch map[string]any) map[string]struct{} {
	set := flattenLeafPaths(oldPatch)
	liveLeaves := flattenLeafPaths(live)
	for _, p := range claudeCredLeafPaths {
		if _, ok := liveLeaves[p]; ok {
			set[p] = struct{}{}
		}
	}
	return set
}

// reconcileBaseAgainstLive 用「live 里非凭据路径的最新值」与「旧基底自身凭据路径的既有值」
// 拼出新基底：credSet 覆盖的路径,其权威值只能来自供应商补丁,旧基底里恰好落在这些路径下
// 的既有值必须原样保留,绝不能被 live 当前内容直接覆盖或清空。
//
// 这一步是必须的：credPathSet 纯按路径名称模式(是否长得像凭据)判定要排除的路径,并不管
// 该路径当前是否真的被某个补丁声明——首次导入等场景下,凭据形状的字段可能只存在于基底、
// 补丁仍为空。若直接用「live 排除 credSet 后的剩余部分」整份替换基底(writeBaseConfig 是
// 整份覆盖语义,不做合并),就会把基底里这些路径下本来合法存在、且未被任何东西记录第二份
// 的值连带清空——不管这次调用是"确无变化"还是"有变化但应被忽略"，都会造成真实数据丢失。
func reconcileBaseAgainstLive(oldBase, liveMap map[string]any, credSet map[string]struct{}) map[string]any {
	credPart, _ := splitByPaths(oldBase, credSet)
	_, basePart := splitByPaths(liveMap, credSet)
	return deepMergeJSON(credPart, basePart)
}

// mergedLive 读 appType 系统配置基底(用给定 db/tx,复用 s.db() 或事务 tx 避免单连接死锁)
// 与传入的供应商凭据补丁 JSON,返回深合并后的规范化 JSON 文本(交给 writeClaudeLive 再规范化)。
func (s *Service) mergedLive(db *gorm.DB, appType, patchJSON string) (string, error) {
	var patch map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
		return "", fmt.Errorf("agentcfg: 补丁不是合法 JSON 对象: %w", err)
	}
	base, err := s.readBaseConfig(db, appType)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(deepMergeJSON(base, patch))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// readClaudeLive 读并解析 Claude 全局配置文件。文件不存在返回可用 errors.Is(err,
// os.ErrNotExist) 区分的错误；JSON 解析失败返回错误。
func (s *Service) readClaudeLive() (map[string]any, []byte, error) {
	path := s.claudeSettingsPath()
	data, err := filex.ReadLimit(path, 8<<20)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("agentcfg: 读取 %s 失败: %w", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, nil, fmt.Errorf("agentcfg: 解析 %s 失败: %w", path, err)
	}
	return m, data, nil
}

// writeClaudeLive 把 settingsJSON（明文完整配置）规范化后原子写入 Claude 全局配置
// 文件，写前做一次性备份（<path>.xkpane-bak，仅在其不存在时生成一次）。
func (s *Service) writeClaudeLive(settingsJSON string) error {
	var m map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &m); err != nil {
		return fmt.Errorf("agentcfg: settingsJson 不是合法 JSON 对象: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	path := s.claudeSettingsPath()
	s.noteOwnWrite()
	if err := backupLiveOnce(path); err != nil {
		return fmt.Errorf("agentcfg: 备份 live 配置失败: %w", err)
	}
	if err := filex.WriteAtomic(path, data, 0o600); err != nil {
		return fmt.Errorf("agentcfg: 写入 live 配置失败: %w", err)
	}
	return nil
}

// backupLiveOnce 若 live 文件存在且备份文件尚不存在，把原文件字节原样备份一份。
// 已有备份 / live 尚不存在 → no-op。供 claude.go / codex.go 共用（第二单元泛化自
// 原先仅 claude 专属的 backupClaudeLiveOnce）。
func backupLiveOnce(path string) error {
	bakPath := path + ".xkpane-bak"
	if _, err := os.Stat(bakPath); err == nil {
		return nil // 已备份过
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	orig, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // live 尚不存在，无需备份
		}
		return err
	}
	return filex.WriteAtomic(bakPath, orig, 0o600)
}

// switchClaude 是 SwitchAgentProvider 的 claude 分支：
//  1. 事务内回填当前 IsCurrent 行（若存在且 live 可读合法，把 live 的公共部分沉淀进基底,
//     补丁不动，并清掉其 IsCurrent，保证单一 current 不变式）；
//  2. 目标行 IsCurrent=true；
//  3. 事务内最后一步把 deepMerge(系统公共基底, 目标供应商补丁) 写入 live —— 写失败则
//     整个事务回滚（DB 状态不变，live 未写成功也未损坏，因为 filex.WriteAtomic 本身是原子的）；
//  4. 事务成功后把 owner 标记为 claude（必须在事务外：SQLite 写锁下事务内另开连接写会超时）。
func (s *Service) switchClaude(ctx context.Context, target storage.AgentProvider) error {
	targetPlain, err := cryptox.DecryptString(target.SettingsEnc)
	if err != nil {
		return fmt.Errorf("agentcfg: 解密目标 provider 配置失败: %w", err)
	}
	err = s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current storage.AgentProvider
		err := tx.Where("app_type = ? AND is_current = ?", appTypeClaude, true).First(&current).Error
		switch {
		case err == nil:
			if err := s.backfillCurrent(tx, current); err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			// 无当前行（异常场景，正常应有 importClaudeDefault 兜底），跳过回填
		default:
			return err
		}

		if err := tx.Model(&storage.AgentProvider{}).Where("id = ?", target.ID).
			Update("is_current", true).Error; err != nil {
			return err
		}

		merged, err := s.mergedLive(tx, appTypeClaude, targetPlain)
		if err != nil {
			return err
		}
		return s.writeClaudeLive(merged)
	})
	if err != nil {
		return err
	}
	return s.setClaudeSettingsOwner(ctx, appTypeClaude)
}

// backfillCurrent 在切走前把 live 的「公共部分」沉淀进系统配置基底,并清掉旧 current 的
// IsCurrent 标志。供应商补丁一律不动 —— 它只由设置面板改写,软件外对 settings.json 的
// 编辑只能影响公共配置。新基底 = 旧基底里「当前补丁已声明的路径 ∪ 凭据兜底集」下的既有值
// (原样保留,见 reconcileBaseAgainstLive)∪ live 里其余路径的最新值。live 不可读则只清
// IsCurrent。
func (s *Service) backfillCurrent(tx *gorm.DB, current storage.AgentProvider) error {
	if liveMap, _, err := s.readClaudeLive(); err == nil {
		oldPatch := map[string]any{}
		if oldPlain, decErr := cryptox.DecryptString(current.SettingsEnc); decErr == nil {
			_ = json.Unmarshal([]byte(oldPlain), &oldPatch)
		}
		base, err := s.readBaseConfig(tx, appTypeClaude)
		if err != nil {
			return err
		}
		newBase := reconcileBaseAgainstLive(base, liveMap, credPathSet(liveMap, oldPatch))
		if err := s.writeBaseConfig(tx, appTypeClaude, newBase); err != nil {
			return err
		}
	}
	return tx.Model(&storage.AgentProvider{}).Where("id = ?", current.ID).
		Update("is_current", false).Error
}

// importDefaultTwoLayer 是 claude 两层模型下的导入逻辑(替代原先各存整份的实现)。
// 首启(无基底行)时以 live 整体作为系统配置基底,「默认配置」补丁为空 →
// deepMerge(基底=live, {})==live,与今日整份行为逐字节等价;手动导入(manual 且已有同类
// provider)不碰基底,补丁 = live 相对现有基底的深 diff。liveMap 取自 ~/.claude/settings.json。
func (s *Service) importDefaultTwoLayer(ctx context.Context, appType string, manual bool) (*AgentProviderDTO, error) {
	liveMap := map[string]any{}
	if m, _, err := s.readClaudeLive(); err == nil {
		liveMap = m
	}

	var baseCount int64
	if err := s.db().WithContext(ctx).Model(&storage.AgentBaseConfig{}).
		Where("app_type = ?", appType).Count(&baseCount).Error; err != nil {
		return nil, err
	}
	if baseCount == 0 {
		if err := s.writeBaseConfig(s.db().WithContext(ctx), appType, liveMap); err != nil {
			return nil, err
		}
	}

	var existing int64
	if err := s.db().WithContext(ctx).Model(&storage.AgentProvider{}).
		Where("app_type = ?", appType).Count(&existing).Error; err != nil {
		return nil, err
	}

	patchJSON := "{}"
	if manual && existing > 0 {
		base, err := s.readBaseConfig(s.db().WithContext(ctx), appType)
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(deepDiffJSON(liveMap, base))
		if err != nil {
			return nil, err
		}
		patchJSON = string(data)
	}

	enc, err := cryptox.EncryptString(patchJSON)
	if err != nil {
		return nil, err
	}

	row := storage.AgentProvider{
		ID:          uuid.NewString(),
		AppType:     appType,
		SettingsEnc: enc,
		Category:    "imported",
	}
	if manual && existing > 0 {
		row.Name = fmt.Sprintf("导入配置 %s", time.Now().Format("2006-01-02 15:04:05"))
		row.IsCurrent = false
	} else {
		row.Name = "默认配置"
		row.IsCurrent = true
	}

	if err := s.db().WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	dto, err := s.toDTO(row)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

func (s *Service) importClaudeDefault(ctx context.Context, manual bool) (*AgentProviderDTO, error) {
	return s.importDefaultTwoLayer(ctx, appTypeClaude, manual)
}

// claudeSettingsOwnerKey 是 AgentCfgKV 里记录「~/.claude/settings.json 当前归属」的键。
// 仅 claude 写 live；保留 owner 标记以便历史数据兼容与自写屏蔽。
const claudeSettingsOwnerKey = "claude_settings_owner"

// getClaudeSettingsOwner 返回 ~/.claude/settings.json 当前归属的 appType。
// 键缺失/空 → 缺省 claude。
func (s *Service) getClaudeSettingsOwner(ctx context.Context) string {
	var kv storage.AgentCfgKV
	if err := s.db().WithContext(ctx).First(&kv, "key = ?", claudeSettingsOwnerKey).Error; err != nil || kv.Value == "" {
		return appTypeClaude
	}
	return kv.Value
}

// setClaudeSettingsOwner 把 owner 标记写为 appType（当前仅 claude）。
func (s *Service) setClaudeSettingsOwner(ctx context.Context, appType string) error {
	return s.db().WithContext(ctx).Save(&storage.AgentCfgKV{Key: claudeSettingsOwnerKey, Value: appType}).Error
}

// ApplyForLaunch 在 terminal 每次启动 claude 会话前调用：把当前供应商凭据补丁与系统公共
// 配置基底深合并后**无条件覆盖写入** ~/.claude/settings.json（即便磁盘上已有同内容或被
// 外部改过，也强制投影一次），并把文件归属标记为 claude。
//
// 仅处理 claude；其它 appType（含 reclaude）一律 no-op，不碰 settings.json。
// 无当前 Provider 时补丁按 "{}" 处理，仍写入 deepMerge(基底, {})（保证「每次启动都覆盖」）。
// applyMu 串行化，避免两次启动交叉写。
func (s *Service) ApplyForLaunch(ctx context.Context, appType string) error {
	if appType != appTypeClaude {
		return nil
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	patchJSON := "{}"
	var cur storage.AgentProvider
	err := s.db().WithContext(ctx).Where("app_type = ? AND is_current = ?", appTypeClaude, true).First(&cur).Error
	if err == nil {
		plain, decErr := cryptox.DecryptString(cur.SettingsEnc)
		if decErr != nil {
			return fmt.Errorf("agentcfg: 解密 claude 当前配置失败: %w", decErr)
		}
		patchJSON = plain
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	merged, err := s.mergedLive(s.db(), appTypeClaude, patchJSON)
	if err != nil {
		return err
	}
	// 每次启动都强制 WriteAtomic 覆盖，不做「内容相同则跳过」优化。
	if err := s.writeClaudeLive(merged); err != nil {
		return err
	}
	return s.setClaudeSettingsOwner(ctx, appTypeClaude)
}
