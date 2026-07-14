package orchestrator

// P6e-6：MC PE 控制 MCP 工具（execute_mc_code / reload_mc / get_mc_pe_logs / list_mc_pe_runs）。
// 让 pane 内的 Agent 经 MCP 在正在跑的网易版 Minecraft 进程内 exec Python 2.7、热重载、读日志。
// 前三个忠实移植 cc-pane orchestrator_service.rs 的非 Win32 工具（截图/点击在 Windows 单元 P6e-7 补）；
// list_mc_pe_runs 为 XK 多开扩展：确定性地列出各客户端 runId 供其余工具定向。
//
// 这些工具只依赖 Bridge（TCP 发帧）与 MCPE（日志缓冲），跨平台编译；Bridge/MCPE 为 nil 时
// 按 cc-pane 约定返回「错误: …」文本结果（不返回 Go error，否则 MCP 协议层判为工具调用失败）。
//
// 显式偏离：execute_mc_code 成功文案由 cc-pane 的「用 get_session_output 看 PTY 日志」改为
// 「用 get_mc_pe_logs 看日志」——cc-pane 原文案与本工具集自相矛盾（MC PE 不挂在 terminal session
// 上，get_session_output 读不到），统一指向 get_mc_pe_logs（见 docs/archive/P6e_PLAN.md §1.4）。

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"xkpane/internal/services/mcdkbridge"
	"xkpane/internal/storage"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerMCPETools 注册 MC PE 控制工具（由 registerTools 调用）。
func (s *Service) registerMCPETools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "execute_mc_code",
		Description: "向 MC PE 桥模组发送一段 Python 2.7 代码并在游戏进程内 exec。isClient=true 在客户端执行，否则在服务端。fire-and-forget——执行结果通过 MC PE 日志观察（用 get_mc_pe_logs 读最近 N 行）。代码需遵守 ModSDK 规范：禁止 f-string / 必须用 print() 函数形式 / 禁止 type hints / async / 海象运算符。多开 MC PE 时必须传 runId 指定目标客户端（单开可省略，runId 用 list_mc_pe_runs 获取）。",
	}, s.toolExecuteMcCode)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "reload_mc",
		Description: "向 MC PE 桥模组发送热重载指令。kind: game=重启游戏世界 / addon_and_game=重载 addon 资源+世界 / all_shaders=重编译全部着色器 / single_shader=重编译指定着色器（kind=single_shader 时 fileName 必填）。fire-and-forget。多开 MC PE 时必须传 runId 指定目标客户端（单开可省略，runId 用 list_mc_pe_runs 获取）。",
	}, s.toolReloadMc)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_mc_pe_logs",
		Description: "读取 MC PE 运行的最近 N 行日志（环形缓冲，cap=2000）。返回纯文本，每行 `HH:MM:SS [Category] [LEVEL] message` 格式——内容跟 MC PE Panel 左下日志区一致（同样的 MC Studio 白名单过滤）。多开 MC PE 时不传 runId 会按客户端分节返回全部日志，传 runId 只看某一个（runId 用 list_mc_pe_runs 获取）。没有任何运行时返回错误。lines 默认 200，最大 2000。",
	}, s.toolGetMcPeLogs)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_mc_pe_runs",
		Description: "列出当前所有正在运行的 MC PE 客户端（runId / 包名 / pid / 调试桥连接状态）。多开 MC PE 时先调它拿 runId，再把 runId 传给 get_mc_pe_logs / execute_mc_code / reload_mc / capture_window / click_window 定向操作。",
	}, s.toolListMcPeRuns)
	s.registerMCPECaptureTools(srv) // capture_window / click_window 仅 Windows 注册（P6e-7）；非 Windows no-op
}

// ── 参数结构（camelCase，对齐 cc-pane McpExecuteMcCodeParams / McpReloadMcParams / McpGetMcPeLogsParams）──

type executeMcCodeParams struct {
	Code     string `json:"code" jsonschema:"要在游戏进程内 exec 的 Python 2.7 代码"`
	IsClient bool   `json:"isClient,omitempty" jsonschema:"是否在客户端执行（传 true 客户端，省略或 false 服务端）"`
	RunID    string `json:"runId,omitempty" jsonschema:"目标 MC PE 客户端的 runId（多开时必填；单开可省略）"`
}

