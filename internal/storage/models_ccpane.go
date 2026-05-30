package storage

import "time"

// 本文件移植 cc-pane 主库（data.db）的核心表 GORM 模型。
//
// 说明：
//   - 这是全新重构（XK-Pane 独立数据目录），不导入 cc-pane 旧 SQLite 数据，
//     故时间戳用 Go 原生 time.Time（GORM 自管 CreatedAt/UpdatedAt），不沿用
//     cc-pane 的 RFC3339 文本列。json tag 保持 camelCase 与前端类型对齐。
//   - 路径字段铁律（D7）：project_path = PTY cwd（永远=用户操作目标）；
//     workspace_path 仅多目录 CLI 拼 --add-dir 且 ≠project_path；launch_cwd 为运行时观测。
//   - 敏感字段（last_prompt / ssh_config）当前无写入方（待 Phase 3 service 落地），
//     届时加 cryptox BeforeSave/AfterFind 钩子；此处先标注，避免无写入方的投机代码。
//   - im_bridge_* 表延后；mc_pe_* 表在 Phase 5 随该子系统加入。

func init() {
	AllModels = append(AllModels,
		ModelDescriptor{Model: &Project{}, TableName: "projects", LabelKey: "projects", Clearable: false},
		ModelDescriptor{Model: &LaunchHistory{}, TableName: "launch_history", LabelKey: "launchHistory", Clearable: true},
		ModelDescriptor{Model: &TerminalSession{}, TableName: "terminal_sessions", LabelKey: "terminalSessions", Clearable: true},
		ModelDescriptor{Model: &TaskBinding{}, TableName: "task_bindings", LabelKey: "taskBindings", Clearable: true},
		// Provider 含敏感凭据 → Clearable:false（禁设置页一键清空，同 projects/app_settings 档）。
		ModelDescriptor{Model: &Provider{}, TableName: "providers", LabelKey: "providers", Clearable: false},
	)
}

// Provider 是一个 CLI Agent 的接入凭据/配置（移植自 cc-pane Provider 模型）。
//
// **偏离说明**：cc-pane 把 providers 存成明文 `providers.json` 文件（无 DB 表、无加密）；
// XK-Pane 按 D8 铁律落 SQLite，且 APIKey **以 cryptox 密文存储**（在 provider service
// 层加解密，不走 GORM 钩子——因为读路径返回脱敏 DTO 无需解密、写路径是 patch 保留）。
// 故 **APIKey 字段恒为密文**：任何消费方读出后须 `cryptox.DecryptString`。无 model 字段（与源一致）。
type Provider struct {
	ID           string `gorm:"primaryKey;size:36" json:"id"`
	Name         string `gorm:"type:text;not null" json:"name"`
	ProviderType string `gorm:"type:text;not null;default:'anthropic'" json:"providerType"` // snake_case: anthropic|bedrock|vertex|proxy|config_profile|open_ai|gemini|kimi|glm|cursor|opencode
	APIKey       string `gorm:"type:text;column:api_key" json:"-"`                          // 密文（cryptox）；json:"-" 防意外序列化；"" = 未设
	BaseURL      string `gorm:"type:text;column:base_url" json:"baseUrl,omitempty"`
	Region       string `gorm:"type:text" json:"region,omitempty"`
	ProjectID    string `gorm:"type:text;column:project_id" json:"projectId,omitempty"`
	AWSProfile   string `gorm:"type:text;column:aws_profile" json:"awsProfile,omitempty"`
	ConfigDir    string `gorm:"type:text;column:config_dir" json:"configDir,omitempty"`
	IsDefault    bool   `gorm:"not null;default:false;index" json:"isDefault"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Project 是一个 Git 仓库 / 工作目录。Path 全局唯一（一个目录只登记一次）。
type Project struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	Name      string    `gorm:"type:text;not null;index" json:"name"`
	Path      string    `gorm:"type:text;not null;uniqueIndex" json:"path"`
	Alias     *string   `gorm:"type:text" json:"alias,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// LaunchHistory 启动历史（cc-pane V1–V15 合并）。
// 路径字段三件套严格遵循 D7 铁律（见文件头与 CLAUDE/findings）。
type LaunchHistory struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID   string    `gorm:"type:text;not null;index" json:"projectId"`
	ProjectName string    `gorm:"type:text;not null" json:"projectName"`
	ProjectPath string    `gorm:"type:text;not null" json:"projectPath"` // = PTY cwd（铁律：永远=操作目标）
	LaunchedAt  time.Time `gorm:"index" json:"launchedAt"`

	ClaudeSessionID *string `gorm:"type:text" json:"claudeSessionId,omitempty"`
	LastPrompt      *string `gorm:"type:text" json:"lastPrompt,omitempty"` // TODO(phase3+): 敏感，cryptox 加密
	WorkspaceName   *string `gorm:"type:text" json:"workspaceName,omitempty"`
	WorkspacePath   *string `gorm:"type:text" json:"workspacePath,omitempty"` // 仅多目录 CLI 拼 --add-dir 且 ≠project_path（铁律）
	LaunchCwd       *string `gorm:"type:text" json:"launchCwd,omitempty"`     // 运行时观测 cwd；resume 优先用它（铁律）

	ProviderID          *string `gorm:"type:text" json:"providerId,omitempty"`
	ResumeSessionID     *string `gorm:"type:text" json:"resumeSessionId,omitempty"`
	CliTool             string  `gorm:"type:text;not null;default:'none'" json:"cliTool"`
	RuntimeKind         string  `gorm:"type:text;not null;default:'local'" json:"runtimeKind"` // local | wsl | ssh
	PtySessionID        *string `gorm:"type:text" json:"ptySessionId,omitempty"`
	WslDistro           *string `gorm:"type:text" json:"wslDistro,omitempty"`
	WorkspaceSnapshotID *string `gorm:"type:text" json:"workspaceSnapshotId,omitempty"`
	LaunchProfileID     *string `gorm:"type:text" json:"launchProfileId,omitempty"`
	ProviderSelection   *string `gorm:"type:text" json:"providerSelection,omitempty"` // manual | auto | last_used
}

