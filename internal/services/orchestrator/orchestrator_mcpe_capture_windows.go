//go:build windows

package orchestrator

// P6e-7：Win32 截图 / 点击 MCP 工具（capture_window / click_window），仅 Windows 注册。
// 让多模态 Agent 看 MC PE 画面（480p JPEG）并归一化点击游戏内 UI。
// 截图/点击实现在 mcdkbridge（Win32 FFI）；这里取桥连接状态 + 目标 run 的 PID 后调用。
// 多开 MC PE 时按 runId 定向目标客户端（消歧统一走 pickMCPERun，单开可省略 runId）。
// 移植自 cc-pane orchestrator_service.rs capture_window / click_window（中文 description 沿袭）。

import (
	"context"
	"fmt"

	"xkpane/internal/services/mcdkbridge"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// captureWindowParams：截图目标（多开消歧）。
type captureWindowParams struct {
	RunID string `json:"runId,omitempty" jsonschema:"目标 MC PE 客户端的 runId（多开时必填；单开可省略，用 list_mc_pe_runs 获取）"`
}

// clickWindowParams：x/y 归一化坐标（cc-pane McpClickWindowParams，无 camelCase）+ 多开消歧 runId。
type clickWindowParams struct {
	X     float64 `json:"x" jsonschema:"归一化横坐标 [0,1]（0=最左，1=最右）"`
	Y     float64 `json:"y" jsonschema:"归一化纵坐标 [0,1]（0=最上，1=最下）"`
	RunID string  `json:"runId,omitempty" jsonschema:"目标 MC PE 客户端的 runId（多开时必填；单开可省略，用 list_mc_pe_runs 获取）"`
}

func (s *Service) registerMCPECaptureTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "capture_window",
		Description: "截取 MC PE 客户端窗口的画面（480p JPEG）。返回 image/jpeg MCP image content——多模态 LLM 可直接视觉理解，不需要解码 base64。需要已启动 MC PE 且 MCDK 调试桥已连接（include_test_mod=true 时桥模组会反向 attach）。多开 MC PE 时必须传 runId 指定目标客户端（单开可省略，runId 用 list_mc_pe_runs 获取）。",
	}, s.toolCaptureWindow)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "click_window",
		Description: "在 MC PE 窗口客户区模拟一次左键点击。坐标用 [0.0, 1.0] 归一化（0,0=左上、1,1=右下）。需要已启动 MC PE 且调试桥已连接；点击会把 MC PE 窗口强制前置。多开 MC PE 时必须传 runId 指定目标客户端（单开可省略，runId 用 list_mc_pe_runs 获取）。",
	}, s.toolClickWindow)
}

// resolveMCPEPID 把 runId 解析成目标客户端 PID（多开消歧统一走 pickMCPERun）。
func (s *Service) resolveMCPEPID(runID string) (uint32, error) {
	sum, err := s.pickMCPERun(runID)
	if err != nil {
		return 0, err
	}
	if sum.PID == nil {
		return 0, fmt.Errorf("runId=%s 无有效 PID（进程未提供）", sum.RunID)
	}
	return uint32(*sum.PID), nil
}

func (s *Service) toolCaptureWindow(_ context.Context, _ *mcp.CallToolRequest, in captureWindowParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bridge == nil {
		return textResult("错误: MCDK 调试桥不可用"), nil, nil
	}
	if !s.deps.Bridge.Status().Connected {
		return textResult("错误: MC PE 未启动或调试桥未连接（桥模组尚未反向 attach）"), nil, nil
	}
	pid, err := s.resolveMCPEPID(in.RunID)
	if err != nil {
		return textResult("错误: MCDK_NO_ACTIVE_MC_PE: " + err.Error()), nil, nil
	}
	jpegBytes, err := mcdkbridge.CaptureWindowByPID(pid)
	if err != nil {
		return textResult(fmt.Sprintf("错误: 截图失败: %v", err)), nil, nil
	}
	// 传原始 JPEG 字节：go-sdk 的 json.Marshal 对 []byte 自动 base64（别自己再 base64）。
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.ImageContent{Data: jpegBytes, MIMEType: "image/jpeg"}}}, nil, nil
}

func (s *Service) toolClickWindow(_ context.Context, _ *mcp.CallToolRequest, in clickWindowParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Bridge == nil {
		return textResult("错误: MCDK 调试桥不可用"), nil, nil
	}
	if !s.deps.Bridge.Status().Connected {
		return textResult("错误: MC PE 未启动或调试桥未连接"), nil, nil
	}
	pid, err := s.resolveMCPEPID(in.RunID)
	if err != nil {
		return textResult("错误: MCDK_NO_ACTIVE_MC_PE: " + err.Error()), nil, nil
	}
	if err := mcdkbridge.ClickWindowByPID(pid, float32(in.X), float32(in.Y)); err != nil {
		return textResult(fmt.Sprintf("错误: 点击失败: %v", err)), nil, nil
	}
	return textResult(fmt.Sprintf("已点击 (x=%.3f, y=%.3f)", in.X, in.Y)), nil, nil
}
