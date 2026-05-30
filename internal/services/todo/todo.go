// Package todo 管理待办（todos + todo_subtasks 表）的 CRUD。
//
// 移植 cc-pane todo_repo 的**核心 CRUD 子集**：Create / Get / List / Update(patch) / Delete +
// 子任务 AddSubtask / UpdateSubtask / DeleteSubtask。**延后**（后续单元）：高级 query 过滤
// （按 scope/status/tags）、stats 统计、到期提醒 reminders、recurrence 轮转、reorder/batch。
// tags 以 JSON 数组字符串存储（模型 Todo.Tags），DTO 出入用 []string。
package todo

import (
	"context"
	"encoding/json"
	"errors"

	"xkpane/internal/storage"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var errNotFound = errors.New("Todo does not exist")

type Service struct {
	holder *storage.Holder
}

func New(holder *storage.Holder) *Service {
	return &Service{holder: holder}
}

func (s *Service) db() *gorm.DB {
	return s.holder.Current().GORM
}

// TodoDTO 是前端可见的待办视图（tags 为 []string，含子任务）。
type TodoDTO struct {
	ID          string                `json:"id"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	Status      string                `json:"status"`
	Priority    string                `json:"priority"`
	Scope       string                `json:"scope"`
	ScopeRef    *string               `json:"scopeRef,omitempty"`
	Tags        []string              `json:"tags"`
	DueDate     *string               `json:"dueDate,omitempty"`
	SortOrder   int                   `json:"sortOrder"`
	MyDay       bool                  `json:"myDay"`
	MyDayDate   *string               `json:"myDayDate,omitempty"`
	ReminderAt  *string               `json:"reminderAt,omitempty"`
	Recurrence  *string               `json:"recurrence,omitempty"`
	TodoType    string                `json:"todoType"`
	Subtasks    []storage.TodoSubtask `json:"subtasks"`
}

// CreateTodoInput 新建待办（仅 Title 必填，其余可选，缺省走模型默认）。
type CreateTodoInput struct {
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	Status      *string   `json:"status,omitempty"`
	Priority    *string   `json:"priority,omitempty"`
	Scope       *string   `json:"scope,omitempty"`
	ScopeRef    *string   `json:"scopeRef,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
	DueDate     *string   `json:"dueDate,omitempty"`
	ReminderAt  *string   `json:"reminderAt,omitempty"`
	Recurrence  *string   `json:"recurrence,omitempty"`
	TodoType    *string   `json:"todoType,omitempty"`
}

// UpdateTodoInput 编辑待办（全部可选，仅更新提供的字段）。
type UpdateTodoInput struct {
	Title       *string   `json:"title,omitempty"`
	Description *string   `json:"description,omitempty"`
	Status      *string   `json:"status,omitempty"`
	Priority    *string   `json:"priority,omitempty"`
	Scope       *string   `json:"scope,omitempty"`
	ScopeRef    *string   `json:"scopeRef,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
	DueDate     *string   `json:"dueDate,omitempty"`
	MyDay       *bool     `json:"myDay,omitempty"`
	MyDayDate   *string   `json:"myDayDate,omitempty"`
	ReminderAt  *string   `json:"reminderAt,omitempty"`
	Recurrence  *string   `json:"recurrence,omitempty"`
	TodoType    *string   `json:"todoType,omitempty"`
}

func marshalTags(tags []string) string {
	if tags == nil {
		return "[]"
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func unmarshalTags(s string) []string {
	out := make([]string, 0)
	if s == "" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

func toDTO(t storage.Todo) TodoDTO {
	subs := t.Subtasks
	if subs == nil {
		subs = []storage.TodoSubtask{}
	}
	return TodoDTO{
		ID: t.ID, Title: t.Title, Description: t.Description, Status: t.Status,
		Priority: t.Priority, Scope: t.Scope, ScopeRef: t.ScopeRef, Tags: unmarshalTags(t.Tags),
		DueDate: t.DueDate, SortOrder: t.SortOrder, MyDay: t.MyDay, MyDayDate: t.MyDayDate,
		ReminderAt: t.ReminderAt, Recurrence: t.Recurrence, TodoType: t.TodoType, Subtasks: subs,
	}
}

func orDefault(p *string, def string) string {
	if p != nil && *p != "" {
		return *p
	}
	return def
}

// Create 新建一个待办（服务端生成 uuid，sort_order 追加到末尾），返回创建后的 DTO。
func (s *Service) Create(ctx context.Context, in CreateTodoInput) (TodoDTO, error) {
	if in.Title == "" {
		return TodoDTO{}, errors.New("title is required")
	}
	var maxOrder int
	s.db().WithContext(ctx).Model(&storage.Todo{}).Select("COALESCE(MAX(sort_order), -1)").Scan(&maxOrder)

	t := storage.Todo{
		ID:          uuid.NewString(),
		Title:       in.Title,
		Status:      orDefault(in.Status, "todo"),
		Priority:    orDefault(in.Priority, "medium"),
		Scope:       orDefault(in.Scope, "global"),
		ScopeRef:    in.ScopeRef,
		Tags:        "[]",
		DueDate:     in.DueDate,
		SortOrder:   maxOrder + 1,
		ReminderAt:  in.ReminderAt,
		Recurrence:  in.Recurrence,
		TodoType:    orDefault(in.TodoType, ""),
	}
	if in.Description != nil {
		t.Description = *in.Description
	}
	if in.Tags != nil {
		t.Tags = marshalTags(*in.Tags)
	}
	if err := s.db().WithContext(ctx).Create(&t).Error; err != nil {
		return TodoDTO{}, err
	}
	return toDTO(t), nil
}

// Get 按 id 查（含子任务）。不存在返回 (nil, nil)。
func (s *Service) Get(ctx context.Context, id string) (*TodoDTO, error) {
	var t storage.Todo
	err := s.db().WithContext(ctx).Preload("Subtasks", subtaskOrder).First(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dto := toDTO(t)
	return &dto, nil
}

// List 列出全部待办（含子任务），按 sort_order、created_at 升序。
func (s *Service) List(ctx context.Context) ([]TodoDTO, error) {
	var rows []storage.Todo
	if err := s.db().WithContext(ctx).Preload("Subtasks", subtaskOrder).
		Order("sort_order, created_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]TodoDTO, 0, len(rows))
	for _, t := range rows {
		out = append(out, toDTO(t))
	}
	return out, nil
}

func subtaskOrder(db *gorm.DB) *gorm.DB { return db.Order("sort_order, created_at") }

// Update patch 一个待办（仅更新提供的字段）。无匹配行报错。
func (s *Service) Update(ctx context.Context, id string, in UpdateTodoInput) error {
	updates := map[string]any{}
	if in.Title != nil {
		updates["title"] = *in.Title
	}
	if in.Description != nil {
		updates["description"] = *in.Description
	}
	if in.Status != nil {
		updates["status"] = *in.Status
	}
	if in.Priority != nil {
		updates["priority"] = *in.Priority
	}
	if in.Scope != nil {
		updates["scope"] = *in.Scope
	}
	if in.ScopeRef != nil {
		updates["scope_ref"] = *in.ScopeRef
	}
	if in.Tags != nil {
		updates["tags"] = marshalTags(*in.Tags)
	}
	if in.DueDate != nil {
		updates["due_date"] = *in.DueDate
	}
	if in.MyDay != nil {
		updates["my_day"] = *in.MyDay
	}
	if in.MyDayDate != nil {
		updates["my_day_date"] = *in.MyDayDate
	}
	if in.ReminderAt != nil {
		updates["reminder_at"] = *in.ReminderAt
	}
	if in.Recurrence != nil {
		updates["recurrence"] = *in.Recurrence
	}
	if in.TodoType != nil {
		updates["todo_type"] = *in.TodoType
	}
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&storage.Todo{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errNotFound
		}
		if len(updates) == 0 {
			return nil
		}
		return tx.Model(&storage.Todo{}).Where("id = ?", id).Updates(updates).Error
	})
}

// Delete 删除待办及其子任务（显式删两表，不依赖 FK cascade）。无匹配行报错。
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", id).Delete(&storage.Todo{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errNotFound
		}
		return tx.Where("todo_id = ?", id).Delete(&storage.TodoSubtask{}).Error
	})
}

// AddSubtask 给某待办加一个子任务（sort_order 追加到末尾），返回创建后的子任务。
func (s *Service) AddSubtask(ctx context.Context, todoID, title string) (storage.TodoSubtask, error) {
	if title == "" {
		return storage.TodoSubtask{}, errors.New("subtask title is required")
	}
	var count int64
	if err := s.db().WithContext(ctx).Model(&storage.Todo{}).Where("id = ?", todoID).Count(&count).Error; err != nil {
		return storage.TodoSubtask{}, err
	}
	if count == 0 {
		return storage.TodoSubtask{}, errNotFound
	}
	var maxOrder int
	s.db().WithContext(ctx).Model(&storage.TodoSubtask{}).Where("todo_id = ?", todoID).
		Select("COALESCE(MAX(sort_order), -1)").Scan(&maxOrder)

	sub := storage.TodoSubtask{
		ID: uuid.NewString(), TodoID: todoID, Title: title, SortOrder: maxOrder + 1,
	}
	if err := s.db().WithContext(ctx).Create(&sub).Error; err != nil {
		return storage.TodoSubtask{}, err
	}
	return sub, nil
}

// UpdateSubtask patch 一个子任务（title / completed 可选）。无匹配行报错。
func (s *Service) UpdateSubtask(ctx context.Context, id string, title *string, completed *bool) error {
	updates := map[string]any{}
	if title != nil {
		updates["title"] = *title
	}
	if completed != nil {
		updates["completed"] = *completed
	}
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&storage.TodoSubtask{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errNotFound
		}
		if len(updates) == 0 {
			return nil
		}
		return tx.Model(&storage.TodoSubtask{}).Where("id = ?", id).Updates(updates).Error
	})
}

// DeleteSubtask 删除一个子任务。无匹配行报错。
func (s *Service) DeleteSubtask(ctx context.Context, id string) error {
	res := s.db().WithContext(ctx).Where("id = ?", id).Delete(&storage.TodoSubtask{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errNotFound
	}
	return nil
}
