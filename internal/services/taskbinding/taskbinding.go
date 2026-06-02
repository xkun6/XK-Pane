// Package taskbinding 管理编排任务绑定（task_bindings 表）的 CRUD + 查询。
//
// 一个 TaskBinding 记录一次「分派」：把某个 Todo / prompt 交给一个 CLI Agent 会话执行，
// 并跟踪其状态（pending→running→waiting→completed/failed）与进度。移植自 cc-pane
// TaskBindingService（薄 CRUD，无业务联动——状态推进由调用方/Agent 显式驱动）。
//
// 与 todo 服务一样持有 *storage.Holder（切数据路径后仍取活跃句柄），方法 ctx-first。
// 返回直接用 storage.TaskBinding（其 json tag 已是 camelCase + omitempty，与 cc-pane
// TaskBinding 序列化形态一致），无需额外 DTO。
package taskbinding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"xkpane/internal/storage"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var errNotFound = errors.New("TaskBinding does not exist")

// validStatuses 复刻 cc-pane TaskBindingStatus 枚举（snake_case）。
var validStatuses = map[string]bool{
	"pending": true, "running": true, "waiting": true, "completed": true, "failed": true,
}

type Service struct {
	holder *storage.Holder
}

func New(holder *storage.Holder) *Service {
	return &Service{holder: holder}
}

func (s *Service) db() *gorm.DB {
	return s.holder.Current().GORM
}

// CreateInput 新建编排任务（Title/ProjectPath 必填，其余可选）。对齐 cc-pane CreateTaskBindingRequest。
type CreateInput struct {
	Title         string  `json:"title"`
	Prompt        *string `json:"prompt,omitempty"`
	SessionID     *string `json:"sessionId,omitempty"`
	TodoID        *string `json:"todoId,omitempty"`
	ProjectPath   string  `json:"projectPath"`
	WorkspaceName *string `json:"workspaceName,omitempty"`
	CliTool       *string `json:"cliTool,omitempty"`
}

// UpdateInput patch 编排任务（全部可选，仅更新提供的字段）。对齐 cc-pane UpdateTaskBindingRequest。
type UpdateInput struct {
	Title             *string `json:"title,omitempty"`
	Prompt            *string `json:"prompt,omitempty"`
	SessionID         *string `json:"sessionId,omitempty"`
	Status            *string `json:"status,omitempty"`
	Progress          *int    `json:"progress,omitempty"`
	CompletionSummary *string `json:"completionSummary,omitempty"`
	ExitCode          *int    `json:"exitCode,omitempty"`
	SortOrder         *int    `json:"sortOrder,omitempty"`
}

// Query 查询过滤条件（全部可选）。对齐 cc-pane TaskBindingQuery。
type Query struct {
	Status        *string `json:"status,omitempty"`
	ProjectPath   *string `json:"projectPath,omitempty"`
	WorkspaceName *string `json:"workspaceName,omitempty"`
	Search        *string `json:"search,omitempty"` // title / prompt LIKE
	Limit         *int    `json:"limit,omitempty"`  // 缺省 50，封顶 200
	Offset        *int    `json:"offset,omitempty"` // 缺省 0
}

// QueryResult 分页查询结果。对齐 cc-pane TaskBindingQueryResult。
type QueryResult struct {
	Items   []storage.TaskBinding `json:"items"`
	Total   int                   `json:"total"`
	HasMore bool                  `json:"hasMore"`
}

// Create 新建一个编排任务绑定（服务端生成 uuid；status=pending、progress=0、cliTool 缺省 reclaude），
// 返回创建后的记录。title 去空后非空校验（对齐 cc-pane）。
func (s *Service) Create(ctx context.Context, in CreateInput) (storage.TaskBinding, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return storage.TaskBinding{}, errors.New("TaskBinding title cannot be empty")
	}
	cliTool := "reclaude" // cc-pane 缺省（非模型列默认 'claude'）：service 层语义优先
	if in.CliTool != nil && *in.CliTool != "" {
		cliTool = *in.CliTool
	}
	tb := storage.TaskBinding{
		ID:            uuid.NewString(),
		Title:         title,
		Prompt:        in.Prompt,
		SessionID:     in.SessionID,
		TodoID:        in.TodoID,
		ProjectPath:   in.ProjectPath,
		WorkspaceName: in.WorkspaceName,
		CliTool:       cliTool,
		Status:        "pending",
		Progress:      0,
		SortOrder:     0,
	}
	if err := s.db().WithContext(ctx).Create(&tb).Error; err != nil {
		return storage.TaskBinding{}, err
	}
	return tb, nil
}

