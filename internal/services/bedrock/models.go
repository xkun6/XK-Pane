// Package bedrock 是网易版 Minecraft（中国版 MC PE）ModSDK 知识库的只读查询引擎，
// 忠实移植 cc-pane cc-panes-core/src/services/bedrock_query。编译期内嵌 ~26MB 基线数据
// （1956 条 API + 76 组件 + 42 模板 + 1982 个 docs + 17 标准），首次查询惰性建索引，
// 为上层 26 个 bedrock_* MCP 只读工具提供 API/事件/组件/模板/指南/规范查询 + 静态审查 +
// JSON/资源校验。OTA 同步（bedrockdocsync 包）产出的数据经 SetDataRoot 覆盖内嵌基线。
//
// 全子系统纯算法、无外网、跨平台可编译（搜索=纯子串无 Levenshtein/BM25；CJK 不分词只按
// 空白切；review=纯行扫描无 AST）。XK 侧零额外业务逻辑，仅在 orchestrator 注册 MCP 工具。
//
// 本文件 = serde 数据模型直译（models.rs）。全字段缺失=零值，等价 serde(default)。
package bedrock

import "encoding/json"

// ApiEntry 对应 data/api-index.json 的一条 API / event / header 条目（顶层为数组）。
// 对齐 models.rs:10-39。rename：type→Kind、return→Returns（return 是 Go 关键字）。
type ApiEntry struct {
	// Kind 条目类型："api" | "event" | "header"。
	Kind string `json:"type"`
	Name string `json:"name"`
	// Method 拥有该 API 的组件上的方法名（仅 api 条目可能有）。
	Method string `json:"method"`
	// Side "server" | "client" | ""。
	Side        string       `json:"side"`
	Description string       `json:"description"`
	Remark      string       `json:"remark"`
	Params      []Param      `json:"params"`
	Returns     []ReturnSpec `json:"return"`
	Example     string       `json:"example"`
	Link        string       `json:"link"`
	AgentNote   string       `json:"agent_note"`
	Category    string       `json:"category"`
}

// Param 是 API 的参数规格（models.rs:42-50）。rename：type→Ty。
type Param struct {
	Name        string `json:"name"`
	Ty          string `json:"type"`
	Description string `json:"description"`
}

// ReturnSpec 是 API 的返回值规格（models.rs:53-59）。rename：type→Ty。
type ReturnSpec struct {
	Ty          string `json:"type"`
	Description string `json:"description"`
}

// ComponentEntry 对应 data/components-index.json（顶层数组）。
// 显式字段 id/type/name/description；其余键经 serde(flatten) 兜进 Extra —— get_component
// 渲染卡片会读 data_type/properties/values/example 等扩展字段，不能丢弃。
// Go 无 flatten → 自定义 UnmarshalJSON：先解 map[string]json.RawMessage，抽已知字段，
// 余下塞 Extra（对齐 models.rs:66-78）。
type ComponentEntry struct {
	ID          string
	Kind        string // 原 JSON 字段 "type"
	Name        string
	Description string
	Extra       map[string]json.RawMessage
}

// UnmarshalJSON 复刻 serde(flatten)：已知字段单独解，其余键全进 Extra。
// 已知字段类型不符时静默跳过（对齐 serde permissive 设计，避免上游 schema 漂移破坏加载）。
func (c *ComponentEntry) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*c = ComponentEntry{Extra: make(map[string]json.RawMessage)}
	for k, v := range raw {
		switch k {
		case "id":
			_ = json.Unmarshal(v, &c.ID)
		case "type":
			_ = json.Unmarshal(v, &c.Kind)
		case "name":
			_ = json.Unmarshal(v, &c.Name)
		case "description":
			_ = json.Unmarshal(v, &c.Description)
		default:
			c.Extra[k] = v
		}
	}
	return nil
}

// TemplateEntry 对应 data/templates-index.json（顶层数组，models.rs:87-122）。
// 多数字段强类型；apis（{components,methods}）/ systems（历史混 string+object）/
// file_count（{json,other,py}）保留为 json.RawMessage（对齐 serde_json::Value），避免过约束。
type TemplateEntry struct {
	ID              string          `json:"id"`
	ModName         string          `json:"mod_name"`
	Path            string          `json:"path"`
	BehaviorPack    string          `json:"behavior_pack"`
	ResourcePack    string          `json:"resource_pack"`
	Tags            []string        `json:"tags"`
	HasClient       bool            `json:"has_client"`
	HasUI           bool            `json:"has_ui"`
	HasCustomEntity bool            `json:"has_custom_entity"`
	HasCustomItems  bool            `json:"has_custom_items"`
	HasCustomBlocks bool            `json:"has_custom_blocks"`
	EventsUsed      []string        `json:"events_used"`
	Apis            json.RawMessage `json:"apis"`
	Systems         json.RawMessage `json:"systems"`
	FileCount       json.RawMessage `json:"file_count"`
}

// GuideEntry 由 scanGuides 走 data/docs 树构造（不反序列化，无 json tag，models.rs:128-137）。
type GuideEntry struct {
	Title string
	// Path 相对 data/docs/ 的路径，正斜杠。
	Path string
	// Source 顶层来源目录："mcguide" | "mconline" | "wiki" | "ui_templates"。
	Source     string
	Breadcrumb string
	Headings   []string
}