type reloadMcParams struct {
	Kind     string `json:"kind" jsonschema:"重载类型：game / addon_and_game / all_shaders / single_shader"`
	FileName string `json:"fileName,omitempty" jsonschema:"要重编译的着色器文件名（kind 为 single_shader 时必填）"`
	RunID    string `json:"runId,omitempty" jsonschema:"目标 MC PE 客户端的 runId（多开时必填；单开可省略）"`
}

type getMcPeLogsParams struct {
	Lines int    `json:"lines,omitempty" jsonschema:"返回最近多少行（默认 200，最大 2000）"`
	RunID string `json:"runId,omitempty" jsonschema:"目标 MC PE 客户端的 runId（多开时必填；单开可省略）"`
}

// sendBridgePacket 决定 execute/reload 的目标客户端：显式 runId 直发；未指定且桥上
// 多于一个客户端时报错（宁可打断，不可发错人），至多一个时沿用旧的最新连接语义。
func (s *Service) sendBridgePacket(runID string, pkt mcdkbridge.Packet) error {
	if runID != "" {
		return s.deps.Bridge.SendPacketToRun(runID, pkt)
	}
	if len(s.deps.Bridge.ClientRunIDs()) > 1 {
		return fmt.Errorf("有多个 MC PE 客户端连着调试桥，请传 runId 指定目标（用 list_mc_pe_runs 查看各客户端 runId）")
	}
	return s.deps.Bridge.SendPacket(pkt)
}

// ── 工具实现 ────────────────────────────────────────────────────────────

func (s *Service) toolExecuteMcCode(_ context.Context, _ *mcp.CallToolRequest, in executeMcCodeParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bridge == nil {
		return textResult("错误: MCDK 调试桥不可用"), nil, nil
	}
	// fire-and-forget，不预查 connected（对齐 cc-pane）：无 client 时 SendPacket 自带报错。
	var pkt mcdkbridge.Packet
	if in.IsClient {
		pkt = mcdkbridge.ExecuteClientCode(in.Code)
	} else {
		pkt = mcdkbridge.ExecuteServerCode(in.Code)
	}
	if err := s.sendBridgePacket(in.RunID, pkt); err != nil {
		return textResult(fmt.Sprintf("错误: 发送失败: %v", err)), nil, nil
	}
	return textResult(fmt.Sprintf("已发送 execute_mc_code (isClient=%t)，执行结果请用 get_mc_pe_logs 看日志", in.IsClient)), nil, nil
}

func (s *Service) toolReloadMc(_ context.Context, _ *mcp.CallToolRequest, in reloadMcParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bridge == nil {
		return textResult("错误: MCDK 调试桥不可用"), nil, nil
	}
	var pkt mcdkbridge.Packet
	switch in.Kind {
	case "game":
		pkt = mcdkbridge.ReloadGame()
	case "addon_and_game":
		pkt = mcdkbridge.ReloadAddonAndGame()
	case "all_shaders":
		pkt = mcdkbridge.ReloadAllShaders()
	case "single_shader":
		if in.FileName == "" {
			return textResult("错误: kind=single_shader 时 fileName 必填"), nil, nil
		}
		pkt = mcdkbridge.ReloadSingleShader(in.FileName)
	default:
		return textResult(fmt.Sprintf("错误: 未知 kind=%s（支持: game / addon_and_game / all_shaders / single_shader）", in.Kind)), nil, nil
	}
	if err := s.sendBridgePacket(in.RunID, pkt); err != nil {
		return textResult(fmt.Sprintf("错误: 发送失败: %v", err)), nil, nil
	}
	return textResult(fmt.Sprintf("已发送 reload_mc kind=%s", in.Kind)), nil, nil
}

