package storage

import "time"

// 本文件是 cc-switch 式「全局配置 Provider」域的表：把 Claude Code / Codex 等 CLI
// 的全局配置文件（~/.claude/settings.json、~/.codex/{auth.json,config.toml}）整份
// 存成一个 Provider，切换时原子性直写 live 配置文件（对齐 cc-switch 的做法），而不是
// 像旧 providers 表那样只存几个零散字段再拼装启动 env。
//
// 三张表：
//   - AgentProvider：Provider 本体，SettingsEnc 是该 CLI 全局配置文件完整内容的密文。
//   - AgentMcpServer：MCP server 单一事实来源（本单元只建表，投影逻辑留后续单元）。
//   - AgentSkill：Skill 单一事实来源元数据（实体目录另存数据目录，本单元只建表）。
//
// 三者均含敏感凭据或不可从别处重建的状态 → Clearable 全部 false（同 providers/projects 档）。

func init() {
	AllModels = append(AllModels,
		ModelDescriptor{Model: &AgentProvider{}, TableName: "agent_providers", LabelKey: "agentProviders", Clearable: false},
		ModelDescriptor{Model: &AgentMcpServer{}, TableName: "agent_mcp_servers", LabelKey: "agentMcpServers", Clearable: false},
		ModelDescriptor{Model: &AgentSkill{}, TableName: "agent_skills", LabelKey: "agentSkills", Clearable: false},
		ModelDescriptor{Model: &AgentCfgKV{}, TableName: "agent_cfg_kv", LabelKey: "agentCfgKV", Clearable: false},
		ModelDescriptor{Model: &AgentBaseConfig{}, TableName: "agent_base_configs", LabelKey: "agentBaseConfigs", Clearable: false},
	)
}

// AgentProvider 是 cc-switch 式全局配置 Provider。SettingsEnc 是「该 CLI 全局配置文件
// 完整内容」的 JSON 文本经 cryptox 加密后的密文（内嵌 API key，铁律必须加密落库）。
//
//   - claude：settings.json 的完整 JSON 对象；
//   - codex：{"auth": <auth.json 对象|null>, "config": "<config.toml 全文>"}。
type AgentProvider struct {
	ID          string `gorm:"primaryKey;size:36" json:"id"`
	AppType     string `gorm:"type:text;not null;index" json:"appType"` // claude | codex
	Name        string `gorm:"type:text;not null" json:"name"`
	SettingsEnc string `gorm:"type:text;column:settings_enc" json:"-"`
	Category    string `gorm:"type:text;not null;default:'custom'" json:"category"` // official | custom | imported
	IsCurrent   bool   `gorm:"not null;default:false;index" json:"isCurrent"`
	SortOrder   int    `gorm:"not null;default:0" json:"sortOrder"`
	Notes       string `gorm:"type:text" json:"notes,omitempty"`
	WebsiteURL  string `gorm:"type:text;column:website_url" json:"websiteUrl,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AgentMcpServer 是 MCP server 单一事实来源（本单元只建表，投影到各 CLI live 配置的
// 逻辑在后续单元实现）。ServerEnc 是 server 定义 JSON（{command,args,env,url,type,...}）
// 的 cryptox 密文（env 里可能含 token，铁律必须加密落库）。
type AgentMcpServer struct {
	ID            string `gorm:"primaryKey;size:36" json:"id"`
	Name          string `gorm:"type:text;not null;uniqueIndex" json:"name"` // 写入 live 配置的键名
	ServerEnc     string `gorm:"type:text;column:server_enc" json:"-"`
	EnabledClaude bool   `gorm:"not null;default:false" json:"enabledClaude"`
	EnabledCodex  bool   `gorm:"not null;default:false" json:"enabledCodex"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AgentSkill 是 Skill 单一事实来源的元数据（实体目录落在数据目录 agent-skills/ 下，
// 本单元只建表）。ID 即 SSOT 目录名。
type AgentSkill struct {
	ID            string `gorm:"primaryKey;size:128" json:"id"` // SSOT 目录名
	Name          string `gorm:"type:text;not null" json:"name"`
	Description   string `gorm:"type:text;not null;default:''" json:"description"`
	EnabledClaude bool   `gorm:"not null;default:false" json:"enabledClaude"`
	EnabledCodex  bool   `gorm:"not null;default:false" json:"enabledCodex"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AgentCfgKV 是 agentcfg 域的内部小状态 KV（当前仅存 claude/reclaude 共用的
// ~/.claude/settings.json 的「文件归属」标记，键 claude_settings_owner）。
type AgentCfgKV struct {
	Key   string `gorm:"primaryKey;size:64" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}

// AgentBaseConfig 是 claude/reclaude 两层合并模型的「系统配置基底」(公共基底)。
// 按 appType 各一行(claude | reclaude 独立)。SettingsEnc 是基底 JSON 对象的 cryptox 密文
// (hooks 的 command 可能内嵌敏感串,按铁律加密;不复用明文的 AgentCfgKV)。codex/grok 无基底行。
type AgentBaseConfig struct {
	AppType     string    `gorm:"primaryKey;size:16" json:"appType"` // claude | reclaude
	SettingsEnc string    `gorm:"type:text;column:settings_enc" json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// 显式 TableName：避免依赖 GORM 复数化推断，表名与 ModelDescriptor 严格一致。
func (AgentProvider) TableName() string   { return "agent_providers" }
func (AgentMcpServer) TableName() string  { return "agent_mcp_servers" }
func (AgentSkill) TableName() string      { return "agent_skills" }
func (AgentCfgKV) TableName() string      { return "agent_cfg_kv" }
func (AgentBaseConfig) TableName() string { return "agent_base_configs" }
