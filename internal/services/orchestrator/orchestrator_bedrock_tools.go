package orchestrator

// P6f：Bedrock 知识库 MCP 工具——让 pane 内的 Agent 经 MCP 只读查询网易版 Minecraft 中国版
// ModSDK 的 API / 事件 / 组件 / 模板 / 指南 / 规范 + 静态审查 + JSON/资源校验。忠实移植 cc-pane
// orchestrator_bedrock_tools.rs 的 bedrock_* 工具集（全部纯算法、无外网，由 bedrock.Service 兜底）。
//
// 返回约定（对齐既有 MCP 工具）：成功返回渲染好的 Markdown 文本；失败以「错误: …」文本作为正常
// 结果返回（不返回 Go error，否则 MCP 协议层判为工具调用失败）。Bedrock 依赖为 nil 时返回
// 「错误: Bedrock 知识库不可用」。cc-pane 用 isError 工具结果 `bedrock_<tool> failed: {e}`，
// 这里改用 XK 统一的「错误: <忠实英文消息>」前缀。
//
// 本单元（P6f-3）落地 Phase 2 高频查询 10 个工具；指南/模板/标准（P6f-4）、审查（P6f-5）、
// 校验（P6f-6）在后续单元追加注册并扩展 Bedrock 接口。

import (
	"context"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Bedrock 是 bedrock_* 工具依赖的查询能力（*bedrock.Service 结构性满足）。随单元增长：
// P6f-3 = Phase 2 的 10 个方法；P6f-4/5/6 再扩指南/模板/标准 + 审查 + 校验。
// nil = 全部 bedrock_* 工具返回「错误: Bedrock 知识库不可用」。
type Bedrock interface {
	SearchAPI(name string) (string, error)
	GetAPI(name string, side *string) (string, error)
	SearchEvent(name string) (string, error)
	GetEvent(name string, side *string) (string, error)
	ValidateCall(name, side string) (string, error)
	APIStats() (string, error)
	ListCategories() (string, error)
	ListApisByCategory(category string) (string, error)
	SearchComponent(query, componentType string) (string, error)
	GetComponent(componentID string) (string, error)
}

// registerBedrockTools 注册 Bedrock 只读知识库工具（由 registerTools 末尾调用）。
func (s *Service) registerBedrockTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_search_api",
		Description: "模糊搜索网易版 Minecraft 中国版 ModSDK 的 API（按名称，大小写不敏感子串）。返回最多 20 条排名结果及简短描述。",
	}, s.toolBedrockSearchAPI)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_get_api",
		Description: "按精确名称获取一个 ModSDK API 的完整 Markdown 卡片（参数/返回/示例/链接）。可选 side 过滤 client 或 server，省略则两侧都返回。",
	}, s.toolBedrockGetAPI)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_search_event",
		Description: "模糊搜索网易版 Minecraft 中国版 ModSDK 的事件（按名称）。返回最多 20 条排名结果。",
	}, s.toolBedrockSearchEvent)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_get_event",
		Description: "按精确名称获取一个 ModSDK 事件的完整 Markdown 卡片。可选 side 过滤 client 或 server。",
	}, s.toolBedrockGetEvent)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_validate_call",
		Description: "校验 (name, side) 是否为已知的 ModSDK API 或事件，返回 VALID/INVALID 及修正建议。side 为 client 或 server（必填）。",
	}, s.toolBedrockValidateCall)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_api_stats",
		Description: "知识库总量与 top API 分类统计（API/事件/header 计数、按 side 分布、前 10 大分类）。",
	}, s.toolBedrockAPIStats)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_list_categories",
		Description: "按字母序列出全部已知 ModSDK API 分类及各自 API 数量。",
	}, s.toolBedrockListCategories)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_list_apis_by_category",
		Description: "列出某分类下的全部 API。分类名可精确或子串，未命中时做大小写不敏感模糊匹配。",
	}, s.toolBedrockListApisByCategory)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_search_component",
		Description: "在组件 id / name / description 上做子串搜索。可选 componentType：item / block / entity / netease_item / netease_block / all（默认 all）。",
	}, s.toolBedrockSearchComponent)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_get_component",
		Description: "按精确 id 获取组件的完整定义卡片（含 JSON 定义）。",
	}, s.toolBedrockGetComponent)
}

