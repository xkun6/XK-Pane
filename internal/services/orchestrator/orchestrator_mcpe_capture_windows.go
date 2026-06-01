//go:build windows

package orchestrator

// P6e-7：Win32 截图 / 点击 MCP 工具（capture_window / click_window），仅 Windows 注册。
// 让多模态 Agent 看 MC PE 画面（480p JPEG）并归一化点击游戏内 UI。
// 截图/点击实现在 mcdkbridge（Win32 FFI）；这里取桥连接状态 + 活跃 run 的 PID 后调用。
// 忠实移植 cc-pane orchestrator_service.rs capture_window / click_window（中文 description 照搬）。

import (
	"context"
	"errors"
	"fmt"

	"xkpane/internal/services/mcdkbridge"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// clickWindowParams：x/y 归一化坐标（cc-pane McpClickWindowParams，无 camelCase）。
type clickWindowParams struct {
	X float64 `json:"x" jsonschema:"归一化横坐标 [0,1]（0=最左，1=最右）"`
	Y float64 `json:"y" jsonschema:"归一化纵坐标 [0,1]（0=最上，1=最下）"`
}

func (s *Service) registerMCPECaptureTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "capture_window",
		Description: "截取当前 MC PE 客户端窗口的画面（480p JPEG）。返回 image/jpeg MCP image content——多模态 LLM 可直接视觉理解，不需要解码 base64。需要已启动 MC PE 且 MCDK 调试桥已连接（include_test_mod=true 时桥模组会反向 attach）。",
	}, s.toolCaptureWindow)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "click_window",
		Description: "在当前 MC PE 窗口客户区模拟一次左键点击。坐标用 [0.0, 1.0] 归一化（0,0=左上、1,1=右下）。需要已启动 MC PE 且调试桥已连接；点击会把 MC PE 窗口强制前置。",
	}, s.toolClickWindow)
}

// currentMCPEPID 取当前活跃 MC PE 运行的 PID（UWP 单实例，最多一个 active），无则报错。
// 走 MCPE.ActiveRunPIDs() 而非 EnumWindows 枚举进程——避免 exe 名硬编码（对齐 cc-pane）。
func (s *Service) currentMCPEPID() (uint32, error) {
	if s.deps.MCPE == nil {
		return 0, errors.New("MCPEService 未挂载")
	}
	pids := s.deps.MCPE.ActiveRunPIDs()
	if len(pids) == 0 {
		return 0, errors.New("未找到活跃的 MC PE 运行（请在 MC PE Panel 启动）")
	}
	return uint32(pids[0]), nil
}

func (s *Service) toolCaptureWindow(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	if s.deps.Bridge == nil {
		return textResult("错误: MCDK 调试桥不可用"), nil, nil
	}
	if !s.deps.Bridge.Status().Connected {
		return textResult("错误: MC PE 未启动或调试桥未连接（桥模组尚未反向 attach）"), nil, nil
	}
	pid, err := s.currentMCPEPID()
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
	pid, err := s.currentMCPEPID()
	if err != nil {
		return textResult("错误: MCDK_NO_ACTIVE_MC_PE: " + err.Error()), nil, nil
	}
	if err := mcdkbridge.ClickWindowByPID(pid, float32(in.X), float32(in.Y)); err != nil {
		return textResult(fmt.Sprintf("错误: 点击失败: %v", err)), nil, nil
	}
	return textResult(fmt.Sprintf("已点击 (x=%.3f, y=%.3f)", in.X, in.Y)), nil, nil
}