// Get 按 id 取单条。不存在返回 (nil, nil)。
func (s *Service) Get(ctx context.Context, id string) (*storage.TaskBinding, error) {
	var tb storage.TaskBinding
	err := s.db().WithContext(ctx).First(&tb, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tb, nil
}

// FindBySession 按 session_id 取首个匹配（对齐 cc-pane find_by_session_id）。不存在返回 (nil, nil)。
func (s *Service) FindBySession(ctx context.Context, sessionID string) (*storage.TaskBinding, error) {
	var tb storage.TaskBinding
	err := s.db().WithContext(ctx).First(&tb, "session_id = ?", sessionID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tb, nil
}

// Update patch 一个编排任务（仅更新提供的字段，始终 bump updated_at），返回更新后的记录。
// title（若传）非空校验；progress（若传）须 0-100；status（若传）须是合法枚举。无匹配行报错。
// 空 patch = 仅校验存在性后返回现有记录（对齐 cc-pane：sets 空时 repo.update 返回 false，仍 get 回原值）。
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (storage.TaskBinding, error) {
	if in.Title != nil && strings.TrimSpace(*in.Title) == "" {
		return storage.TaskBinding{}, errors.New("TaskBinding title cannot be empty")
	}
	if in.Progress != nil && (*in.Progress < 0 || *in.Progress > 100) {
		return storage.TaskBinding{}, errors.New("Progress must be between 0 and 100")
	}
	if in.Status != nil && !validStatuses[*in.Status] {
		return storage.TaskBinding{}, fmt.Errorf("Invalid TaskBindingStatus: %s", *in.Status)
	}

	updates := map[string]any{}
	if in.Title != nil {
		updates["title"] = *in.Title
	}
	if in.Prompt != nil {
		updates["prompt"] = *in.Prompt
	}
	if in.SessionID != nil {
		updates["session_id"] = *in.SessionID
	}
	if in.Status != nil {
		updates["status"] = *in.Status
	}
	if in.Progress != nil {
		updates["progress"] = *in.Progress
	}
	if in.CompletionSummary != nil {
		updates["completion_summary"] = *in.CompletionSummary
	}
	if in.ExitCode != nil {
		updates["exit_code"] = *in.ExitCode
	}
	if in.SortOrder != nil {
		updates["sort_order"] = *in.SortOrder
	}

	if len(updates) > 0 {
		// Model(&TaskBinding{}).Updates(map) 在模型含 UpdatedAt 字段时自动 bump updated_at。
		if err := s.db().WithContext(ctx).Model(&storage.TaskBinding{}).
			Where("id = ?", id).Updates(updates).Error; err != nil {
			return storage.TaskBinding{}, err
		}
	}
	tb, err := s.Get(ctx, id)
	if err != nil {
		return storage.TaskBinding{}, err
	}
	if tb == nil {
		return storage.TaskBinding{}, errNotFound
	}
	return *tb, nil
}

// Delete 删除一个编排任务。无匹配行报错。
func (s *Service) Delete(ctx context.Context, id string) error {
	res := s.db().WithContext(ctx).Where("id = ?", id).Delete(&storage.TaskBinding{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errNotFound
	}
	return nil
}

// Query 动态过滤 + 分页查询编排任务，按 sort_order ASC, created_at DESC 排序。
// 对齐 cc-pane task_binding_repo.query（limit 缺省 50 封顶 200）。
func (s *Service) Query(ctx context.Context, q Query) (QueryResult, error) {
	applyConds := func(db *gorm.DB) *gorm.DB {
		if q.Status != nil && *q.Status != "" {
			db = db.Where("status = ?", *q.Status)
		}
		if q.ProjectPath != nil && *q.ProjectPath != "" {
			db = db.Where("project_path = ?", *q.ProjectPath)
		}
		if q.WorkspaceName != nil && *q.WorkspaceName != "" {
			db = db.Where("workspace_name = ?", *q.WorkspaceName)
		}
		if q.Search != nil && *q.Search != "" {
			p := "%" + *q.Search + "%"
			db = db.Where("title LIKE ? OR prompt LIKE ?", p, p)
		}
		return db
	}

	var total int64
	if err := applyConds(s.db().WithContext(ctx).Model(&storage.TaskBinding{})).Count(&total).Error; err != nil {
		return QueryResult{}, err
	}

	limit := 50
	if q.Limit != nil {
		limit = *q.Limit
	}
	if limit > 200 {
		limit = 200
	}
	offset := 0
	if q.Offset != nil {
		offset = *q.Offset
	}

	items := make([]storage.TaskBinding, 0)
	if err := applyConds(s.db().WithContext(ctx).Model(&storage.TaskBinding{})).
		Order("sort_order ASC, created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return QueryResult{}, err
	}
	return QueryResult{Items: items, Total: int(total), HasMore: offset+limit < int(total)}, nil
}
