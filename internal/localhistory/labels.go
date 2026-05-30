package localhistory

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Label 是标签元数据表（labels）。对齐 cc-pane labels 表。
type Label struct {
	ID        string `gorm:"primaryKey;column:id"`
	Name      string `gorm:"column:name"`
	LabelType string `gorm:"column:label_type"`       // "auto" | "manual"
	Source    string `gorm:"column:source"`           // git_commit|claude_session|user|build|restore|branch_switch
	Timestamp string `gorm:"column:timestamp;index"`  // RFC3339
	Branch    string `gorm:"column:branch;index"`     // 所属 git 分支（空 = 未知）
}

// TableName 固定表名。
func (Label) TableName() string { return "labels" }

// LabelSnapshot 是「标签 → 文件版本」关联表（label_snapshots）。复合主键 (LabelID, FilePath)。
// 不依赖 FK cascade（DeleteLabel 显式删两表），保持 modernc/glebarez 可移植。
type LabelSnapshot struct {
	LabelID   string `gorm:"primaryKey;column:label_id;index"`
	FilePath  string `gorm:"primaryKey;column:file_path"`
	VersionID string `gorm:"column:version_id"`
}

// TableName 固定表名。
func (LabelSnapshot) TableName() string { return "label_snapshots" }

// LabelFileSnapshot 是标签关联的「文件 → 版本」引用（也用作 GetAllLatestSnapshots 的结果）。
type LabelFileSnapshot struct {
	FilePath  string `json:"filePath"`
	VersionID string `json:"versionId"`
}

// HistoryLabel 是组合视图（标签元数据 + 文件快照列表）。对齐 cc-pane HistoryLabel。
type HistoryLabel struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	LabelType     string              `json:"labelType"` // "auto" | "manual"
	Source        string              `json:"source"`    // git_commit|claude_session|user|build|restore|branch_switch
	Timestamp     string              `json:"timestamp"` // RFC3339
	Branch        string              `json:"branch"`
	FileSnapshots []LabelFileSnapshot `json:"fileSnapshots"`
}

// PutLabel upsert 一个标签并**整体替换**其文件快照（事务）。对齐 cc-pane put_label。
func (s *Store) PutLabel(ctx context.Context, label HistoryLabel) error {
	return s.gdb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := Label{
			ID: label.ID, Name: label.Name, LabelType: label.LabelType,
			Source: label.Source, Timestamp: label.Timestamp, Branch: label.Branch,
		}
		// INSERT ... ON CONFLICT(id) DO UPDATE（等价 cc-pane 的 INSERT OR REPLACE）。
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Where("label_id = ?", label.ID).Delete(&LabelSnapshot{}).Error; err != nil {
			return err
		}
		for _, snap := range label.FileSnapshots {
			if err := tx.Create(&LabelSnapshot{
				LabelID: label.ID, FilePath: snap.FilePath, VersionID: snap.VersionID,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListLabels 列出所有标签（按 timestamp DESC, id），含各自文件快照。两次查询 + Go 端分组避免 N+1。
func (s *Store) ListLabels(ctx context.Context) ([]HistoryLabel, error) {
	var rows []Label
	if err := s.gdb.WithContext(ctx).Order("timestamp DESC, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	var snaps []LabelSnapshot
	if err := s.gdb.WithContext(ctx).Order("label_id, file_path").Find(&snaps).Error; err != nil {
		return nil, err
	}
	byLabel := make(map[string][]LabelFileSnapshot, len(rows))
	for _, sn := range snaps {
		byLabel[sn.LabelID] = append(byLabel[sn.LabelID], LabelFileSnapshot{FilePath: sn.FilePath, VersionID: sn.VersionID})
	}
	out := make([]HistoryLabel, 0, len(rows))
	for _, r := range rows {
		fs := byLabel[r.ID]
		if fs == nil {
			fs = []LabelFileSnapshot{}
		}
		out = append(out, HistoryLabel{
			ID: r.ID, Name: r.Name, LabelType: r.LabelType, Source: r.Source,
			Timestamp: r.Timestamp, Branch: r.Branch, FileSnapshots: fs,
		})
	}
	return out, nil
}

// DeleteLabel 删除标签及其快照（显式删两表，不依赖 FK cascade）。
func (s *Store) DeleteLabel(ctx context.Context, labelID string) error {
	return s.gdb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("label_id = ?", labelID).Delete(&LabelSnapshot{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", labelID).Delete(&Label{}).Error
	})
}

// GetAllLatestSnapshots 返回每个已跟踪文件的最新版本引用（创建「快照全部文件」的标签时用）。
// 对齐 cc-pane：SELECT file_path, MAX(id) GROUP BY file_path（id 字典序==时序，MAX 即最新）。
func (s *Store) GetAllLatestSnapshots(ctx context.Context) ([]LabelFileSnapshot, error) {
	out := make([]LabelFileSnapshot, 0)
	err := s.gdb.WithContext(ctx).Model(&FileVersion{}).
		Select("file_path", "MAX(id) as version_id").
		Group("file_path").
		Order("file_path").
		Scan(&out).Error
	return out, err
}
