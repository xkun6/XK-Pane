// TerminalPage 当前无页面级 ViewModel 状态：左列 WorkspaceTree 自包含消费 WorkspaceContext +
// TerminalContext（含 CLI 启动），右列 PaneTree 消费 TerminalContext.rootPane。
// 保留此文件以符合「页面三件齐全」约定，便于将来扩展（如页面级筛选 / 工具栏）。
export interface TerminalPageModel {}

export function useTerminalPage(): TerminalPageModel {
  return {};
}
