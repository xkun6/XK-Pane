package cliadapters

// Registry 是 CLI 工具注册表。保持注册顺序，供前端按固定顺序列出工具。
type Registry struct {
	adapters map[string]Adapter
	order    []string
}

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]Adapter)}
}

// Register 注册一个适配器（id 取自 adapter.Info().ID）。重复 id 覆盖实现但不重复入序。
func (r *Registry) Register(a Adapter) {
	id := a.Info().ID
	if _, ok := r.adapters[id]; !ok {
		r.order = append(r.order, id)
	}
	r.adapters[id] = a
}

// Get 按 id 查找适配器；不存在返回 (nil, false)。
func (r *Registry) Get(id string) (Adapter, bool) {
	a, ok := r.adapters[id]
	return a, ok
}

// IDs 返回注册顺序的 id 列表（拷贝）。
func (r *Registry) IDs() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// ListTools 按注册顺序返回各工具静态元信息（未检测安装状态）。
func (r *Registry) ListTools() []ToolInfo {
	out := make([]ToolInfo, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.adapters[id].Info())
	}
	return out
}

// DetectAll 按注册顺序检测所有工具的安装状态与版本。
func (r *Registry) DetectAll() []ToolInfo {
	out := make([]ToolInfo, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.adapters[id].Detect())
	}
	return out
}

// Default 返回注册了全部 8 个适配器的注册表。
//
// 注册顺序严格对齐 cc-pane 生产注册点（src-tauri/src/lib.rs:920-927）：
// Claude → Reclaude → Codex → Gemini → Kimi → Glm → OpenCode → Cursor。
// 该顺序即前端工具列表 / DetectAll / 能力列表的展示顺序（Reclaude 紧跟 Claude）。
func Default() *Registry {
	r := NewRegistry()
	r.Register(NewClaude())
	r.Register(NewReclaude())
	r.Register(NewCodex())
	r.Register(NewGemini())
	r.Register(NewKimi())
	r.Register(NewGlm())
	r.Register(NewOpenCode())
	r.Register(NewCursor())
	return r
}
