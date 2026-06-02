// Package todo 管理待办（todos + todo_subtasks 表）的 CRUD + 查询 + 统计。
//
// 移植 cc-pane todo_repo：核心 CRUD（Create / Get / List / Update(patch) / Delete + 子任务
// AddSubtask / UpdateSubtask / DeleteSubtask）+ 高级 Query（动态过滤 + 排序 + 分页）+ Stats
// （按 status/scope/priority 分组计数 + 过期数）+ Reorder（拖拽重排 sort_order）+
// BatchUpdateStatus（多选批量改状态）+ recurrence 轮转（done 跃迁时按周期克隆下一实例）。
// **延后**（后续单元）：到期提醒 reminders 的实际触发。
// tags 以 JSON 数组字符串存储（模型 Todo.Tags），DTO 出入用 []string。
package todo

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

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
	// 标题去空后判空（对齐 cc-pane create_todo：trim 后存、纯空白拒），落库 trim 后值。
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return TodoDTO{}, errors.New("title is required")
	}
	var maxOrder int
	s.db().WithContext(ctx).Model(&storage.Todo{}).Select("COALESCE(MAX(sort_order), -1)").Scan(&maxOrder)

	t := storage.Todo{
		ID:          uuid.NewString(),
		Title:       title,
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
		// 对齐 cc-pane update_todo：title 若提供则 trim 后非空校验，存 trim 后值。
		tt := strings.TrimSpace(*in.Title)
		if tt == "" {
			return errors.New("title is required")
		}
		updates["title"] = tt
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
		// 取旧记录：既做存在性校验，也供 done 跃迁判断 + recurrence 轮转克隆字段（对齐 cc-pane update_todo）。
		var old storage.Todo
		if err := tx.First(&old, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errNotFound
			}
			return err
		}
		if len(updates) > 0 {
			if err := tx.Model(&storage.Todo{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return err
			}
		}
		// 重复任务：状态变为 done（旧态非 done）且有非空 recurrence 时，在同事务内创建下一实例。
		becomingDone := in.Status != nil && *in.Status == "done" && old.Status != "done"
		if becomingDone && old.Recurrence != nil && *old.Recurrence != "" {
			return s.createNextRecurrence(tx, old, *old.Recurrence)
		}
		return nil
	})
}

// createNextRecurrence 按 recurrence 规则克隆 old 为下一个待办实例（status=todo，due/reminder 前推）。
// 移植 cc-pane todo_service.create_next_recurrence：复制 title/description/priority/scope/scopeRef/
// tags/todoType/recurrence，due_date 按周期前推，reminder_at 保持与 due 相同偏移。在调用方事务内执行。
func (s *Service) createNextRecurrence(tx *gorm.DB, old storage.Todo, recurrence string) error {
	nextDue := nextDueDate(old.DueDate, recurrence)
	nextReminder := nextReminderAt(old.DueDate, old.ReminderAt, nextDue)

	var maxOrder int
	tx.Model(&storage.Todo{}).Select("COALESCE(MAX(sort_order), -1)").Scan(&maxOrder)

	rec := recurrence
	next := storage.Todo{
		ID:          uuid.NewString(),
		Title:       old.Title,
		Description: old.Description,
		Status:      "todo",
		Priority:    old.Priority,
		Scope:       old.Scope,
		ScopeRef:    old.ScopeRef,
		Tags:        old.Tags, // 已是 JSON 数组串，直接复制
		DueDate:     nextDue,
		SortOrder:   maxOrder + 1,
		ReminderAt:  nextReminder,
		Recurrence:  &rec,
		TodoType:    old.TodoType,
	}
	return tx.Create(&next).Error
}

// nextDueDate 把 due 的日期部分按 recurrence 滚到下一周期（daily +1d / weekly +1w / monthly +1月，
// 月末按目标月最后一天钳制——对齐 cc-pane checked_add_months 而非 Go AddDate 的溢出归一）。
// due 为空或不可解析为 YYYY-MM-DD 则返回 nil。
func nextDueDate(due *string, recurrence string) *string {
	if due == nil || *due == "" {
		return nil
	}
	ds := *due
	if len(ds) >= 10 {
		ds = ds[:10] // 容忍带时间部分的 ISO 8601
	}
	d, err := time.Parse("2006-01-02", ds)
	if err != nil {
		return nil
	}
	var next time.Time
	switch recurrence {
	case "weekly":
		next = d.AddDate(0, 0, 7)
	case "monthly":
		next = addMonthsClamped(d, 1)
	default: // daily 及未知规则
		next = d.AddDate(0, 0, 1)
	}
	out := next.Format("2006-01-02")
	return &out
}

