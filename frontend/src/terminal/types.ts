// 终端面板树类型 —— 忠实移植 cc-pane `web/types/{pane,terminal}.ts` 的 Phase 2 MVP 子集。
//
// 两级嵌套分屏模型：
//   外层 PaneNode 树（窗口布局）：Panel（含多个 Tab）| SplitPane（拆分多个 Panel）。
//   内层 TerminalPaneNode 树（单个 Tab 内）：TerminalPaneLeaf（持 sessionId）| TerminalPaneSplit。
//
// MVP 刻意裁剪的字段（依赖未落地子系统，落地时再补，勿现在硬塞——CLAUDE.md 简洁铁律）：
//   - Provider / launch-profile：providerId / providerSelection / launchProfileId /
//     workspaceSnapshotId（Phase 3 Provider 管理）
//   - 运行时：ssh / machineName / disconnected / restoring / savedSessionId（Phase 4 SSH）
//   - resume：resumeId（Phase 3 会话恢复）
//   - 非终端 Tab：contentType 的 editor / mcp-config / skill-manager / ... 变体（Phase 4/5）
//   - UI 细节：pinned / minimized / dirty / filePath / reclaimKey（后续）

import type { SSHSpec } from '@/services/terminal/TerminalService';

/** CLI 工具 id（已知值自动补全 + 允许任意字符串，对齐后端 cliadapters 保留的 3 适配器）。 */
export type KnownCliTool = 'none' | 'claude' | 'reclaude' | 'codex';
export type CliTool = KnownCliTool | (string & {});

/** Tab 视图模式：terminal = xterm 终端镜像；card = SessionTimeline 结构化对话卡片（仅 claude/reclaude）。 */
export type TabViewMode = 'terminal' | 'card';

// ---- 内层：单个 Tab 内的终端分屏树 ----

export interface TerminalPaneLeaf {
  type: 'leaf';
  id: string;
  /** CC-Panes 持有的活动 PTY 会话 id；null = 尚未启动。 */
  sessionId: string | null;
  cliTool?: CliTool;
  /** 多目录 CLI 的 --add-dir（铁律 D7）；仅 ≠projectPath 时生效。 */
  workspacePath?: string;
  /** 恢复已有会话的 id（明文透传后端 → CLI resume）；undefined = 新会话。 */
  resumeId?: string;
  // TODO(phase3/4): providerId / providerSelection / launchProfileId /
  //   workspaceSnapshotId / ssh / machineName / disconnected / restoring / savedSessionId
}

export interface TerminalPaneSplit {
  type: 'split';
  id: string;
  direction: 'horizontal' | 'vertical';
  children: TerminalPaneNode[];
  sizes: number[]; // 各子节点占比百分比
}

export type TerminalPaneNode = TerminalPaneLeaf | TerminalPaneSplit;

// ---- Tab ----

export interface Tab {
  id: string;
  title: string;
  /** MVP 仅 'terminal'；Phase 4/5 widen 到 editor / mcp-config / skill-manager / memory-manager / file-explorer。 */
  contentType: 'terminal';
  projectId: string;
  projectPath: string; // = PTY cwd（铁律 D7）
  /** Tab 级活动 PTY 会话 id（与 terminalRootPane 的活动 leaf 协同）。 */
  sessionId: string | null;
  cliTool?: CliTool;
  workspacePath?: string;
  /** 恢复已有会话的 id（明文透传后端 → CLI resume）；undefined = 新会话。 */
  resumeId?: string;
  /** 运行时：缺省/'local' = 本地 PTY；'ssh' = 本地 ssh 客户端拉起远程会话。 */
  runtimeKind?: string;
  /** runtimeKind=='ssh' 时的连接信息（与 CreateSpec.ssh 同形，透传后端 buildSSHCommand）。 */
  ssh?: SSHSpec;
  /** 该 Tab 内的终端分屏树根；单终端时即一个 leaf。 */
  terminalRootPane?: TerminalPaneNode;
  activeTerminalPaneId?: string;
  /** 置顶：置顶的 Tab 不可被关闭（关闭按钮隐藏、closeTab 守门），与 cc-pane 一致。 */
  pinned?: boolean;
  /** 视图模式（缺省 = 'terminal'）：在 TabBar 切换；'card' 时该 Tab 渲染 SessionTimeline 覆盖层。 */
  viewMode?: TabViewMode;
  // TODO(phase3/4): provider* / launchProfileId / workspaceSnapshotId /
  //   ssh / machineName / minimized / ... 见文件头
}

// ---- 外层：窗口布局面板树 ----

export interface Panel {
  type: 'panel';
  id: string;
  tabs: Tab[];
  activeTabId: string;
}

export interface SplitPane {
  type: 'split';
  id: string;
  direction: 'horizontal' | 'vertical'; // horizontal=左右分割, vertical=上下分割
  children: PaneNode[];
  sizes: number[]; // 各子面板占比百分比
}

export type PaneNode = Panel | SplitPane;

/** 分屏操作方向：向右拆（左右）/ 向下拆（上下）。 */
export type SplitDirection = 'right' | 'down';

/** 面板的四条边（拖 Tab 落到某边 → 在该面板旁拆出新面板）。left/top=插到目标前，right/bottom=插到目标后。 */
export type PaneEdge = 'left' | 'right' | 'top' | 'bottom';