// TerminalSession 跨重启会话恢复（一次启动 / 标签）。
type TerminalSession struct {
	SessionID     string  `gorm:"primaryKey;size:64" json:"sessionId"`
	TabID         string  `gorm:"type:text;not null" json:"tabId"`
	PaneID        string  `gorm:"type:text;not null" json:"paneId"`
	ProjectPath   string  `gorm:"type:text;not null;index" json:"projectPath"` // = PTY cwd（铁律）
	WorkspaceName *string `gorm:"type:text" json:"workspaceName,omitempty"`
	WorkspacePath *string `gorm:"type:text" json:"workspacePath,omitempty"` // --add-dir 用（铁律）
	ProviderID    *string `gorm:"type:text" json:"providerId,omitempty"`
	CliTool       string  `gorm:"type:text;not null;default:'none'" json:"cliTool"`
	ResumeID      *string `gorm:"type:text" json:"resumeId,omitempty"`
	SshConfig     *string `gorm:"type:text" json:"sshConfig,omitempty"` // TODO(phase3): 敏感，cryptox 加密
	CustomTitle   *string `gorm:"type:text" json:"customTitle,omitempty"`
	RuntimeKind   string  `gorm:"type:text;not null;default:'local'" json:"runtimeKind"`

	WorkspaceSnapshotID *string `gorm:"type:text" json:"workspaceSnapshotId,omitempty"`
	LaunchProfileID     *string `gorm:"type:text" json:"launchProfileId,omitempty"`
	ProviderSelection   *string `gorm:"type:text" json:"providerSelection,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	SavedAt   time.Time `gorm:"index" json:"savedAt"`
}

// TaskBinding 编排任务绑定（session + todo + prompt）。
type TaskBinding struct {
	ID                string  `gorm:"primaryKey;size:36" json:"id"`
	Title             string  `gorm:"type:text;not null;index" json:"title"`
	Prompt            *string `gorm:"type:text" json:"prompt,omitempty"`
	SessionID         *string `gorm:"type:text;index" json:"sessionId,omitempty"`
	TodoID            *string `gorm:"type:text" json:"todoId,omitempty"`
	ProjectPath       string  `gorm:"type:text;not null;index" json:"projectPath"`
	WorkspaceName     *string `gorm:"type:text" json:"workspaceName,omitempty"`
	CliTool           string  `gorm:"type:text;not null;default:'claude'" json:"cliTool"`
	Status            string  `gorm:"type:text;not null;default:'pending';index" json:"status"` // pending|running|waiting|completed|failed
	Progress          int     `gorm:"not null;default:0" json:"progress"`
	CompletionSummary *string   `gorm:"type:text" json:"completionSummary,omitempty"`
	ExitCode          *int      `json:"exitCode,omitempty"`
	SortOrder         int       `gorm:"not null;default:0" json:"sortOrder"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// 显式 TableName：避免依赖 GORM 复数化推断（LaunchHistory 会被错误地复数成
// "launch_histories"）。表名与 ModelDescriptor / cc-pane 原表名严格一致。
func (Project) TableName() string         { return "projects" }
func (LaunchHistory) TableName() string   { return "launch_history" }
func (TerminalSession) TableName() string { return "terminal_sessions" }
func (TaskBinding) TableName() string     { return "task_bindings" }
func (Provider) TableName() string        { return "providers" }
