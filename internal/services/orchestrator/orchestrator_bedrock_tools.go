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
	// P6f-4：指南 / 模板 / 标准（12）。
	SearchGuide(keyword string, source *string) (string, error)
	ReadGuide(path string, maxLines *uint32) (string, error)
	MatchTemplate(tags []string, keyword string) (string, error)
	GetTemplateInfo(templateID string) (string, error)
	GetTemplateFile(templateID, filePath string) (string, error)
	GetStandard(name string) (string, error)
	GetExpertGuide() (string, error)
	GetReviewRules() (string, error)
	GetOptimizationRules() (string, error)
	GetJSONSpec(contentType *string) (string, error)
	BedrockHelp() (string, error)
	ListModPacks() (string, error)
	// P6f-5：静态代码审查（1）。
	ReviewCode(code string, filename *string) (string, error)
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

	// ── Phase 3：指南 / 模板 / 标准（12，P6f-4）─────────────────────────
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_search_guide",
		Description: "多 token 子串搜索内嵌文档（mcguide / mconline / wiki / ui_templates）。可选 source 限定单一来源。返回最多 20 条排名结果。中文关键词建议用单个核心词。",
	}, s.toolBedrockSearchGuide)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_read_guide",
		Description: "读取一个内嵌 Markdown 指南。path 相对 data/docs/（用 bedrock_search_guide 返回的路径）。可选 maxLines 限制起始行数（默认 200，最大 2000）。",
	}, s.toolBedrockReadGuide)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_match_template",
		Description: "按 tags / keyword 对官方示例模板打分排名。tags 与 keyword 至少给一个。",
	}, s.toolBedrockMatchTemplate)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_get_template_info",
		Description: "获取单个模板的完整元数据卡片（mod / 路径 / 标签 / 标志 / 系统 / API）。",
	}, s.toolBedrockGetTemplateInfo)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_get_template_file",
		Description: "读取模板目录内的某个文件（当前模板源未内嵌，通常返回提示改用 bedrock_get_template_info）。",
	}, s.toolBedrockGetTemplateFile)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_get_standard",
		Description: "按名称（文件 stem，可带或不带 .md）获取任意 standards/*.md 规范文档，支持精确或模糊匹配。",
	}, s.toolBedrockGetStandard)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_get_expert_guide",
		Description: "返回 modsdk-expert.md 专家指南全文。",
	}, s.toolBedrockGetExpertGuide)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_get_review_rules",
		Description: "返回 netease-code-review.md 代码审查规则全文。",
	}, s.toolBedrockGetReviewRules)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_get_optimization_rules",
		Description: "返回 netease-optimization-expert.md 优化专家规则全文。",
	}, s.toolBedrockGetOptimizationRules)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_get_json_spec",
		Description: "JSON 编写规范。contentType：netease（仅网易）/ modsdk（仅 ModSDK）/ both 或省略（两者拼接）。",
	}, s.toolBedrockGetJSONSpec)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_help",
		Description: "Bedrock 知识库全部工具的速览说明。",
	}, s.toolBedrockHelp)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_list_mod_packs",
		Description: "按 mod_name 分组列出全部内嵌模板，渲染 Markdown 汇总。",
	}, s.toolBedrockListModPacks)

	// ── Phase 4：静态代码审查（1，P6f-5）───────────────────────────────
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bedrock_review_code",
		Description: "对一段 Python 2.7 代码跑 ModSDK 静态审查规则，返回 Markdown 报告（按 Critical/Warning/Suggestion 分级）。可选 filename 用于报告标题（默认 unknown.py）。",
	}, s.toolBedrockReviewCode)
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

// ── Phase 3 参数结构（P6f-4）─────────────────────────────────────────────

type bedrockSearchGuideParams struct {
	Keyword string  `json:"keyword" jsonschema:"空白分隔的多 token 关键词查询（中文连续串当单 token）"`
	Source  *string `json:"source,omitempty" jsonschema:"可选来源过滤：mcguide / mconline / wiki / ui_templates"`
}

