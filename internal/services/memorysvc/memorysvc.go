// Package memorysvc 是「记忆（Memory）」子系统的 Wails service（移植 cc-memory crate）。
//
// 自包含 CRUD-over-SQLite（memories 表，app 级单库走 Holder）：Store / Search / List / Get /
// Update / Delete（软删）/ Stats。搜索走 **LIKE**（title/content/tags，对齐 cc-memory 的 LIKE
// 降级路径）；FTS5 虚表留可选增强未移植。tags 以 JSON 串存储，DTO 出入用 []string。
//
// **延后**（耦合受门控的终端 / AI 工作流，其上游触发未落地）：format_for_injection（注入终端）、
// 日报 DailyReport、云端 sync。
package memorysvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"xkpane/internal/storage"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var validScope = map[string]bool{"global": true, "workspace": true, "project": true, "session": true}

type Service struct {
	holder *storage.Holder
}

func New(holder *storage.Holder) *Service { return &Service{holder: holder} }

func (s *Service) db() *gorm.DB { return s.holder.Current().GORM }

// MemoryDTO 是前端可见的记忆视图（tags 为 []string，时间为 RFC3339）。
type MemoryDTO struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	Scope         string    `json:"scope"`
	Category      string    `json:"category"`
	Importance    int       `json:"importance"`
	WorkspaceName *string   `json:"workspaceName,omitempty"`
	ProjectPath   *string   `json:"projectPath,omitempty"`
	SessionID     *string   `json:"sessionId,omitempty"`
	Tags          []string  `json:"tags"`
	Source        string    `json:"source"`
	AccessCount   int       `json:"accessCount"`
	SyncStatus    string    `json:"syncStatus"`
	IsDeleted     bool      `json:"isDeleted"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	AccessedAt    time.Time `json:"accessedAt"`
}

// StoreMemoryInput 新建记忆（Title/Content 必填，其余可选走默认）。
type StoreMemoryInput struct {
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	Scope         *string   `json:"scope,omitempty"`
	Category      *string   `json:"category,omitempty"`
	Importance    *int      `json:"importance,omitempty"`
	WorkspaceName *string   `json:"workspaceName,omitempty"`
	ProjectPath   *string   `json:"projectPath,omitempty"`
	SessionID     *string   `json:"sessionId,omitempty"`
	Tags          *[]string `json:"tags,omitempty"`
	Source        *string   `json:"source,omitempty"`
}

// UpdateMemoryInput 编辑记忆（全部可选，patch）。
type UpdateMemoryInput struct {
	Title      *string   `json:"title,omitempty"`
	Content    *string   `json:"content,omitempty"`
	Category   *string   `json:"category,omitempty"`
	Importance *int      `json:"importance,omitempty"`
	Tags       *[]string `json:"tags,omitempty"`
}

// MemoryQuery 查询过滤（全部可选）。对齐 cc-memory MemoryQuery（FTS5 sort_by=relevance 退化为 LIKE 排序）。
type MemoryQuery struct {
	Search        *string   `json:"search,omitempty"`
	Scope         *string   `json:"scope,omitempty"`
	Category      *string   `json:"category,omitempty"`
	MinImportance *int      `json:"minImportance,omitempty"`
	WorkspaceName *string   `json:"workspaceName,omitempty"`
	ProjectPath   *string   `json:"projectPath,omitempty"`
	SessionID     *string   `json:"sessionId,omitempty"`
	Tags          *[]string `json:"tags,omitempty"`
	FromDate      *string   `json:"fromDate,omitempty"`
	ToDate        *string   `json:"toDate,omitempty"`
	SortBy        *string   `json:"sortBy,omitempty"` // created_at|updated_at|importance|relevance
	Limit         *int      `json:"limit,omitempty"`
	Offset        *int      `json:"offset,omitempty"`
}

// MemoryQueryResult 分页查询结果。
type MemoryQueryResult struct {
	Items   []MemoryDTO `json:"items"`
	Total   int         `json:"total"`
	HasMore bool        `json:"hasMore"`
}

// MemoryStats 按维度计数。
type MemoryStats struct {
	Total      int            `json:"total"`
	ByScope    map[string]int `json:"byScope"`
	ByCategory map[string]int `json:"byCategory"`
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

func toDTO(m storage.Memory) MemoryDTO {
	return MemoryDTO{
		ID: m.ID, Title: m.Title, Content: m.Content, Scope: m.Scope, Category: m.Category,
		Importance: m.Importance, WorkspaceName: m.WorkspaceName, ProjectPath: m.ProjectPath,
		SessionID: m.SessionID, Tags: unmarshalTags(m.Tags), Source: m.Source, AccessCount: m.AccessCount,
		SyncStatus: m.SyncStatus, IsDeleted: m.IsDeleted,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, AccessedAt: m.AccessedAt,
	}
}

func orStr(p *string, def string) string {
	if p != nil && *p != "" {
		return *p
	}
	return def
}

// Store 新建记忆。校验（标题非空 ≤200、内容非空）+ scope 约束（workspace/project/session 需对应字段）
// + importance 默认 3 钳 1-5 + 默认 scope=project / category=fact / source=user。对齐 cc-memory store。
func (s *Service) Store(ctx context.Context, in StoreMemoryInput) (MemoryDTO, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return MemoryDTO{}, errors.New("memorysvc: title cannot be empty")
	}
	if len([]rune(title)) > 200 {
		return MemoryDTO{}, errors.New("memorysvc: title must be <= 200 chars")
	}
	if strings.TrimSpace(in.Content) == "" {
		return MemoryDTO{}, errors.New("memorysvc: content cannot be empty")
	}
	scope := orStr(in.Scope, "project")
	if !validScope[scope] {
		return MemoryDTO{}, fmt.Errorf("memorysvc: invalid scope %q", scope)
	}
	switch scope {
	case "workspace":
		if in.WorkspaceName == nil || *in.WorkspaceName == "" {
			return MemoryDTO{}, errors.New("memorysvc: workspace scope requires workspaceName")
		}
	case "project":
		if in.ProjectPath == nil || *in.ProjectPath == "" {
			return MemoryDTO{}, errors.New("memorysvc: project scope requires projectPath")
		}
	case "session":
		if in.SessionID == nil || *in.SessionID == "" {
			return MemoryDTO{}, errors.New("memorysvc: session scope requires sessionId")
		}
	}
	importance := 3
	if in.Importance != nil {
		importance = *in.Importance
	}
	if importance < 1 {
		importance = 1
	} else if importance > 5 {
		importance = 5
	}
	now := time.Now().UTC()
	m := storage.Memory{
		ID: uuid.NewString(), Title: title, Content: in.Content, Scope: scope,
		Category: orStr(in.Category, "fact"), Importance: importance,
		WorkspaceName: in.WorkspaceName, ProjectPath: in.ProjectPath, SessionID: in.SessionID,
		Tags: "[]", Source: orStr(in.Source, "user"),
		SyncStatus: "local_only", CreatedAt: now, UpdatedAt: now, AccessedAt: now,
	}
	if in.Tags != nil {
		m.Tags = marshalTags(*in.Tags)
	}
	if err := s.db().WithContext(ctx).Create(&m).Error; err != nil {
		return MemoryDTO{}, err
	}
	return toDTO(m), nil
}

// applyFilters 把 query 的过滤条件（不含 search）加到 db（参数化，避免注入）。is_deleted=0 由调用方加。
func applyFilters(db *gorm.DB, q MemoryQuery) *gorm.DB {
	if q.Scope != nil && *q.Scope != "" {
		db = db.Where("scope = ?", *q.Scope)
	}
	if q.Category != nil && *q.Category != "" {
		db = db.Where("category = ?", *q.Category)
	}
	if q.MinImportance != nil {
		db = db.Where("importance >= ?", *q.MinImportance)
	}
	if q.WorkspaceName != nil && *q.WorkspaceName != "" {
		db = db.Where("workspace_name = ?", *q.WorkspaceName)
	}
	if q.ProjectPath != nil && *q.ProjectPath != "" {
		db = db.Where("project_path = ?", *q.ProjectPath)
	}
	if q.SessionID != nil && *q.SessionID != "" {
		db = db.Where("session_id = ?", *q.SessionID)
	}
	if q.FromDate != nil && *q.FromDate != "" {
		db = db.Where("created_at >= ?", *q.FromDate)
	}
	if q.ToDate != nil && *q.ToDate != "" {
		db = db.Where("created_at < ?", *q.ToDate)
	}
	if q.Tags != nil {
		for _, tag := range *q.Tags {
			db = db.Where("tags LIKE ?", "%\""+tag+"\"%")
		}
	}
	return db
}

// Search 过滤 + 排序 + 分页查询（软删过滤）。有 search 词则 LIKE（title/content/tags），排序按
// importance/updated_at（对齐 cc-memory LIKE 路径）；无 search 词则按 sortBy。limit 默认 20。
func (s *Service) Search(ctx context.Context, q MemoryQuery) (MemoryQueryResult, error) {
	hasSearch := q.Search != nil && strings.TrimSpace(*q.Search) != ""

	base := func() *gorm.DB {
		db := applyFilters(s.db().WithContext(ctx).Model(&storage.Memory{}).Where("is_deleted = ?", false), q)
		if hasSearch {
			p := "%" + escapeLike(*q.Search) + "%"
			db = db.Where("title LIKE ? ESCAPE '\\' OR content LIKE ? ESCAPE '\\' OR tags LIKE ? ESCAPE '\\'", p, p, p)
		}
		return db
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return MemoryQueryResult{}, err
	}

	order := "updated_at DESC"
	if hasSearch {
		order = "importance DESC, updated_at DESC" // LIKE 路径固定排序（无 FTS5 rank）
	} else if q.SortBy != nil {
		switch *q.SortBy {
		case "importance":
			order = "importance DESC, updated_at DESC"
		case "created_at":
			order = "created_at DESC"
		case "updated_at":
			order = "updated_at DESC"
		}
	}

	limit := 20
	if q.Limit != nil {
		limit = *q.Limit
	}
	offset := 0
	if q.Offset != nil {
		offset = *q.Offset
	}

	var rows []storage.Memory
	if err := base().Order(order).Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return MemoryQueryResult{}, err
	}
	items := make([]MemoryDTO, 0, len(rows))
	for _, m := range rows {
		items = append(items, toDTO(m))
	}
	return MemoryQueryResult{Items: items, Total: int(total), HasMore: offset+len(items) < int(total)}, nil
}

// List 无搜索词的过滤列表（scope/workspace/project + 分页）。对齐 cc-memory list（包成 Search）。
func (s *Service) List(ctx context.Context, scope, workspaceName, projectPath *string, limit, offset *int) (MemoryQueryResult, error) {
	return s.Search(ctx, MemoryQuery{
		Scope: scope, WorkspaceName: workspaceName, ProjectPath: projectPath, Limit: limit, Offset: offset,
	})
}

// Get 按 id 取（软删的视为不存在）；命中则 touch（accessed_at + access_count，不 bump updated_at）。
func (s *Service) Get(ctx context.Context, id string) (*MemoryDTO, error) {
	var m storage.Memory
	err := s.db().WithContext(ctx).Where("id = ? AND is_deleted = ?", id, false).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// touch：UpdateColumns 跳过 GORM 自动 updated_at（touch 不算内容变更）。
	_ = s.db().WithContext(ctx).Model(&storage.Memory{}).Where("id = ?", id).
		UpdateColumns(map[string]any{"accessed_at": time.Now().UTC(), "access_count": gorm.Expr("access_count + 1")}).Error
	dto := toDTO(m)
	return &dto, nil
}

// Update patch 记忆（title/content/category/importance/tags）。校验 title ≤200 / importance 1-5。
// 无匹配（含已软删）报错。
func (s *Service) Update(ctx context.Context, id string, in UpdateMemoryInput) error {
	updates := map[string]any{}
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if t == "" {
			return errors.New("memorysvc: title cannot be empty")
		}
		if len([]rune(t)) > 200 {
			return errors.New("memorysvc: title must be <= 200 chars")
		}
		updates["title"] = t
	}
	if in.Content != nil {
		updates["content"] = *in.Content
	}
	if in.Category != nil {
		updates["category"] = *in.Category
	}
	if in.Importance != nil {
		if *in.Importance < 1 || *in.Importance > 5 {
			return errors.New("memorysvc: importance must be 1-5")
		}
		updates["importance"] = *in.Importance
	}
	if in.Tags != nil {
		updates["tags"] = marshalTags(*in.Tags)
	}
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&storage.Memory{}).Where("id = ? AND is_deleted = ?", id, false).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.New("memorysvc: memory not found")
		}
		if len(updates) == 0 {
			return nil
		}
		return tx.Model(&storage.Memory{}).Where("id = ?", id).Updates(updates).Error // Updates 自动 bump updated_at
	})
}

// Delete 软删（is_deleted=1 + bump updated_at）。无匹配（含已删）报错。对齐 cc-memory delete。
func (s *Service) Delete(ctx context.Context, id string) error {
	res := s.db().WithContext(ctx).Model(&storage.Memory{}).
		Where("id = ? AND is_deleted = ?", id, false).Update("is_deleted", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("memorysvc: memory not found")
	}
	return nil
}

// Stats 统计：total + byScope + byCategory（软删过滤；workspace/project 可选限定）。对齐 cc-memory stats。
func (s *Service) Stats(ctx context.Context, workspaceName, projectPath *string) (MemoryStats, error) {
	applyScope := func(db *gorm.DB) *gorm.DB {
		db = db.Where("is_deleted = ?", false)
		if workspaceName != nil && *workspaceName != "" {
			db = db.Where("workspace_name = ?", *workspaceName)
		}
		if projectPath != nil && *projectPath != "" {
			db = db.Where("project_path = ?", *projectPath)
		}
		return db
	}
	out := MemoryStats{ByScope: map[string]int{}, ByCategory: map[string]int{}}

	var total int64
	if err := applyScope(s.db().WithContext(ctx).Model(&storage.Memory{})).Count(&total).Error; err != nil {
		return out, err
	}
	out.Total = int(total)

	groupCount := func(col string, dst map[string]int) error {
		type kv struct {
			K string `gorm:"column:k"`
			C int    `gorm:"column:c"`
		}
		var rows []kv
		if err := applyScope(s.db().WithContext(ctx).Model(&storage.Memory{})).
			Select(col + " as k, COUNT(*) as c").Group(col).Scan(&rows).Error; err != nil {
			return err
		}
		for _, r := range rows {
			dst[r.K] = r.C
		}
		return nil
	}
	if err := groupCount("scope", out.ByScope); err != nil {
		return out, err
	}
	if err := groupCount("category", out.ByCategory); err != nil {
		return out, err
	}
	return out, nil
}

// escapeLike 转义 LIKE 通配符（% _ \），配合 ESCAPE '\\' 使搜索词被当字面量。
func escapeLike(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '%' || c == '_' || c == '\\' {
			out = append(out, '\\')
		}
		out = append(out, c)
	}
	return string(out)
}
