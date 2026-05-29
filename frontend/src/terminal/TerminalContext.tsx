// 终端域状态 Context —— 替代 cc-pane 的 Zustand `usePanesStore`（D2：弃 Zustand，用 React Context + useReducer）。
// 承载外层面板树（窗口布局：Panel / SplitPane）与活动面板，对外暴露状态 + 绑定好的操作回调。
//
// 与 foundation 既有 Provider（PreferencesProvider / FoundationThemeProvider）同构：
//   createContext + Provider 组件 + use* hook（消费者不在 Provider 内会抛错）。
//
// TODO(下一单元 · 随 TerminalView)：TerminalService 生命周期 effect ——
//   newTab 后调 TerminalService.create(...) 拿 sessionId → dispatch setTabSession 回填；
//   订阅 onExit → 清理对应 leaf/Tab。本单元只落地纯状态层（可 Vitest 覆盖的 reducer + Provider 壳）。

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useReducer,
  type ReactNode,
} from 'react';
import {
  createInitialTreeState,
  terminalTreeReducer,
  type NewTabOptions,
  type TerminalTreeState,
} from './reducer';
import type { PaneNode, SplitDirection } from './types';

export interface TerminalContextValue {
  /** 外层面板树根（窗口布局）。 */
  rootPane: PaneNode;
  /** 当前活动面板 id。 */
  activePaneId: string;
  /** 在指定面板新建一个终端 Tab 并激活。 */
  newTab: (paneId: string, opts: NewTabOptions) => void;
  /** 关闭 Tab；若是面板最后一个 Tab 则连带关闭面板（自动折叠单子 split）。 */
  closeTab: (paneId: string, tabId: string) => void;
  /** 激活某面板内的某 Tab（同时把该面板设为活动面板）。 */
  setActiveTab: (paneId: string, tabId: string) => void;
  /** 设置活动面板。 */
  setActivePanel: (paneId: string) => void;
  /** 分屏：向右（左右）或向下（上下）拆出一个新面板。 */
  splitPanel: (paneId: string, direction: SplitDirection) => void;
  /** 关闭面板（自动折叠单子 split / 重整 sizes / 修正活动面板）。 */
  closePanel: (paneId: string) => void;
  /** 调整某 split 节点的子面板占比。 */
  resizePanels: (paneId: string, sizes: number[]) => void;
  /** 回填某 Tab 的活动 PTY 会话 id（供生命周期 effect 在 create 完成后调用）。 */
  setTabSession: (paneId: string, tabId: string, sessionId: string) => void;
}

const TerminalContext = createContext<TerminalContextValue | null>(null);

export interface TerminalProviderProps {
  children: ReactNode;
  /** 可选注入初始状态（测试 / 会话恢复场景）。 */
  initial?: TerminalTreeState;
}

export const TerminalProvider = ({ children, initial }: TerminalProviderProps) => {
  const [state, dispatch] = useReducer(
    terminalTreeReducer,
    undefined,
    () => initial ?? createInitialTreeState(),
  );

  const newTab = useCallback(
    (paneId: string, opts: NewTabOptions) => dispatch({ type: 'addTab', paneId, opts }),
    [],
  );
  const closeTab = useCallback(
    (paneId: string, tabId: string) => dispatch({ type: 'closeTab', paneId, tabId }),
    [],
  );
  const setActiveTab = useCallback(
    (paneId: string, tabId: string) => dispatch({ type: 'selectTab', paneId, tabId }),
    [],
  );
  const setActivePanel = useCallback(
    (paneId: string) => dispatch({ type: 'setActivePane', paneId }),
    [],
  );
  const splitPanel = useCallback(
    (paneId: string, direction: SplitDirection) => dispatch({ type: 'split', paneId, direction }),
    [],
  );
  const closePanel = useCallback(
    (paneId: string) => dispatch({ type: 'closePane', paneId }),
    [],
  );
  const resizePanels = useCallback(
    (paneId: string, sizes: number[]) => dispatch({ type: 'resizePanes', paneId, sizes }),
    [],
  );
  const setTabSession = useCallback(
    (paneId: string, tabId: string, sessionId: string) =>
      dispatch({ type: 'setTabSession', paneId, tabId, sessionId }),
    [],
  );

  const value = useMemo<TerminalContextValue>(
    () => ({
      rootPane: state.rootPane,
      activePaneId: state.activePaneId,
      newTab,
      closeTab,
      setActiveTab,
      setActivePanel,
      splitPanel,
      closePanel,
      resizePanels,
      setTabSession,
    }),
    [
      state.rootPane,
      state.activePaneId,
      newTab,
      closeTab,
      setActiveTab,
      setActivePanel,
      splitPanel,
      closePanel,
      resizePanels,
      setTabSession,
    ],
  );

  return <TerminalContext.Provider value={value}>{children}</TerminalContext.Provider>;
};

export const useTerminal = (): TerminalContextValue => {
  const ctx = useContext(TerminalContext);
  if (!ctx) {
    throw new Error('useTerminal must be used inside <TerminalProvider>');
  }
  return ctx;
};