type bedrockReadGuideParams struct {
	Path     string  `json:"path" jsonschema:"内嵌指南路径（如 mcguide/foo.md），由 bedrock_search_guide 返回"`
	MaxLines *uint32 `json:"maxLines,omitempty" jsonschema:"从开头最多读取的行数（默认 200，最大 2000）"`
}

type bedrockMatchTemplateParams struct {
	Tags    []string `json:"tags,omitempty" jsonschema:"可选标签列表（如 [\"ui\", \"dimension\"]）"`
	Keyword string   `json:"keyword,omitempty" jsonschema:"可选自由文本关键词"`
}

type bedrockTemplateIDParams struct {
	TemplateID string `json:"templateId" jsonschema:"模板 id（如 ui_demo）"`
}

type bedrockGetTemplateFileParams struct {
	TemplateID string `json:"templateId" jsonschema:"模板 id"`
	FilePath   string `json:"filePath" jsonschema:"模板目录内的相对文件路径"`
}

type bedrockGetStandardParams struct {
	Name string `json:"name" jsonschema:"规范文档名（文件 stem，可带或不带 .md，精确或模糊）"`
}

type bedrockGetJSONSpecParams struct {
	ContentType *string `json:"contentType,omitempty" jsonschema:"可选内容类型：netease / modsdk / both（默认 both）"`
}

// ── Phase 3 工具实现（P6f-4）────────────────────────────────────────────

func (s *Service) toolBedrockSearchGuide(_ context.Context, _ *mcp.CallToolRequest, in bedrockSearchGuideParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.SearchGuide(in.Keyword, in.Source))
}

func (s *Service) toolBedrockReadGuide(_ context.Context, _ *mcp.CallToolRequest, in bedrockReadGuideParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.ReadGuide(in.Path, in.MaxLines))
}

func (s *Service) toolBedrockMatchTemplate(_ context.Context, _ *mcp.CallToolRequest, in bedrockMatchTemplateParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.MatchTemplate(in.Tags, in.Keyword))
}

func (s *Service) toolBedrockGetTemplateInfo(_ context.Context, _ *mcp.CallToolRequest, in bedrockTemplateIDParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.GetTemplateInfo(in.TemplateID))
}

func (s *Service) toolBedrockGetTemplateFile(_ context.Context, _ *mcp.CallToolRequest, in bedrockGetTemplateFileParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.GetTemplateFile(in.TemplateID, in.FilePath))
}

func (s *Service) toolBedrockGetStandard(_ context.Context, _ *mcp.CallToolRequest, in bedrockGetStandardParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.GetStandard(in.Name))
}

func (s *Service) toolBedrockGetExpertGuide(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.GetExpertGuide())
}

func (s *Service) toolBedrockGetReviewRules(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.GetReviewRules())
}

func (s *Service) toolBedrockGetOptimizationRules(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.GetOptimizationRules())
}

func (s *Service) toolBedrockGetJSONSpec(_ context.Context, _ *mcp.CallToolRequest, in bedrockGetJSONSpecParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.GetJSONSpec(in.ContentType))
}

func (s *Service) toolBedrockHelp(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.BedrockHelp())
}

func (s *Service) toolBedrockListModPacks(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.ListModPacks())
}

// ── Phase 4 参数 + 工具（P6f-5）─────────────────────────────────────────

type bedrockReviewCodeParams struct {
	Code     string  `json:"code" jsonschema:"要审查的 Python 2.7 源代码"`
	Filename *string `json:"filename,omitempty" jsonschema:"报告标题用的文件名（默认 unknown.py）"`
}

func (s *Service) toolBedrockReviewCode(_ context.Context, _ *mcp.CallToolRequest, in bedrockReviewCodeParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bedrock == nil {
		return textResult(bedrockUnavailable), nil, nil
	}
	return bedrockResult(s.deps.Bedrock.ReviewCode(in.Code, in.Filename))
}
