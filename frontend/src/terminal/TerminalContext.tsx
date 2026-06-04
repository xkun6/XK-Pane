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
import { LaunchHistoryService, OrchestratorService, TerminalService } from '@/services';
import { lifecycleRole } from '@/services/transport';
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
import type { PaneEdge, PaneNode, SplitDirection, TabViewMode } from './types';
import { lastTerminalSize } from './terminalSize';

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
  /** 置顶 / 取消置顶某 Tab（置顶 Tab 不可关闭）。 */
  togglePin: (paneId: string, tabId: string) => void;
  /** 重命名某 Tab（空标题忽略）。 */
  renameTab: (paneId: string, tabId: string, title: string) => void;
  /** 切换某 Tab 的视图模式（终端 ↔ 卡片，按 tabId 全树定位）。 */
  setTabViewMode: (tabId: string, mode: TabViewMode) => void;
  /** 设置活动面板。 */
  setActivePanel: (paneId: string) => void;
  /** 分屏：向右（左右）或向下（上下）拆出一个新面板。 */
  splitPanel: (paneId: string, direction: SplitDirection) => void;
  /** 关闭面板（自动折叠单子 split / 重整 sizes / 修正活动面板）。 */
  closePanel: (paneId: string) => void;
  /** 调整某 split 节点的子面板占比。 */
  resizePanels: (paneId: string, sizes: number[]) => void;
  /** 同面板内重排 Tab（拖拽落到同面板另一 Tab 上）。 */
  reorderTabs: (paneId: string, fromIndex: number, toIndex: number) => void;
  /** 跨面板移动 Tab（拖到另一面板）；toIndex 缺省追加到末尾；源面板掏空则折叠。 */
  moveTab: (fromPaneId: string, toPaneId: string, tabId: string, toIndex?: number) => void;
  /** 把任意 Tab 拖到任意目标面板的某条边 → 在其旁拆出新面板装它（跨面板时源面板掏空则折叠）。 */
  moveTabToEdge: (fromPaneId: string, tabId: string, targetPaneId: string, edge: PaneEdge) => void;
  /** 回填某 Tab 的活动 PTY 会话 id（供生命周期 effect 在 create 完成后调用，按 tabId 全树定位）。 */
  setTabSession: (tabId: string, sessionId: string) => void;
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

      // Web（viewer）：不本地建会话（reconcile 在 viewer 不跑，addTab-without-sid 会成孤儿），
      // 改经 POST /api/launch → 后端 LaunchTask → 广播 orchestrator:launch → 桌面 + 本端同时
      // 开窗附着同 sid（P7 D1/D2）。新任务（带 prompt）走 WebLauncher；此处覆盖 resume 等场景。
      // 不记启动历史（LaunchHistory 是桌面专属 binding，未在 Web 暴露）。
      if (lifecycleRole === 'viewer') {
        void OrchestratorService.launch({
          projectPath: input.projectPath,
          cliTool: effectiveCliTool,
          resumeId: input.resumeId,
          title,
        }).catch(() => {});
        return;
      }

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
  const togglePin = useCallback(
    (paneId: string, tabId: string) => dispatch({ type: 'togglePinTab', paneId, tabId }),
    [],
  );
  const renameTab = useCallback(
    (paneId: string, tabId: string, title: string) =>
      dispatch({ type: 'renameTab', paneId, tabId, title }),
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
  const reorderTabs = useCallback(
    (paneId: string, fromIndex: number, toIndex: number) =>
      dispatch({ type: 'reorderTabs', paneId, fromIndex, toIndex }),
    [],
  );
  const moveTab = useCallback(
    (fromPaneId: string, toPaneId: string, tabId: string, toIndex?: number) =>
      dispatch({ type: 'moveTab', fromPaneId, toPaneId, tabId, toIndex }),
    [],
  );
  const moveTabToEdge = useCallback(
    (fromPaneId: string, tabId: string, targetPaneId: string, edge: PaneEdge) =>
      dispatch({ type: 'moveTabToEdge', fromPaneId, tabId, targetPaneId, edge }),
    [],
  );
  const setTabSession = useCallback(
    (tabId: string, sessionId: string) => dispatch({ type: 'setTabSession', tabId, sessionId }),
    [],
  );
  const setTabViewMode = useCallback(
    (tabId: string, mode: TabViewMode) => dispatch({ type: 'setTabViewMode', tabId, mode }),
    [],
  );

  // ── 生命周期 reconcile：树变化时对账后端 PTY 会话（创建缺失 / 清理孤儿）──
  const aliveRef = useRef(true);
  const rootPaneRef = useRef(state.rootPane);
  const creatingRef = useRef<Set<string>>(new Set());
  // knownRef = 上一轮 reconcile 观察到的「树内会话集」；据此 diff 出被关闭的孤儿会话。
  const knownRef = useRef<Set<string>>(new Set());
  // activePaneIdRef = 最新活跃面板 id（供「只挂一次」的编排订阅 effect 取当前值，避免重订阅）。
  const activePaneIdRef = useRef(state.activePaneId);

  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
    };
  }, []);
  useEffect(() => {
    rootPaneRef.current = state.rootPane;
    activePaneIdRef.current = state.activePaneId;
  });

  // ── 编排：后端 MCP launch_task 建好会话后广播 orchestrator:launch → 开新 Tab 附着该 sessionId。
  // 关键：Tab 直接带 sessionId 入树，reconcile 的 tabsNeedingSession（sessionId===null 才建）会跳过，
  // 不会重复 CreateSession。挂一次（TerminalProvider 全局常驻，不漏事件）。
  useEffect(() => {
    return OrchestratorService.onLaunch((p) => {
      dispatch({
        type: 'addTab',
        paneId: activePaneIdRef.current,
        opts: {
          projectId: '',
          projectPath: p.projectPath,
          cliTool: p.cliTool || undefined,
          title: p.title || undefined,
          sessionId: p.sessionId,
        },
      });
    });
  }, []);

  useEffect(() => {
    // viewer（Web）生命周期守护（P7 D1/D4）：绝不跑「孤儿 kill」与「建会话」——
    //   - Web 的 Tab 只经 orchestrator:launch 带 sid 入树（无需 create）；
    //   - Web 关窗 = 纯 detach（移本地 Tab，不 kill），否则一端关 Tab 会杀掉另一端正用的会话。
    // kill 永远显式即时（桌面关 Tab / Web「终止会话」M5 / 进程自退），后端不做引用计数。
    if (lifecycleRole === 'viewer') return;

    const tabs = collectTabsWithPane(state.rootPane);

    // 1) 为「有 projectPath 且无 session」的 Tab 创建 PTY，完成后回填 sessionId。
    for (const t of tabsNeedingSession(tabs, creatingRef.current)) {
      creatingRef.current.add(t.tabId);
      // 初始列宽 = 当前终端面板最近 fit 出的真实列宽（terminalSize 模块快照）。让 PTY 一创建就贴近
      // 真实显示尺寸 —— 否则 reclaude --resume 会按后端默认列宽折行重放历史，与前端实际列宽不符而整屏
      // 错位（详见 terminalSize.ts）。cols=0（冷启动尚无终端 fit 过）则回退后端默认。
      const initialSize = lastTerminalSize();
      TerminalService.create({
        projectPath: t.projectPath,
        workspacePath: t.workspacePath,
        cliTool: t.cliTool,
        resumeId: t.resumeId,
        cols: initialSize.cols,
        rows: initialSize.rows,
      })
        .then((sessionId) => {
          // 创建途中 Tab 可能已被关闭 → 二次校验，避免把 session 回填到不存在的 Tab（泄漏）。
          const stillNeeds = collectTabsWithPane(rootPaneRef.current).some(
            (x) => x.tabId === t.tabId && x.sessionId === null,
          );
          if (stillNeeds && aliveRef.current) {
            // 按 tabId 回填（setTabSession 已改为全树按 tabId 定位）—— P5 moveTab 落地后 Tab 可
            // 跨 pane 迁移，捕获的旧 t.paneId 可能失效；按 tabId 查当前 pane 才不会 no-op 泄漏。
            dispatch({ type: 'setTabSession', tabId: t.tabId, sessionId });
            // 触发启动历史回填：后端后台轮询扫盘发现本次启动对应的可恢复会话 id 写回记录，
            // 供「最近启动」点击 resume 续会话。fire-and-forget；none/裸 shell 后端自动 no-op。
            void LaunchHistoryService.startBackfill({
              projectPath: t.projectPath,
              workspacePath: t.workspacePath,
              cliTool: t.cliTool ?? '',
              ptySessionId: sessionId,
            }).catch(() => {});
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
      togglePin,
      renameTab,
      setActivePanel,
      splitPanel,
      closePanel,
      resizePanels,
      reorderTabs,
      moveTab,
      moveTabToEdge,
      setTabSession,
      setTabViewMode,
    }),
    [
      state.rootPane,
      state.activePaneId,
      newTab,
      launchProject,
      closeTab,
      setActiveTab,
      togglePin,
      renameTab,
      setActivePanel,
      splitPanel,
      closePanel,
      resizePanels,
      reorderTabs,
      moveTab,
      moveTabToEdge,
      setTabSession,
      setTabViewMode,
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
