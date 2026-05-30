// Package launchhistory 记录与读取「启动历史」（launch_history 表）。
//
// 移植 cc-pane LaunchHistoryService 的子集：Record（每次启动写一条）+ List（最近 N 条，倒序）。
// P3b 补 resume 相关字段（ResumeSessionID / LastPrompt / ProviderID / LaunchCwd）。cc-pane 的
// 轮询 backfill（touch_by_session_id / update_last_prompt）属 P3b 拍板点 9，留后续。
//
// 铁律 / 要点：
//   - 路径字段 D7：ProjectPath = PTY cwd（启动目标），原样存。WorkspacePath 仅多目录
//     CLI 拼 --add-dir 用，可空。LaunchCwd 为运行时观测 cwd，resume 优先用它。
//   - LaunchedAt 是手填字段且用于倒序排序：必须存 UTC，否则跨时区/DST 偏移不同会让
//     字典序 TEXT 排序倒挂（与 created_at 同类问题，见 storage UTC NowFunc 修复）。
//   - LastPrompt（敏感）落库**必 cryptox.EncryptString**（service 层加解密，仿 provider，
//     非 GORM 钩子）：Record 写前加密、List 读后解密。ResumeSessionID **明文**（CLI resume 必需）。
package launchhistory

import (
	"context"
	"time"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
	"xkpane/internal/utils/logx"

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

// RecordSpec 是一次启动的记录输入。可选字段用指针表达 NULL。
type RecordSpec struct {
	ProjectID     string  `json:"projectId"`
	ProjectName   string  `json:"projectName"`
	ProjectPath   string  `json:"projectPath"` // = PTY cwd（铁律 D7）
	CliTool       string  `json:"cliTool"`     // 'none' | 适配器 id
	RuntimeKind   string  `json:"runtimeKind"` // local | wsl | ssh；缺省 local
	WorkspaceName *string `json:"workspaceName,omitempty"`
	WorkspacePath *string `json:"workspacePath,omitempty"` // 仅 --add-dir 用（铁律）

	// resume（P3b）：从历史会话恢复时填。ResumeSessionID 明文；LastPrompt 写前 cryptox 加密。
	ResumeSessionID *string `json:"resumeSessionId,omitempty"`
	LastPrompt      *string `json:"lastPrompt,omitempty"`
	ProviderID      *string `json:"providerId,omitempty"`
	LaunchCwd       *string `json:"launchCwd,omitempty"` // 运行时观测 cwd；resume 优先用它
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
	// LastPrompt 敏感：落库前 cryptox 加密（service 层加解密，仿 provider）。
	lastPrompt := spec.LastPrompt
	if lastPrompt != nil {
		enc, err := cryptox.EncryptString(*lastPrompt)
		if err != nil {
			return storage.LaunchHistory{}, err
		}
		lastPrompt = &enc
	}

	rec := storage.LaunchHistory{
		ProjectID:       spec.ProjectID,
		ProjectName:     spec.ProjectName,
		ProjectPath:     spec.ProjectPath,
		CliTool:         cliTool,
		RuntimeKind:     runtimeKind,
		WorkspaceName:   spec.WorkspaceName,
		WorkspacePath:   spec.WorkspacePath,
		ResumeSessionID: spec.ResumeSessionID, // 明文
		LastPrompt:      lastPrompt,           // 密文
		ProviderID:      spec.ProviderID,
		LaunchCwd:       spec.LaunchCwd,
		LaunchedAt:      time.Now().UTC(), // 手填字段：必须 UTC，保字典序==绝对时序
	}
	if err := s.db().WithContext(ctx).Create(&rec).Error; err != nil {
		return storage.LaunchHistory{}, err
	}
	// 返回给调用方的是明文（与 List 一致：DTO 不外泄密文）。
	rec.LastPrompt = spec.LastPrompt
	return rec, nil
}

// Delete 按 id 删除一条启动记录（幂等：不存在不报错）。
func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.db().WithContext(ctx).Delete(&storage.LaunchHistory{}, id).Error
}

// Clear 清空所有启动记录。
func (s *Service) Clear(ctx context.Context) error {
	return s.db().WithContext(ctx).Where("1 = 1").Delete(&storage.LaunchHistory{}).Error
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
	// LastPrompt 读后解密（密文 → 明文给前端）。单条失败不阻断整列表：置空 + 记日志。
	for i := range out {
		if out[i].LastPrompt == nil {
			continue
		}
		plain, err := cryptox.DecryptString(*out[i].LastPrompt)
		if err != nil {
			logx.For("launchhistory").Warn("decrypt lastPrompt failed", "id", out[i].ID, "err", err)
			out[i].LastPrompt = nil
			continue
		}
		out[i].LastPrompt = &plain
	}
	return out, nil
}
