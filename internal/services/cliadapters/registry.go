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

// Default 返回注册表，仅含 Claude / Reclaude / Codex 三个适配器（用户决策 2026-05-30：
// 其余 5 个 CLI——Gemini/Kimi/GLM/OpenCode/Cursor——已完全移除）。
//
// 注册顺序 = 前端启动菜单 / DetectAll 的展示顺序（Reclaude 紧跟 Claude）。
func Default() *Registry {
	r := NewRegistry()
	r.Register(NewClaude())
	r.Register(NewReclaude())
	r.Register(NewCodex())
	return r
}
