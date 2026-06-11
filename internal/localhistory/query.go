package localhistory

import (
	"context"
)

// RecentChange 是跨文件的最近变更条目（含该版本若被某标签快照引用的标签名）。对齐 cc-pane。
type RecentChange struct {
	FilePath  string `json:"filePath"`
	VersionID string `json:"versionId"`
	Timestamp string `json:"timestamp"`
	Size      int64  `json:"size"`
	Hash      string `json:"hash"`
	LabelName string `json:"labelName"` // 空 = 该版本无关联标签
	Branch    string `json:"branch"`
}

// recentChangesSQL：最近变更跨文件查询。label_name 走相关子查询（取该版本被引用的首个标签名），
// COALESCE 成空串避免 NULL 无法 Scan 进 string。排序 created_at DESC + id DESC（id 含 ms+计数，
// 作为同秒 created_at 的确定性 tiebreak——本仓 created_at 为秒精度 RFC3339）。
const recentChangesSQL = `
SELECT fv.file_path AS file_path,
       fv.id AS version_id,
       fv.created_at AS timestamp,
       fv.size AS size,
       fv.hash AS hash,
       COALESCE((SELECT l.name FROM label_snapshots ls
                 JOIN labels l ON l.id = ls.label_id
                 WHERE ls.file_path = fv.file_path AND ls.version_id = fv.id
                 LIMIT 1), '') AS label_name,
       fv.branch AS branch
FROM file_versions fv
ORDER BY fv.created_at DESC, fv.id DESC
LIMIT ?`

// GetRecentChanges 返回最近的版本变更（跨文件，最新在前），带该版本若被标签引用的标签名。
// limit<=0 取默认 50。对齐 cc-pane get_recent_changes。
func (s *Store) GetRecentChanges(ctx context.Context, limit int) ([]RecentChange, error) {
	if limit <= 0 {
		limit = 50
	}
	out := make([]RecentChange, 0)
	err := s.gdb.WithContext(ctx).Raw(recentChangesSQL, limit).Scan(&out).Error
	return out, err
}

// ListDeletedFiles 列出「最新版本为删除标记」的文件（当前已删但有历史可恢复）。对齐 cc-pane。
func (s *Store) ListDeletedFiles(ctx context.Context) ([]FileVersion, error) {
	out := make([]FileVersion, 0)
	err := s.gdb.WithContext(ctx).
		Where("is_deleted = 1").
		Where("(file_path, id) IN (SELECT file_path, MAX(id) FROM file_versions GROUP BY file_path)").
		Order("created_at DESC, id DESC").
		Find(&out).Error
	return out, err
}
