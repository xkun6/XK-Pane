package storage

import "time"

// 本文件定义「文件浏览器」域的持久化表。
//
// RecentDirectory 记录用户在文件浏览器里「打开过的根目录」，供文件页的「最近」
// 快捷入口展示。路径字段同样遵循 D7 铁律：原样存储，绝不规范化。

func init() {
	AllModels = append(AllModels,
		ModelDescriptor{Model: &RecentDirectory{}, TableName: "recent_directories", LabelKey: "recentDirectories", Clearable: true},
	)
}

// RecentDirectory 文件浏览器最近打开的目录（按 Path 唯一去重，原样存储不规范化）。
type RecentDirectory struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Path         string    `gorm:"type:text;not null;uniqueIndex" json:"path"`
	OpenCount    int       `gorm:"not null;default:0" json:"openCount"`
	LastOpenedAt time.Time `gorm:"index" json:"lastOpenedAt"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (RecentDirectory) TableName() string { return "recent_directories" }
