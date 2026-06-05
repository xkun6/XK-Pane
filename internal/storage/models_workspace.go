package storage

import "time"

// 本文件定义 P3a「Workspace 两层树」的持久化模型。
//
// **偏离 cc-pane 说明**：
//   - cc-pane 每个 workspace 一个 <data>/workspaces/<name>/workspace.json，项目内联在
//     Workspace.projects: Vec<WorkspaceProject>（cc-panes-core/src/models/workspace.rs:50-77）。
//     XK 按 SQLite-only 铁律拆成两张关系表：workspaces + workspace_projects（外键 + sort_order）。
//   - cc-pane 的 workspace name 同时是磁盘目录名；XK 用 id(UUID) 主键，name 仅做显示 + 唯一约束，
//     无磁盘目录语义（故 Rename 只改列、不 fs::rename）。
//   - 忠实保留完整 schema（含暂不接 UI 的列：ProviderID / LaunchProfileID / Hidden /
//     DefaultEnvironment），这些是 AutoMigrate 友好的零成本映射；service / UI 只实现 P3a 真正
//     消费的部分（避免无调用方的投机方法）。
//   - SSH 嵌套配置（WorkspaceProject.SSH）：以 JSON 文本列存（同 AppSettings.CustomTheme 范式），
//     非空 = 该项目是「SSH 远程项目」，侧栏 globe 渲染 + 启动走 ssh 客户端；service 层
//     owns 序列化（见 workspace.AddSSHProject）。不含密码（仅 machineId 引用机器，密码由
//     sshmachine.ResolveForSSH 在启动时解密），故本列不敏感。

func init() {
	// 工作空间是组织结构，含项目归属/排序，非"可重建的缓存"→ Clearable:false（同 projects 档）。
	AllModels = append(AllModels,
		ModelDescriptor{Model: &Workspace{}, TableName: "workspaces", LabelKey: "workspaces", Clearable: false},
		ModelDescriptor{Model: &WorkspaceProject{}, TableName: "workspace_projects", LabelKey: "workspaceProjects", Clearable: false},
	)
}

// Workspace 是项目的组织容器（cc-pane 侧栏两层树的第一层）。
// Name 唯一；List 排序规则：Pinned 优先 → SortOrder 升序(NULL 垫底) → CreatedAt 升序。
type Workspace struct {
	ID                 string    `gorm:"primaryKey;size:36" json:"id"`
	Name               string    `gorm:"type:text;not null;uniqueIndex" json:"name"`
	Alias              *string   `gorm:"type:text" json:"alias,omitempty"`
	Path               *string   `gorm:"type:text" json:"path,omitempty"` // 工作空间根目录；可空
	ProviderID         *string   `gorm:"type:text;index" json:"providerId,omitempty"`
	LaunchProfileID    *string   `gorm:"type:text" json:"launchProfileId,omitempty"`
	DefaultEnvironment string    `gorm:"type:text;not null;default:'local'" json:"defaultEnvironment"` // local|ssh（P3a 仅 local）
	Pinned             bool      `gorm:"not null;default:false;index" json:"pinned"`
	Hidden             bool      `gorm:"not null;default:false" json:"hidden"`
	SortOrder          *int      `gorm:"index" json:"sortOrder,omitempty"` // NULL 垫底
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`

	// Projects 关联（供 List 预加载内联返回）；删除 workspace 时级联删 projects。
	Projects []WorkspaceProject `gorm:"foreignKey:WorkspaceID;constraint:OnDelete:CASCADE" json:"projects"`
}

// WorkspaceProject 是 Workspace 下的项目节点（树的第二层）。
// (WorkspaceID, Path) 联合唯一 = 同一工作空间内 path 去重（service 层归一化后存）。
// SSH 远程项目无本地 path，service 用合成键 `ssh://<machineId>/<remotePath>` 填 Path 满足唯一约束。
type WorkspaceProject struct {
	ID              string    `gorm:"primaryKey;size:36" json:"id"`
	WorkspaceID     string    `gorm:"type:text;not null;uniqueIndex:idx_ws_proj_path,priority:1" json:"workspaceId"`
	Path            string    `gorm:"type:text;not null;uniqueIndex:idx_ws_proj_path,priority:2" json:"path"` // 本地项目=原始路径；SSH 项目=合成键
	Alias           *string   `gorm:"type:text" json:"alias,omitempty"`
	SSH             *string   `gorm:"type:text" json:"ssh,omitempty"` // 非空=SSH 远程项目；JSON(SshProjectConfig)，service 层序列化
	LaunchProfileID *string   `gorm:"type:text" json:"launchProfileId,omitempty"`
	SortOrder       int       `gorm:"not null;default:0" json:"sortOrder"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// 显式 TableName 防 GORM 复数化推断（与 ModelDescriptor 严格一致）。
func (Workspace) TableName() string        { return "workspaces" }
func (WorkspaceProject) TableName() string { return "workspace_projects" }
