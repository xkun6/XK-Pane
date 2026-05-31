// 应用启动后由后端定时推送的事件名集合，
// 与 internal/events/events.go 中的常量保持同步。
export const AppEvents = {
  Time: 'time',
  // Local History 自动捕获保存版本后后端广播（前端历史页按 projectPath 过滤后 live 刷新）。
  LocalHistoryChanged: 'localhistory:changed',
  // 编排：后端 MCP launch_task 建好会话后广播，前端开新 Tab 附着该 sessionId。
  OrchestratorLaunch: 'orchestrator:launch',
} as const;

export type AppEventName = (typeof AppEvents)[keyof typeof AppEvents];
