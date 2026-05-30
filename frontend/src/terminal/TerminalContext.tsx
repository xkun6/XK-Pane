// 终端域状态 Context —— 替代 cc-pane 的 Zustand `usePanesStore`（D2：弃 Zustand，用 React Context + useReducer）。
// 承载外层面板树（窗口布局：Panel / SplitPane）与活动面板，对外暴露状态 + 绑定好的操作回调。
//
// 与 foundation 既有 Provider（PreferencesProvider / FoundationThemeProvider）同构：
//   createContext + Provider 组件 + use* hook（消费者不在 Provider 内会抛错）。
//
// 生命周期 reconcile effect（会话11）：对账 rootPane 期望态 vs 后端实际 PTY ——
//   有 projectPath 且无 session 的 Tab → TerminalService.create → setTabSession 回填；
//   树中已消失的 session → kill（防泄漏，含「创建途中 Tab 被关」的二次校验）。
//   纯对账计算在 sessionReconcile.ts（可测）；onExit 由各 TerminalView 自订阅（写退出提示）。

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  type ReactNode,
} from 'react';
import { LaunchHistoryService, TerminalService } from '@/services';
import {
  createInitialTreeState,
  terminalTreeReducer,
  type NewTabOptions,
  type TerminalTreeState,
} from './reducer';
import {
  collectTabsWithPane,
  orphanedSessionIds,
  tabsNeedingSession,
  treeSessionIds,
} from './sessionReconcile';
import type { PaneNode, SplitDirection } from './types';

/** 启动一个项目终端所需的最小信息（projectPath = PTY cwd，铁律 D7）。 */
export interface LaunchProjectInput {
  projectId: string;
  projectName: string;
  projectPath: string;
  alias?: string | null;
  cliTool: string;
  /** 恢复已有会话 id（明文 → CLI resume）；缺省 = 新会话。 */
  resumeId?: string;
}

export interface TerminalContextValue {
  /** 外层面板树根（窗口布局）。 */
  rootPane: PaneNode;
  /** 当前活动面板 id。 */
  activePaneId: string;
  /** 在指定面板新建一个终端 Tab 并激活。 */
  newTab: (paneId: string, opts: NewTabOptions) => void;
  /**
   * 在活动面板启动一个项目终端：新建 Tab（projectPath + cliTool）并记录启动历史。
   * Home 最近启动、ProjectPanel 启动菜单共用此入口（跨页面启动的唯一事实源）。
   */
  launchProject: (input: LaunchProjectInput) => void;
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
  const launchProject = useCallback(
    (input: LaunchProjectInput): void => {
      // resume 必须带会话引擎对应 CLI —— 'none'/空会走裸 shell 丢掉 --resume。
      // 调用方（SessionsView）总按会话来源传正确 tool；此处仅兜底为 claude（最常见）。
      const effectiveCliTool =
        input.resumeId && (!input.cliTool || input.cliTool === 'none')
          ? 'claude'
          : input.cliTool;
      const base = input.alias ?? input.projectName;
      const title =
        effectiveCliTool && effectiveCliTool !== 'none' ? `${base} (${effectiveCliTool})` : base;
      dispatch({
        type: 'addTab',
        paneId: state.activePaneId,
        opts: {
          projectId: input.projectId,
          projectPath: input.projectPath,
          cliTool: effectiveCliTool,
          resumeId: input.resumeId,
          title,
        },
      });
      // 记录启动历史（fire-and-forget：记录失败绝不阻断终端启动）。
      void LaunchHistoryService.record({
        projectId: input.projectId,
        projectName: input.projectName,
        projectPath: input.projectPath,
        cliTool: effectiveCliTool,
        runtimeKind: 'local',
      }).catch(() => {});
    },
    [state.activePaneId],
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

  // ── 生命周期 reconcile：树变化时对账后端 PTY 会话（创建缺失 / 清理孤儿）──
  const aliveRef = useRef(true);
  const rootPaneRef = useRef(state.rootPane);
  const creatingRef = useRef<Set<string>>(new Set());
  // knownRef = 上一轮 reconcile 观察到的「树内会话集」；据此 diff 出被关闭的孤儿会话。
  const knownRef = useRef<Set<string>>(new Set());

  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
    };
  }, []);
  useEffect(() => {
    rootPaneRef.current = state.rootPane;
  });

  useEffect(() => {
    const tabs = collectTabsWithPane(state.rootPane);

    // 1) 为「有 projectPath 且无 session」的 Tab 创建 PTY，完成后回填 sessionId。
    for (const t of tabsNeedingSession(tabs, creatingRef.current)) {
      creatingRef.current.add(t.tabId);
      TerminalService.create({
        projectPath: t.projectPath,
        workspacePath: t.workspacePath,
        cliTool: t.cliTool,
        resumeId: t.resumeId,
      })
        .then((sessionId) => {
          // 创建途中 Tab 可能已被关闭 → 二次校验，避免把 session 回填到不存在的 Tab（泄漏）。
          const stillNeeds = collectTabsWithPane(rootPaneRef.current).some(
            (x) => x.tabId === t.tabId && x.sessionId === null,
          );
          if (stillNeeds && aliveRef.current) {
            // 用捕获的 t.paneId 回填 —— 当前安全：reducer 未实现 moveTab/reorderTabs（Tab 不会换 pane）。
            // TODO(移植 moveTab 时)：Tab 可跨 pane 迁移后，须按 tabId 查当前 pane 再回填，否则
            // setTabSession 落到旧 paneId 会 no-op 并泄漏该 session（reconcile 评审 wf_651d3b47 指出）。
            dispatch({ type: 'setTabSession', paneId: t.paneId, tabId: t.tabId, sessionId });
          } else {
            void TerminalService.kill(sessionId).catch(() => {});
          }
        })
        .catch(() => {
          // 创建失败：不回填；下次树变化时（creatingRef 已清）才会重试，无紧循环。
        })
        .finally(() => {
          creatingRef.current.delete(t.tabId);
        });
    }

    // 2) 清理孤儿：上轮已知但本轮已不在树中的会话（Tab/面板被关）→ kill。
    const current = treeSessionIds(tabs);
    for (const sid of orphanedSessionIds(knownRef.current, current)) {
      void TerminalService.kill(sid).catch(() => {});
    }
    knownRef.current = current;
  }, [state.rootPane]);

  const value = useMemo<TerminalContextValue>(
    () => ({
      rootPane: state.rootPane,
      activePaneId: state.activePaneId,
      newTab,
      launchProject,
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
      launchProject,
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
