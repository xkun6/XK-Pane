// 终端域出口：Context（状态层）+ 公共类型。
// 树纯函数（paneTree）与 reducer 为内部实现，消费方按需经相对路径引用。
export { TerminalProvider, useTerminal } from './TerminalContext';
export type { TerminalContextValue, TerminalProviderProps } from './TerminalContext';
export type { NewTabOptions, TerminalTreeState } from './reducer';
export type {
  CliTool,
  KnownCliTool,
  PaneEdge,
  PaneNode,
  Panel,
  SplitPane,
  SplitDirection,
  Tab,
  TabViewMode,
  TerminalPaneLeaf,
  TerminalPaneNode,
  TerminalPaneSplit,
} from './types';
