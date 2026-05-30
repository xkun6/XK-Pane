// Package clitools 是把 CLI 工具注册表暴露给前端的 Wails service。
//
// 对齐 cc-pane 的 list_cli_tools（src-tauri 命令）+ web/services/cliToolService.ts：
// 前端启动菜单据此列出 8 个 CLI 工具、各自 installed/version/capabilities，决定
// 菜单项是否可点、是否显示 Provider/Resume 等。
//
// 设计要点：本 service 与 terminal service 共享**同一个** *cliadapters.Registry 实例
// （由 app 装配时传入），避免两处各自 cliadapters.Default() 导致 DetectAll 重复跑
// which + --version 探测（每工具最多 5s）。
package clitools

import (
	"context"

	"xkpane/internal/services/cliadapters"
)

// Service 是 Wails CLI 工具服务，前端通过生成的 bindings 调用。
type Service struct {
	registry *cliadapters.Registry
}

// New 构造服务。registry 由 app 装配传入（与 terminal service 共享同一实例）。
func New(registry *cliadapters.Registry) *Service {
	return &Service{registry: registry}
}

// ListTools 按注册顺序返回各工具的**静态**元信息（不检测安装状态，快）。
// 用于只需要 id/displayName 的场景。
func (s *Service) ListTools(_ context.Context) ([]cliadapters.ToolInfo, error) {
	return s.registry.ListTools(), nil
}

// DetectAll 按注册顺序检测所有工具的安装状态、版本与能力声明（慢：跑 which + --version）。
// 启动菜单用此结果决定哪些 CLI 可点、各自 capabilities（supportsProvider/Resume 等）。
// 前端应缓存结果（仿 cc-pane useCliTools），避免每次开菜单都重新探测。
func (s *Service) DetectAll(_ context.Context) ([]cliadapters.ToolInfo, error) {
	return s.registry.DetectAll(), nil
}