// ── 参数结构（camelCase wire 名，对齐 cc-pane McpBedrock*Params）─────────────

type bedrockNameParams struct {
	Name string `json:"name" jsonschema:"API 或 event 名称（大小写不敏感子串匹配）"`
}

type bedrockGetAPIParams struct {
	Name string  `json:"name" jsonschema:"精确的 API/event 方法名"`
	Side *string `json:"side,omitempty" jsonschema:"可选 side 过滤：client 或 server；省略则两侧都返回"`
}

type bedrockValidateCallParams struct {
	Name string `json:"name" jsonschema:"API 或 event 名称"`
	Side string `json:"side" jsonschema:"side：client 或 server（必填）"`
}

type bedrockCategoryParams struct {
	Category string `json:"category" jsonschema:"分类名，可精确或模糊（子串）匹配"`
}

type bedrockSearchComponentParams struct {
	Query         string  `json:"query" jsonschema:"在 id / name / description 上子串匹配"`
	ComponentType *string `json:"componentType,omitempty" jsonschema:"组件类型过滤：item / block / entity / netease_item / netease_block / all（默认 all）"`
}

type bedrockGetComponentParams struct {
	ComponentID string `json:"componentId" jsonschema:"组件 id（精确匹配）"`
}

// ── 工具实现 ────────────────────────────────────────────────────────────

// bedrockResult 统一把 (text, err) 包成工具结果：err 非 nil → 「错误: <消息>」文本（非 Go error）。
func bedrockResult(text string, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return textResult("错误: " + err.Error()), nil, nil
	}
	return textResult(text), nil, nil
}

const bedrockUnavailable = "错误: Bedrock 知识库不可用"

func (s *Service) toolBedrockSearchAPI(_ context.Context, _ *mcp.CallToolRequest, in bedrockNameParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.SearchAPI(in.Name))
}

func (s *Service) toolBedrockGetAPI(_ context.Context, _ *mcp.CallToolRequest, in bedrockGetAPIParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.GetAPI(in.Name, in.Side))
}

func (s *Service) toolBedrockSearchEvent(_ context.Context, _ *mcp.CallToolRequest, in bedrockNameParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.SearchEvent(in.Name))
}

func (s *Service) toolBedrockGetEvent(_ context.Context, _ *mcp.CallToolRequest, in bedrockGetAPIParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.GetEvent(in.Name, in.Side))
}

func (s *Service) toolBedrockValidateCall(_ context.Context, _ *mcp.CallToolRequest, in bedrockValidateCallParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.ValidateCall(in.Name, in.Side))
}

func (s *Service) toolBedrockAPIStats(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.APIStats())
}

func (s *Service) toolBedrockListCategories(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.ListCategories())
}

func (s *Service) toolBedrockListApisByCategory(_ context.Context, _ *mcp.CallToolRequest, in bedrockCategoryParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.ListApisByCategory(in.Category))
}

func (s *Service) toolBedrockSearchComponent(_ context.Context, _ *mcp.CallToolRequest, in bedrockSearchComponentParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	// 默认 all 在工具层解析（对齐 cc-pane router 层 unwrap_or("all")）。
	componentType := "all"
	if in.ComponentType != nil {
		componentType = *in.ComponentType
	}
	return bedrockResult(s.deps.Bedrock.SearchComponent(in.Query, componentType))
}

func (s *Service) toolBedrockGetComponent(_ context.Context, _ *mcp.CallToolRequest, in bedrockGetComponentParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.GetComponent(in.ComponentID))
}