// addMonthsClamped 给 d 加 n 个月，目标月没有该日（如 1/31 → 2 月）时钳制到目标月最后一天，
// 避免 Go time.AddDate 的溢出归一（1/31 + 1月 = 3/3 会跳过 2 月）。
func addMonthsClamped(d time.Time, n int) time.Time {
	y, m, day := d.Date()
	firstTarget := time.Date(y, m, 1, 0, 0, 0, 0, d.Location()).AddDate(0, n, 0)
	ty, tm, _ := firstTarget.Date()
	lastDay := time.Date(ty, tm+1, 0, 0, 0, 0, 0, d.Location()).Day() // 目标月下月的第 0 天 = 目标月最后一天
	if day > lastDay {
		day = lastDay
	}
	return time.Date(ty, tm, day, 0, 0, 0, 0, d.Location())
}

// nextReminderAt 让下一实例的 reminder_at 保持与 due_date 相同的偏移量（next_due@00:00:00 UTC + offset）。
// due/reminder 任一为空或不可解析为 RFC3339、或 nextDue 为空时返回 nil（对齐 cc-pane）。
func nextReminderAt(due, reminder, nextDue *string) *string {
	if due == nil || reminder == nil || nextDue == nil {
		return nil
	}
	dueDt, err1 := time.Parse(time.RFC3339, *due)
	remDt, err2 := time.Parse(time.RFC3339, *reminder)
	if err1 != nil || err2 != nil {
		return nil
	}
	nd, err := time.Parse("2006-01-02", *nextDue)
	if err != nil {
		return nil
	}
	offset := remDt.Sub(dueDt)
	base := time.Date(nd.Year(), nd.Month(), nd.Day(), 0, 0, 0, 0, time.UTC)
	out := base.Add(offset).Format(time.RFC3339)
	return &out
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
	title = strings.TrimSpace(title)
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

// TodoQuery 是查询过滤条件（全部可选，nil/空串 = 不约束）。对齐 cc-pane TodoQuery。
type TodoQuery struct {
	Status   *string `json:"status,omitempty"`
	Priority *string `json:"priority,omitempty"`
	Scope    *string `json:"scope,omitempty"`
	ScopeRef *string `json:"scopeRef,omitempty"`
	Search   *string `json:"search,omitempty"` // 标题 / 描述 LIKE
	Tag      *string `json:"tag,omitempty"`    // tags JSON 含该精确标签
	SortBy   *string `json:"sortBy,omitempty"` // created_at|updated_at|priority|due_date，缺省 sort_order
	Limit    *int    `json:"limit,omitempty"`  // 缺省 50
	Offset   *int    `json:"offset,omitempty"` // 缺省 0
	MyDay    *bool   `json:"myDay,omitempty"`  // true = 仅今日 My Day
	TodoType *string `json:"todoType,omitempty"`
}

// TodoQueryResult 是分页查询结果。对齐 cc-pane TodoQueryResult。
type TodoQueryResult struct {
	Items   []TodoDTO `json:"items"`
	Total   int       `json:"total"`
	HasMore bool      `json:"hasMore"`
}

// TodoStats 是按维度的计数统计。对齐 cc-pane TodoStats。
type TodoStats struct {
	Total      int            `json:"total"`
	ByStatus   map[string]int `json:"byStatus"`
	ByScope    map[string]int `json:"byScope"`
	ByPriority map[string]int `json:"byPriority"`
	Overdue    int            `json:"overdue"` // due_date < now 且 status != done
}

// Query 动态过滤 + 排序 + 分页查询待办（含子任务），返回结果 + 总数 + 是否还有更多。
// 对齐 cc-pane todo_repo.query。条件构造抽成闭包，count 与 data 各起一条独立查询（避免 GORM
// 复用同一 Statement 把 SELECT count(*) 带进 data 查询）。
func (s *Service) Query(ctx context.Context, q TodoQuery) (TodoQueryResult, error) {
	applyConds := func(db *gorm.DB) *gorm.DB {
		if q.Status != nil && *q.Status != "" {
			db = db.Where("status = ?", *q.Status)
		}
		if q.Priority != nil && *q.Priority != "" {
			db = db.Where("priority = ?", *q.Priority)
		}
		if q.Scope != nil && *q.Scope != "" {
			db = db.Where("scope = ?", *q.Scope)
		}
		if q.ScopeRef != nil && *q.ScopeRef != "" {
			db = db.Where("scope_ref = ?", *q.ScopeRef)
		}
		if q.Search != nil && *q.Search != "" {
			p := "%" + *q.Search + "%"
			db = db.Where("title LIKE ? OR description LIKE ?", p, p)
		}
		if q.Tag != nil && *q.Tag != "" {
			db = db.Where("tags LIKE ?", "%\""+*q.Tag+"\"%") // tags 是 JSON 数组串，匹配带引号的精确标签
		}
		if q.TodoType != nil && *q.TodoType != "" {
			db = db.Where("todo_type = ?", *q.TodoType)
		}
		if q.MyDay != nil && *q.MyDay {
			db = db.Where("my_day = 1 AND my_day_date = ?", time.Now().Format("2006-01-02")) // 本地日期，对齐 cc-pane chrono::Local
		}
		return db
	}

	var total int64
	if err := applyConds(s.db().WithContext(ctx).Model(&storage.Todo{})).Count(&total).Error; err != nil {
		return TodoQueryResult{}, err
	}

	order := "sort_order ASC, created_at DESC"
	if q.SortBy != nil {
		switch *q.SortBy {
		case "created_at":
			order = "created_at DESC"
		case "updated_at":
			order = "updated_at DESC"
		case "priority":
			order = "CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 END ASC, sort_order ASC"
		case "due_date":
			order = "due_date IS NULL, due_date ASC, sort_order ASC" // SQLite 无 NULLS LAST，用 IS NULL 排序模拟
		}
	}

	limit := 50
	if q.Limit != nil {
		limit = *q.Limit
	}
	offset := 0
	if q.Offset != nil {
		offset = *q.Offset
	}

	var rows []storage.Todo
	if err := applyConds(s.db().WithContext(ctx).Model(&storage.Todo{})).
		Preload("Subtasks", subtaskOrder).Order(order).Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return TodoQueryResult{}, err
	}
	items := make([]TodoDTO, 0, len(rows))
	for _, t := range rows {
		items = append(items, toDTO(t))
	}
	return TodoQueryResult{Items: items, Total: int(total), HasMore: offset+limit < int(total)}, nil
}

