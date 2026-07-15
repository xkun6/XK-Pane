package agentcfg

// 本文件实现 MCP server 的 SSOT + 投影：agent_mcp_servers 表以 XK-Pane 为准，按
// EnabledClaude / EnabledCodex 开关投影到各 CLI 的 live 位置——Claude 是
// ~/.claude.json 顶层的 mcpServers 对象，Codex 是 ~/.codex/config.toml 的
// [mcp_servers.*] 段（复用 codex.go 已有的 stripMcpServers / renderCodexMcpToml）。
// 「禁用」= 从 live 移除但 SSOT 保留；「删除」= SSOT 行与两侧 live 键一并移除。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
	"xkpane/internal/utils/filex"

	"github.com/google/uuid"
	toml "github.com/pelletier/go-toml/v2"
	"gorm.io/gorm"
)

var errMcpNotFound = errors.New("agentcfg: mcp server does not exist")

// mcpNamePattern 是 MCP server 名称的合法字符集——该名称同时也是写入两侧 live 配置的
// 键名，必须是 JSON 对象键 / TOML 表名都合法的安全字符集。
var mcpNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// AgentMcpServerDTO 是前端可见视图：ServerJSON 是解密后的**明文** server 定义 JSON
// （{command,args,env,url,type,...}）。仅供桌面本地 IPC 使用，理由同 AgentProviderDTO。
type AgentMcpServerDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	ServerJSON    string    `json:"serverJson"`
	EnabledClaude bool      `json:"enabledClaude"`
	EnabledCodex  bool      `json:"enabledCodex"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// AgentMcpServerInput 是前端提交的可写字段。
type AgentMcpServerInput struct {
	Name          string `json:"name"`
	ServerJSON    string `json:"serverJson"`
	EnabledClaude bool   `json:"enabledClaude"`
	EnabledCodex  bool   `json:"enabledCodex"`
}

func (s *Service) mcpToDTO(row storage.AgentMcpServer) (AgentMcpServerDTO, error) {
	plain, err := cryptox.DecryptString(row.ServerEnc)
	if err != nil {
		return AgentMcpServerDTO{}, fmt.Errorf("agentcfg: 解密 mcp server %s 失败: %w", row.ID, err)
	}
	return AgentMcpServerDTO{
		ID:            row.ID,
		Name:          row.Name,
		ServerJSON:    plain,
		EnabledClaude: row.EnabledClaude,
		EnabledCodex:  row.EnabledCodex,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}, nil
}

// validateMcpName 校验 name 非空且匹配 mcpNamePattern——正则本身要求至少一个字符，
// 空字符串已经不匹配，不必再单独判空；含首尾空白等非法字符同样会在这里被拒绝，不做
// 静默 trim（trim 会把「用户输入带空格」的错误悄悄纠正掉）。
func validateMcpName(name string) error {
	if !mcpNamePattern.MatchString(name) {
		return fmt.Errorf("agentcfg: name 不合法（仅允许字母数字 . _ -，且非空）: %q", name)
	}
	return nil
}

func validateMcpServerJSON(raw string) error {
	var v map[string]any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return fmt.Errorf("agentcfg: serverJson 不是合法的 JSON 对象: %w", err)
	}
	return nil
}

// ListMcpServers 按 Name 排序列出全部 MCP server。
func (s *Service) ListMcpServers(ctx context.Context) ([]AgentMcpServerDTO, error) {
	var rows []storage.AgentMcpServer
	if err := s.db().WithContext(ctx).Order("name").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AgentMcpServerDTO, 0, len(rows))
	for _, row := range rows {
		dto, err := s.mcpToDTO(row)
		if err != nil {
			return nil, err
		}
		out = append(out, dto)
	}
	return out, nil
}

// AddMcpServer 新增一个 MCP server 行（服务端生成 id），随即按其 Enabled 开关投影到
// 两侧 live 配置。
func (s *Service) AddMcpServer(ctx context.Context, in AgentMcpServerInput) (*AgentMcpServerDTO, error) {
	if err := validateMcpName(in.Name); err != nil {
		return nil, err
	}
	if err := validateMcpServerJSON(in.ServerJSON); err != nil {
		return nil, err
	}
	enc, err := cryptox.EncryptString(in.ServerJSON)
	if err != nil {
		return nil, err
	}
	row := storage.AgentMcpServer{
		ID:            uuid.NewString(),
		Name:          in.Name,
		ServerEnc:     enc,
		EnabledClaude: in.EnabledClaude,
		EnabledCodex:  in.EnabledCodex,
	}
	if err := s.db().WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	if err := s.projectMcp(ctx, nil); err != nil {
		return nil, err
	}
	dto, err := s.mcpToDTO(row)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

// UpdateMcpServer 修改一个 MCP server 行，允许改名。改名时把旧名键从两侧 live 显式
// 移除（新名键由投影按当前 Enabled 状态写回）——旧名不再对应任何 SSOT 行，projectMcp
// 按行遍历的常规逻辑够不到它，必须靠 removedNames 补一刀。
func (s *Service) UpdateMcpServer(ctx context.Context, id string, in AgentMcpServerInput) (*AgentMcpServerDTO, error) {
	if err := validateMcpName(in.Name); err != nil {
		return nil, err
	}
	if err := validateMcpServerJSON(in.ServerJSON); err != nil {
		return nil, err
	}
	enc, err := cryptox.EncryptString(in.ServerJSON)
	if err != nil {
		return nil, err
	}

	var existing storage.AgentMcpServer
	if err := s.db().WithContext(ctx).First(&existing, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errMcpNotFound
		}
		return nil, err
	}
	oldName := existing.Name

	updates := map[string]any{
		"name":           in.Name,
		"server_enc":     enc,
		"enabled_claude": in.EnabledClaude,
		"enabled_codex":  in.EnabledCodex,
	}
	if err := s.db().WithContext(ctx).Model(&storage.AgentMcpServer{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}

	var removed []string
	if oldName != in.Name {
		removed = []string{oldName}
	}
	if err := s.projectMcp(ctx, removed); err != nil {
		return nil, err
	}

	var row storage.AgentMcpServer
	if err := s.db().WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	dto, err := s.mcpToDTO(row)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

// DeleteMcpServer 删除一个 MCP server 行，并把其名字键从两侧 live 移除。
func (s *Service) DeleteMcpServer(ctx context.Context, id string) error {
	var row storage.AgentMcpServer
	if err := s.db().WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errMcpNotFound
		}
		return err
	}
	if err := s.db().WithContext(ctx).Delete(&storage.AgentMcpServer{}, "id = ?", id).Error; err != nil {
		return err
	}
	return s.projectMcp(ctx, []string{row.Name})
}

// projectMcp 把 agent_mcp_servers 表的当前状态投影到两侧 live 配置。removedNames 是
// 本次操作中「不再对应任何 SSOT 行」的旧名字（改名产生旧名 / 删除产生被删名），需要
// 显式从两侧 live 清除——Codex 侧因为整段重建（stripMcpServers 全量剥离旧投影段再按
// 当前 SSOT 重新拼），不需要 removedNames；Claude 侧是就地增删单个键，需要它。
//
// 调用时机：必须在触发它的 DB 写操作提交之后调用，不能包在同一个事务里——holder 的
// 连接池 MaxOpenConns=1，事务内再发起新查询会自我死锁，见 renderCodexMcpToml 的注释。
// 任一侧失败即返回错误；另一侧若已写成功不回滚——两侧写入都是幂等的（下次任意一次
// 增删改都会重新投影出正确状态），失败重试即可收敛。
func (s *Service) projectMcp(ctx context.Context, removedNames []string) error {
	if err := s.projectMcpClaude(ctx, removedNames); err != nil {
		return err
	}
	return s.projectMcpCodex(ctx)
}

// readClaudeMcpFull 读 ~/.claude.json 整份内容为 map。文件不存在 → 空 map（不是错误，
// 首次使用本功能时这个文件也可能还没有）。
func (s *Service) readClaudeMcpFull() (map[string]any, error) {
	path := s.claudeMcpPath()
	data, err := filex.ReadLimit(path, 64<<20)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("agentcfg: 读取 %s 失败: %w", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("agentcfg: 解析 %s 失败: %w", path, err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// projectMcpClaude 只碰 ~/.claude.json 的 mcpServers 子对象，其余顶层键（projects /
// history / 用户自己加的任何字段）原样保留；mcpServers 内部也只处理 SSOT 认领的键
// （本表各行的 Name）与 removedNames，用户手工加的其它 mcpServer 键不受影响。
func (s *Service) projectMcpClaude(ctx context.Context, removedNames []string) error {
	var rows []storage.AgentMcpServer
	if err := s.db().WithContext(ctx).Find(&rows).Error; err != nil {
		return err
	}

	full, err := s.readClaudeMcpFull()
	if err != nil {
		return err
	}
	mcpServers, _ := full["mcpServers"].(map[string]any)
	if mcpServers == nil {
		mcpServers = map[string]any{}
	}

	for _, row := range rows {
		if !row.EnabledClaude {
			delete(mcpServers, row.Name)
			continue
		}
		plain, err := cryptox.DecryptString(row.ServerEnc)
		if err != nil {
			s.log.Warn("解密 mcp server 失败，跳过投影", "name", row.Name, "err", err)
			continue
		}
		var def map[string]any
		if err := json.Unmarshal([]byte(plain), &def); err != nil {
			s.log.Warn("解析 mcp server 定义失败，跳过投影", "name", row.Name, "err", err)
			continue
		}
		mcpServers[row.Name] = def
	}
	for _, name := range removedNames {
		delete(mcpServers, name)
	}
	full["mcpServers"] = mcpServers

	path := s.claudeMcpPath()
	if err := backupLiveOnce(path); err != nil {
		return fmt.Errorf("agentcfg: 备份 live %s 失败: %w", path, err)
	}
	data, err := json.MarshalIndent(full, "", "  ")
	if err != nil {
		return err
	}
	if err := filex.WriteAtomic(path, data, 0o600); err != nil {
		return fmt.Errorf("agentcfg: 写入 live %s 失败: %w", path, err)
	}
	return nil
}

// projectMcpCodex 把 config.toml 的 [mcp_servers.*] 段整体重建：先剥掉旧投影段
// （stripMcpServers），再按当前 SSOT 的 EnabledCodex=true 行重新拼一段
// （renderCodexMcpToml，与 writeCodexLive 用的是同一份逻辑）。整体重建意味着改名 /
// 删除天然生效，不需要 removedNames。
func (s *Service) projectMcpCodex(ctx context.Context) error {
	path := s.codexConfigPath()
	raw := ""
	data, err := filex.ReadLimit(path, 8<<20)
	switch {
	case err == nil:
		raw = string(data)
	case errors.Is(err, os.ErrNotExist):
		// config.toml 尚不存在，raw 保持 ""
	default:
		return fmt.Errorf("agentcfg: 读取 %s 失败: %w", path, err)
	}

	stripped, err := stripMcpServers(raw)
	if err != nil {
		return fmt.Errorf("agentcfg: 剥离 config.toml 中的 mcp_servers 失败: %w", err)
	}
	mcpToml, err := s.renderCodexMcpToml(ctx, s.db())
	if err != nil {
		return err
	}
	finalConfig := stripped
	if mcpToml != "" {
		if strings.TrimSpace(stripped) != "" {
			finalConfig = strings.TrimRight(stripped, "\n") + "\n\n" + mcpToml
		} else {
			finalConfig = mcpToml
		}
	}
	if strings.TrimSpace(finalConfig) != "" {
		var m map[string]any
		if err := toml.Unmarshal([]byte(finalConfig), &m); err != nil {
			return fmt.Errorf("agentcfg: 拼接后的 config.toml 不是合法 TOML: %w", err)
		}
	}

	if err := backupLiveOnce(path); err != nil {
		return fmt.Errorf("agentcfg: 备份 live %s 失败: %w", path, err)
	}
	if err := filex.WriteAtomic(path, []byte(finalConfig), 0o600); err != nil {
		return fmt.Errorf("agentcfg: 写入 live %s 失败: %w", path, err)
	}
	return nil
}

// readClaudeMcpServers 读 ~/.claude.json 的 mcpServers 子对象，返回 name→定义 map，
// 供 ImportMcpFromLive 使用。文件不存在 → 空 map（跳过，不视为错误）。
func (s *Service) readClaudeMcpServers() (map[string]map[string]any, error) {
	full, err := s.readClaudeMcpFull()
	if err != nil {
		return nil, err
	}
	raw, _ := full["mcpServers"].(map[string]any)
	out := make(map[string]map[string]any, len(raw))
	for name, v := range raw {
		def, ok := v.(map[string]any)
		if !ok {
			s.log.Warn("claude mcpServers 中该键值不是对象，跳过导入", "name", name)
			continue
		}
		out[name] = def
	}
	return out, nil
}

// readCodexMcpServers 读 config.toml 的 [mcp_servers.*] 表，返回 name→定义 map（值经
// json.Marshal/Unmarshal 往返一次转成纯 JSON 兼容类型——TOML 解析出的值可能含
// map[string]interface{} 之外的具体类型，往返一次统一形状，便于与 Claude 侧比较/加密
// 存储）。文件不存在 → 空 map。
func (s *Service) readCodexMcpServers() (map[string]map[string]any, error) {
	path := s.codexConfigPath()
	data, err := filex.ReadLimit(path, 8<<20)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]map[string]any{}, nil
		}
		return nil, fmt.Errorf("agentcfg: 读取 %s 失败: %w", path, err)
	}
	var full map[string]any
	if err := toml.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("agentcfg: 解析 %s 失败: %w", path, err)
	}
	raw, _ := full["mcp_servers"].(map[string]any)
	out := make(map[string]map[string]any, len(raw))
	for name, v := range raw {
		defRaw, ok := v.(map[string]any)
		if !ok {
			s.log.Warn("codex mcp_servers 中该键值不是表，跳过导入", "name", name)
			continue
		}
		jsonBytes, err := json.Marshal(defRaw)
		if err != nil {
			s.log.Warn("序列化 codex mcp server 定义失败，跳过导入", "name", name, "err", err)
			continue
		}
		var def map[string]any
		if err := json.Unmarshal(jsonBytes, &def); err != nil {
			s.log.Warn("反序列化 codex mcp server 定义失败，跳过导入", "name", name, "err", err)
			continue
		}
		out[name] = def
	}
	return out, nil
}

// defsEqual 用规范化 JSON 文本比较两个定义是否等价（encoding/json 序列化 map 时按 key
// 字典序排序，天然规范化），仅用于导入时的「两侧定义不同」提示，不追求跨类型系统的
// 绝对精确（TOML 数字类型与 JSON 数字类型偶有差异，最坏情况多打一条无害的日志）。
func defsEqual(a, b map[string]any) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
}

// ImportMcpFromLive 从两侧 live 配置读入 SSOT，返回新增行数。已存在同名 SSOT 行 →
// 跳过，不覆盖（导入是「补齐初始状态」，不是「用 live 覆盖用户在 SSOT 里的编辑」）。
// 同名出现在两侧 → 合并成单行、两个 Enabled 都置 true，定义以 Claude 侧为准（两侧定义
// 不同只记日志，不报错——这是导入阶段的最佳努力，不是强一致校验）。
func (s *Service) ImportMcpFromLive(ctx context.Context) (int, error) {
	claudeDefs, err := s.readClaudeMcpServers()
	if err != nil {
		return 0, err
	}
	codexDefs, err := s.readCodexMcpServers()
	if err != nil {
		return 0, err
	}

	var existingNames []string
	if err := s.db().WithContext(ctx).Model(&storage.AgentMcpServer{}).Pluck("name", &existingNames).Error; err != nil {
		return 0, err
	}
	existing := make(map[string]bool, len(existingNames))
	for _, n := range existingNames {
		existing[n] = true
	}

	names := make(map[string]bool, len(claudeDefs)+len(codexDefs))
	for n := range claudeDefs {
		names[n] = true
	}
	for n := range codexDefs {
		names[n] = true
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)

	added := 0
	for _, name := range sorted {
		if existing[name] {
			continue
		}
		claudeDef, inClaude := claudeDefs[name]
		codexDef, inCodex := codexDefs[name]

		var def map[string]any
		switch {
		case inClaude && inCodex:
			def = claudeDef
			if !defsEqual(claudeDef, codexDef) {
				s.log.Info("mcp server 在 claude/codex 两侧定义不同，导入以 claude 侧为准", "name", name)
			}
		case inClaude:
			def = claudeDef
		default:
			def = codexDef
		}

		raw, err := json.Marshal(def)
		if err != nil {
			s.log.Warn("序列化导入的 mcp server 定义失败，跳过", "name", name, "err", err)
			continue
		}
		enc, err := cryptox.EncryptString(string(raw))
		if err != nil {
			return added, err
		}
		row := storage.AgentMcpServer{
			ID:            uuid.NewString(),
			Name:          name,
			ServerEnc:     enc,
			EnabledClaude: inClaude,
			EnabledCodex:  inCodex,
		}
		if err := s.db().WithContext(ctx).Create(&row).Error; err != nil {
			return added, err
		}
		added++
	}
	return added, nil
}
