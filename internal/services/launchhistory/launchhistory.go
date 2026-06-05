// Package launchhistory 记录与读取「启动历史」（launch_history 表）。
//
// 移植 cc-pane LaunchHistoryService 的 MVP 子集：Record（每次启动写一条）+
// List（最近 N 条，倒序）。cc-pane 的会话恢复相关方法（update_session_started /
// touch_by_session_id / update_last_prompt / find_by_pty_session_id 等）属后续
// 会话恢复，本单元不含。
//
// 铁律 / 要点：
//   - 路径字段 D7：ProjectPath = PTY cwd（启动目标），原样存。WorkspacePath 仅多目录
//     CLI 拼 --add-dir 用，可空。LaunchCwd（运行时观测）属后续，本单元不写。
//   - LaunchedAt 是手填字段且用于倒序排序：必须存 UTC，否则跨时区/DST 偏移不同会让
//     字典序 TEXT 排序倒挂（与 created_at 同类问题，见 storage UTC NowFunc 修复）。
//   - LastPrompt（敏感）当前无写入方（MVP 启动无 prompt），保持 NULL；待 prompt 启动 /
//     会话恢复落地时加 cryptox 钩子（Phase 1 既定 TODO）。
package launchhistory

import (
	"context"
	"errors"
	"strings"
	"time"

	"xkpane/internal/storage"

	"gorm.io/gorm"
)

const (
	// backfillMaxPolls × backfillInterval = 回填轮询总窗口（5 分钟）。codex 在用户首条消息后
	// 才落 session 文件、间隔不可预测，故窗口要够长；claude/reclaude 通常启动即落盘，早期即命中。
	backfillMaxPolls = 150
	backfillInterval = 2 * time.Second
	// backfillLaunchSlackSec：只回填「启动后 ≤ 此秒数」内记录的行，避免把发现的会话误配到
	// 同 project+cli 的一条陈旧未回填记录上（前端 record 与本次会话创建几乎同刻，故窗口取宽松 5min）。
	backfillLaunchSlackSec = 300
)

// ResumeDetector 据 cwd / cliTool 扫盘发现本次启动对应的可恢复会话 id（sessionscan.Service 实现）。
// 用接口而非直接依赖 sessionscan 包：保持 launchhistory 解耦、可单测注入假实现。
type ResumeDetector interface {
	DetectResumeSession(projectPath, workspacePath, cliTool, pinnedSessionID string, afterUnix int64) string
}

type Service struct {
	holder *storage.Holder
	// scan 注入的扫盘发现器；nil = 回填禁用（裸单测 / app 未接线）。
	scan ResumeDetector
}

func New(holder *storage.Holder) *Service {
	return &Service{holder: holder}
}

// SetResumeDetector 注入扫盘发现器（app 装配阶段调，传 sessionscan.Service）。nil 即不启用回填。
func (s *Service) SetResumeDetector(d ResumeDetector) { s.scan = d }

func (s *Service) db() *gorm.DB {
	return s.holder.Current().GORM
}

// RecordSpec 是一次启动的记录输入（MVP 字段集）。可选字段用指针表达 NULL。
type RecordSpec struct {
	ProjectID     string  `json:"projectId"`
	ProjectName   string  `json:"projectName"`
	ProjectPath   string  `json:"projectPath"` // = PTY cwd（铁律 D7）
	CliTool       string  `json:"cliTool"`     // 'none' | 适配器 id
	RuntimeKind   string  `json:"runtimeKind"` // local | ssh；缺省 local
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

// StartBackfill 由前端在会话创建成功后调用：后台轮询扫盘发现本次启动对应的可恢复会话 id，
// 写回最近一条匹配的启动记录（cli_tool+project_path，resume_session_id 仍空）。命中即停。
// 立即返回（goroutine 后台跑，绝不阻塞前端）；none/裸 shell / 未注入发现器时 no-op。
// 对齐 cc-pane start_launch_history_backfill：前端触发、后端轮询、回填 resume_session_id。
func (s *Service) StartBackfill(_ context.Context, projectPath, workspacePath, cliTool, ptySessionID string) error {
	if s.scan == nil {
		return nil
	}
	tool := strings.TrimSpace(cliTool)
	if tool == "" || tool == "none" {
		return nil
	}
	if strings.TrimSpace(projectPath) == "" || strings.TrimSpace(ptySessionID) == "" {
		return nil
	}
	after := time.Now().Unix()
	go s.runBackfill(projectPath, workspacePath, tool, ptySessionID, after)
	return nil
}

// runBackfill 轮询发现 → 回填；命中并写入即返回，否则最多轮询 backfillMaxPolls 次。
func (s *Service) runBackfill(projectPath, workspacePath, cliTool, ptySessionID string, afterUnix int64) {
	for i := 0; i < backfillMaxPolls; i++ {
		if resumeID := s.scan.DetectResumeSession(projectPath, workspacePath, cliTool, ptySessionID, afterUnix); resumeID != "" {
			if id, err := s.updateSessionStarted(projectPath, cliTool, ptySessionID, resumeID, afterUnix); err == nil && id != 0 {
				return // 已发现并回填 → 收工
			}
			// 发现了会话但记录尚未落库（前端 record 异步）→ 继续轮询直到记录就绪。
		}
		time.Sleep(backfillInterval)
	}
}

// updateSessionStarted 把发现的 resumeID 回填到最近一条匹配的启动记录（窗口内、resume 仍空）。
// 返回回填的记录 ID（0 = 未找到匹配行，调用方据此判断是否继续轮询）。
func (s *Service) updateSessionStarted(projectPath, cliTool, ptySessionID, resumeID string, afterUnix int64) (uint, error) {
	floor := time.Unix(afterUnix-backfillLaunchSlackSec, 0).UTC()
	var rec storage.LaunchHistory
	err := s.db().
		Where("project_path = ? AND cli_tool = ? AND resume_session_id IS NULL AND launched_at >= ?", projectPath, cliTool, floor).
		Order("launched_at DESC").
		First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if err := s.db().Model(&storage.LaunchHistory{}).Where("id = ?", rec.ID).Updates(map[string]any{
		"resume_session_id": resumeID,
		"pty_session_id":    ptySessionID,
		"claude_session_id": resumeID, // claude/reclaude 即转录 id；codex 存其 session id（恢复无需此列，无害）
	}).Error; err != nil {
		return 0, err
	}
	return rec.ID, nil
}