// Stats 统计待办：总数 + 按 status/scope/priority 分组计数 + 过期数。scope/scopeRef 非空则限定范围。
// 对齐 cc-pane todo_repo.stats。
func (s *Service) Stats(ctx context.Context, scope *string, scopeRef *string) (TodoStats, error) {
	applyScope := func(db *gorm.DB) *gorm.DB {
		if scope != nil && *scope != "" {
			db = db.Where("scope = ?", *scope)
		}
		if scopeRef != nil && *scopeRef != "" {
			db = db.Where("scope_ref = ?", *scopeRef)
		}
		return db
	}
	out := TodoStats{ByStatus: map[string]int{}, ByScope: map[string]int{}, ByPriority: map[string]int{}}

	var total int64
	if err := applyScope(s.db().WithContext(ctx).Model(&storage.Todo{})).Count(&total).Error; err != nil {
		return out, err
	}
	out.Total = int(total)

	// col 仅取固定字面量（status/scope/priority），非用户输入 —— 无注入面。
	groupCount := func(col string, dst map[string]int) error {
		type kv struct {
			K string `gorm:"column:k"`
			C int    `gorm:"column:c"`
		}
		var rows []kv
		if err := applyScope(s.db().WithContext(ctx).Model(&storage.Todo{})).
			Select(col + " as k, COUNT(*) as c").Group(col).Scan(&rows).Error; err != nil {
			return err
		}
		for _, r := range rows {
			dst[r.K] = r.C
		}
		return nil
	}
	if err := groupCount("status", out.ByStatus); err != nil {
		return out, err
	}
	if err := groupCount("scope", out.ByScope); err != nil {
		return out, err
	}
	if err := groupCount("priority", out.ByPriority); err != nil {
		return out, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var overdue int64
	if err := applyScope(s.db().WithContext(ctx).Model(&storage.Todo{})).
		Where("due_date IS NOT NULL AND due_date < ? AND status != 'done'", now).Count(&overdue).Error; err != nil {
		return out, err
	}
	out.Overdue = int(overdue)
	return out, nil
}

// Reorder 按给定 id 顺序把各待办的 sort_order 重写为其下标（i 即新次序），事务内逐条更新。
// 用 UpdateColumn **不触碰 updated_at**（重排是排序而非内容变更，对齐 cc-pane reorder）。
// ids 中不存在的 id 静默跳过（更新 0 行）。
func (s *Service) Reorder(ctx context.Context, ids []string) error {
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&storage.Todo{}).Where("id = ?", id).
				UpdateColumn("sort_order", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// BatchUpdateStatus 批量把 ids 的状态设为 status，返回实际更新的行数。用 Update（**自动 bump
// updated_at**，对齐 cc-pane batch_update_status）。空 ids 直接返回 0。
func (s *Service) BatchUpdateStatus(ctx context.Context, ids []string, status string) (int, error) {
	if status == "" {
		return 0, errors.New("status is required")
	}
	if len(ids) == 0 {
		return 0, nil
	}
	res := s.db().WithContext(ctx).Model(&storage.Todo{}).Where("id IN ?", ids).Update("status", status)
	if res.Error != nil {
		return 0, res.Error
	}
	return int(res.RowsAffected), nil
}
