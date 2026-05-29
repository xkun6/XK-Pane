// Package launchhistory 记录与读取「启动历史」（launch_history 表）。
//
// 移植 cc-pane LaunchHistoryService 的 MVP 子集：Record（每次启动写一条）+
// List（最近 N 条，倒序）。cc-pane 的会话恢复相关方法（update_session_started /
// touch_by_session_id / update_last_prompt / find_by_pty_session_id 等）属 Phase 4
// 会话恢复，本单元不含。
//
// 铁律 / 要点：
//   - 路径字段 D7：ProjectPath = PTY cwd（启动目标），原样存。WorkspacePath 仅多目录
//     CLI 拼 --add-dir 用，可空。LaunchCwd（运行时观测）属 Phase 5，本单元不写。
//   - LaunchedAt 是手填字段且用于倒序排序：必须存 UTC，否则跨时区/DST 偏移不同会让
//     字典序 TEXT 排序倒挂（与 created_at 同类问题，见 storage UTC NowFunc 修复）。
//   - LastPrompt（敏感）当前无写入方（MVP 启动无 prompt），保持 NULL；待 prompt 启动 /
//     会话恢复落地时加 cryptox 钩子（Phase 1 既定 TODO）。
package launchhistory

import (
	"context"
	"time"

	"xkpane/internal/storage"

	"gorm.io/gorm"
)

type Service struct {
	holder *storage.Holder
}

func New(holder *storage.Holder) *Service {
	return &Service{holder: holder}
}

func (s *Service) db() *gorm.DB {
	return s.holder.Current().GORM
}

// RecordSpec 是一次启动的记录输入（MVP 字段集）。可选字段用指针表达 NULL。
type RecordSpec struct {
	ProjectID     string  `json:"projectId"`
	ProjectName   string  `json:"projectName"`
	ProjectPath   string  `json:"projectPath"` // = PTY cwd（铁律 D7）
	CliTool       string  `json:"cliTool"`     // 'none' | 适配器 id
	RuntimeKind   string  `json:"runtimeKind"` // local | wsl | ssh；缺省 local
	WorkspaceName *string `json:"workspaceName,omitempty"`
	WorkspacePath *string `json:"workspacePath,omitempty"` // 仅 --add-dir 用（铁律）
}

// Record 写入一条启动历史，返回创建出的记录（含自增 ID 与 LaunchedAt）。
func (s *Service) Record(ctx context.Context, spec RecordSpec) (storage.LaunchHistory, error) {
	cliTool := spec.CliTool
	if cliTool == "" {
		cliTool = "none"
	}
	runtimeKind := spec.RuntimeKind
	if runtimeKind == "" {
		runtimeKind = "local"
	}
	rec := storage.LaunchHistory{
		ProjectID:     spec.ProjectID,
		ProjectName:   spec.ProjectName,
		ProjectPath:   spec.ProjectPath,
		CliTool:       cliTool,
		RuntimeKind:   runtimeKind,
		WorkspaceName: spec.WorkspaceName,
		WorkspacePath: spec.WorkspacePath,
		LaunchedAt:    time.Now().UTC(), // 手填字段：必须 UTC，保字典序==绝对时序
	}
	if err := s.db().WithContext(ctx).Create(&rec).Error; err != nil {
		return storage.LaunchHistory{}, err
	}
	return rec, nil
}

// List 返回最近的启动记录，按 launched_at 倒序（最新在前）。limit<=0 表示不限。
// 空结果返回非 nil 空切片（JSON []）。
func (s *Service) List(ctx context.Context, limit int) ([]storage.LaunchHistory, error) {
	out := make([]storage.LaunchHistory, 0)
	q := s.db().WithContext(ctx).Order("launched_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
