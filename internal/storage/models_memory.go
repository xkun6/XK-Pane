package storage

import "time"

// 本文件移植 cc-memory crate 的 memories 表 GORM 模型（记忆子系统）。
//
// 偏离 cc-memory：时间戳用 Go 原生 time.Time（GORM 自管 created/updated，accessed 手动 touch），
// 不沿用 cc-memory 的 RFC3339 文本列（与本仓其它表一致，JSON 序列化仍是 RFC3339）。
// search 的 FTS5 虚表 + 触发器**不移植**（service 层走 LIKE，FTS5 留可选增强）。
// tags 以 JSON 数组字符串存储（同 Todo），DTO 出入用 []string。云端预留字段（user_id /
// sync_status / sync_version）保留但本阶段恒为默认（local_only）。

func init() {
	AllModels = append(AllModels,
		ModelDescriptor{Model: &Memory{}, TableName: "memories", LabelKey: "memories", Clearable: true},
	)
}

// Memory 记忆条目（cc-memory 移植；跨 scope：global / workspace / project / session）。
type Memory struct {
	ID         string `gorm:"primaryKey;size:36" json:"id"`
	Title      string `gorm:"type:text;not null;index" json:"title"`
	Content    string `gorm:"type:text;not null" json:"content"`
	Scope      string `gorm:"type:text;not null;default:'project';index" json:"scope"` // global|workspace|project|session
	Category   string `gorm:"type:text;not null;default:'fact';index" json:"category"` // decision|lesson|preference|pattern|fact|plan|<custom>
	Importance int    `gorm:"not null;default:3" json:"importance"`                    // 1-5

	WorkspaceName *string `gorm:"type:text;index" json:"workspaceName,omitempty"`
	ProjectPath   *string `gorm:"type:text;index" json:"projectPath,omitempty"`
	SessionID     *string `gorm:"type:text;index" json:"sessionId,omitempty"`

	Tags        string `gorm:"type:text;not null;default:'[]'" json:"tags"`     // JSON 数组
	Source      string `gorm:"type:text;not null;default:'user'" json:"source"` // user|agent|mcp|hook
	AccessCount int    `gorm:"not null;default:0" json:"accessCount"`

	// 云端预留（本阶段恒为默认）。
	UserID      *string `gorm:"type:text" json:"userId,omitempty"`
	SyncStatus  string  `gorm:"type:text;not null;default:'local_only'" json:"syncStatus"`
	SyncVersion int64   `gorm:"not null;default:0" json:"syncVersion"`
	IsDeleted   bool    `gorm:"not null;default:false;index" json:"isDeleted"` // 软删

	CreatedAt  time.Time `gorm:"index" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"index" json:"updatedAt"`
	AccessedAt time.Time `json:"accessedAt"` // Get 时 touch（手动，不 bump updated_at）
}

// TableName 固定表名（与 ModelDescriptor / cc-memory 原表名一致）。
func (Memory) TableName() string { return "memories" }