func (s *Service) toolGetMcPeLogs(_ context.Context, _ *mcp.CallToolRequest, in getMcPeLogsParams) (*mcp.CallToolResult, any, error) {
	if s.deps.MCPE == nil {
		return textResult("错误: MC PE 服务不可用"), nil, nil
	}
	// cc-pane: lines.unwrap_or(200).min(2000)。Go 零值无法区分「缺省」与「显式 0」，统一按缺省 → 200。
	lines := in.Lines
	if lines <= 0 {
		lines = 200
	}
	if lines > 2000 {
		lines = 2000
	}

	if in.RunID != "" {
		logs := s.deps.MCPE.RecentLogs(in.RunID, lines)
		if len(logs) == 0 {
			return textResult(fmt.Sprintf("错误: runId=%s 不存在或还没有日志输出（用 list_mc_pe_runs 查看当前客户端）", in.RunID)), nil, nil
		}
		return textResult(strings.Join(logs, "\n")), nil, nil
	}

	sums := s.deps.MCPE.ActiveRunSummaries()
	if len(sums) > 1 {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("当前有 %d 个 MC PE 客户端在运行，已分节返回各自日志（可传 runId 只看某一个）：\n", len(sums)))
		for _, sum := range sums {
			pid := "?"
			if sum.PID != nil {
				pid = strconv.Itoa(*sum.PID)
			}
			b.WriteString(fmt.Sprintf("\n━━ 客户端 [%s] runId=%s pid=%s ━━\n", sum.PackageName, sum.RunID, pid))
			logs := s.deps.MCPE.RecentLogs(sum.RunID, lines)
			if len(logs) == 0 {
				b.WriteString("（还没有日志输出）")
			} else {
				b.WriteString(strings.Join(logs, "\n"))
			}
			b.WriteString("\n")
		}
		return textResult(b.String()), nil, nil
	}

	logs := s.deps.MCPE.RecentLogs("", lines)
	if len(logs) == 0 {
		return textResult("错误: 没有活跃的 MC PE 运行（在 MC PE Panel 启动后再试），或当前还没有日志输出"), nil, nil
	}
	return textResult(strings.Join(logs, "\n")), nil, nil
}

func (s *Service) toolListMcPeRuns(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	if s.deps.MCPE == nil {
		return textResult("错误: MC PE 服务不可用"), nil, nil
	}
	sums := s.deps.MCPE.ActiveRunSummaries()
	if len(sums) == 0 {
		return textResult("没有正在运行的 MC PE 客户端（在 MC PE Panel 启动后再试）"), nil, nil
	}
	connected := map[string]bool{}
	legacy := 0 // 未携带 runId 的旧版桥连接（改版前启动的客户端，无 hello 帧）
	if s.deps.Bridge != nil {
		for _, id := range s.deps.Bridge.ClientRunIDs() {
			if id == "" {
				legacy++
				continue
			}
			connected[id] = true
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "共 %d 个 MC PE 客户端在运行：\n", len(sums))
	for _, sum := range sums {
		pid := "?"
		if sum.PID != nil {
			pid = strconv.Itoa(*sum.PID)
		}
		bridge := "未连入"
		if connected[sum.RunID] {
			bridge = "已连入"
		}
		fmt.Fprintf(&b, "runId=%s 包=[%s] pid=%s 调试桥=%s\n", sum.RunID, sum.PackageName, pid, bridge)
	}
	if legacy > 0 {
		fmt.Fprintf(&b, "（另有 %d 个未携带 runId 的旧版桥连接，无法与客户端对应，重启对应客户端即可）\n", legacy)
	}
	return textResult(b.String()), nil, nil
}

// pickMCPERun 按 runID 选择目标 run：显式 runId 直取；未指定时仅单 run 才允许兜底，
// 多开必须显式指定（与 sendBridgePacket 同一「宁可打断，不可选错人」策略）。
func (s *Service) pickMCPERun(runID string) (storage.MCRunSummary, error) {
	if s.deps.MCPE == nil {
		return storage.MCRunSummary{}, errors.New("MCPEService 未挂载")
	}
	sums := s.deps.MCPE.ActiveRunSummaries()
	if runID != "" {
		for _, sum := range sums {
			if sum.RunID == runID {
				return sum, nil
			}
		}
		return storage.MCRunSummary{}, fmt.Errorf("runId=%s 不存在（用 list_mc_pe_runs 查看当前客户端）", runID)
	}
	switch len(sums) {
	case 0:
		return storage.MCRunSummary{}, errors.New("未找到活跃的 MC PE 运行（请在 MC PE Panel 启动）")
	case 1:
		return sums[0], nil
	default:
		return storage.MCRunSummary{}, fmt.Errorf("有 %d 个 MC PE 客户端在运行，请传 runId 指定目标（用 list_mc_pe_runs 查看）", len(sums))
	}
}
