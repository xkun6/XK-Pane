// 后端推送的事件名集合 —— **前后端唯一事实源**（与 internal/events/events.go 的常量字符串
// 严格一致，由 events.test.ts 断言防漂移，P7 §7 铁律 5）。桌面经 Wails 应用级总线、Web 经
// /ws 镜像广播，两路都用这套名字。transport 抽象按这些名字订阅（见 services/transport）。
export const AppEvents = {
  Time: 'time',
  // Local History 自动捕获保存版本后后端广播（前端历史页按 projectPath 过滤后 live 刷新）。
  LocalHistoryChanged: 'localhistory:changed',
  // 编排：后端 launch_task / POST /api/launch 建好会话后广播，前端开新 Tab 附着该 sessionId。
  OrchestratorLaunch: 'orchestrator:launch',
  // 终端会话事件（PTY reader/wait → 前端 xterm）。payload 按 sessionId 过滤分发。
  // 这三个是 Web 只读镜像的核心：经 /ws 送浏览器后多端 attach 同 sid 即同步显示（P7 M3）。
  TerminalOutput: 'terminal:output',
  TerminalStatus: 'terminal:status',
  TerminalExit: 'terminal:exit',
  // 待答 AskUserQuestion（被启动 CLI 的 PreToolUse hook → orchestrator → 卡片）。payload 按 sessionId 分发，
  // 让卡片在该问题落盘转录前就能渲染可点选项并作答（待答态不在转录里）。
  TerminalAskPending: 'terminal:ask-pending',
  // MC PE 运行事件（后端 launcher PTY reader/wait → 前端 MC PE 面板）。payload 按 runId 路由。P6e。
  MCPeOutput: 'mc_pe:output',
  MCPeLaunched: 'mc_pe:launched',
  MCPeExited: 'mc_pe:exited',
  // Bedrock 知识库 OTA 同步进度（后端 bedrockdocsync.sync → 前端「设置 → 知识库」面板）。按 stage 分发。P6f。
  BedrockDocSyncProgress: 'bedrock-doc-sync:progress',
  // 托盘「设置」菜单 → 主窗口导航到设置页（桌面托盘专属，无 payload，不进 Web 镜像）。
  NavigateSettings: 'app:navigate-settings',
} as const;

export type AppEventName = (typeof AppEvents)[keyof typeof AppEvents];
