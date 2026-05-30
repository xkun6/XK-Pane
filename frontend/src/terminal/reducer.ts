// 终端外层面板树归约器 —— 忠实移植 cc-pane `web/stores/usePanesStore.ts` 的面板树操作。
//
// cc-pane 用 Zustand + immer（可变 draft 风格）；本实现遵循 D2（弃 Zustand，用纯 useReducer）。
// 移植手法 = clone-then-mutate：对 rootPane 做 structuredClone 后，套用 cc-pane 的可变逻辑，
// 再返回新树。这样 1:1 复刻 immer 的可变语义，同时保持 reducer 为纯函数（可 Vitest 覆盖）。
//
// 仅移植 Phase 2 MVP 所需的外层操作：split / closePane / addTab / closeTab / selectTab /
// setActivePane / resizePanes / setTabSession（生命周期回填钩子）。
// 刻意未移植（依赖未落地子系统，勿现在硬塞——CLAUDE.md 简洁铁律）：
//   - closedTabs 可恢复快照 + syncPtyRegistryUnregister（Phase 3 会话恢复 + 生命周期 effect）
//   - pinned 校验（UI 细节，后续）
//   - moveTab / reorderTabs / splitAndMoveTab（拖拽，随 TabBar 单元）
//   - 内层 terminalPane 分屏 splitTerminalPane / closeTerminalPane（单 Tab 内分屏，后续）

import {
  collectPanels,
  createEmptyPanel,
  createPanel,
  createTab,
  findPane,
  findParent,
  generateId,
} from './paneTree';
import type { CliTool, PaneNode, SplitDirection } from './types';

export interface TerminalTreeState {
  rootPane: PaneNode;
  activePaneId: string;
}

export interface NewTabOptions {
  projectId: string;
  projectPath: string;
  cliTool?: CliTool;
  workspacePath?: string;
  title?: string;
  /** 恢复已有会话 id（明文 → CLI resume）；缺省 = 新会话。 */
  resumeId?: string;
}

export type TerminalAction =
  | { type: 'split'; paneId: string; direction: SplitDirection }
  | { type: 'closePane'; paneId: string }
  | { type: 'addTab'; paneId: string; opts: NewTabOptions }
  | { type: 'closeTab'; paneId: string; tabId: string }
  | { type: 'selectTab'; paneId: string; tabId: string }
  | { type: 'setActivePane'; paneId: string }
  | { type: 'resizePanes'; paneId: string; sizes: number[] }
  | { type: 'setTabSession'; paneId: string; tabId: string; sessionId: string };

const DIRECTION_MAP: Record<SplitDirection, 'horizontal' | 'vertical'> = {
  right: 'horizontal',
  down: 'vertical',
};

// 折叠单子 split / 清理空 split + 重整 sizes —— 忠实移植 usePanesStore.normalizePaneTree。
// 注意：原地修改 root.children 并可能返回某个子节点（折叠），调用方须接住返回值。
function normalizePaneTree(root: PaneNode): PaneNode {
  if (root.type === 'panel') return root;
  root.children = root.children.map((child) => normalizePaneTree(child));
  if (root.children.length === 0) return createPanel();
  if (root.children.length === 1) return root.children[0]!;
  const total = root.sizes.reduce((sum, size) => sum + size, 0);
  root.sizes =
    total > 0
      ? root.sizes.map((size) => (size / total) * 100)
      : root.children.map(() => 100 / root.children.length);
  return root;
}

/** 初始状态：单个空面板（无 Tab），活动面板即该面板。终端页打开即空态，由 WorkspaceTree 启动会话。 */
export function createInitialTreeState(): TerminalTreeState {
  const panel = createEmptyPanel();
  return { rootPane: panel, activePaneId: panel.id };
}

export function terminalTreeReducer(
  state: TerminalTreeState,
  action: TerminalAction,
): TerminalTreeState {
  switch (action.type) {
    case 'setActivePane': {
      if (action.paneId === state.activePaneId) return state;
      return { ...state, activePaneId: action.paneId };
    }

    case 'split': {
      const splitDirection = DIRECTION_MAP[action.direction];
      const root = structuredClone(state.rootPane);
      const parentResult = findParent(root, action.paneId);
      if (!parentResult) return state;
      const targetPane = findPane(root, action.paneId);
      if (!targetPane || targetPane.type !== 'panel') return state;

      const newPane = createPanel();
      let nextRoot = root;

      if (parentResult.parent === null) {
        nextRoot = {
          type: 'split',
          id: generateId('split'),
          direction: splitDirection,
          children: [targetPane, newPane],
          sizes: [50, 50],
        };
      } else {
        const parent = parentResult.parent;
        const index = parentResult.index;
        if (parent.direction === splitDirection) {
          parent.children.splice(index + 1, 0, newPane);
          const newSize = 100 / parent.children.length;
          parent.sizes = parent.children.map(() => newSize);
        } else {
          parent.children[index] = {
            type: 'split',
            id: generateId('split'),
            direction: splitDirection,
            children: [targetPane, newPane],
            sizes: [50, 50],
          };
        }
      }
      return { rootPane: nextRoot, activePaneId: newPane.id };
    }

    case 'closePane': {
      const root = structuredClone(state.rootPane);
      const parentResult = findParent(root, action.paneId);
      if (!parentResult) return state;

      if (parentResult.parent === null) {
        const newPane = createEmptyPanel();
        return { rootPane: newPane, activePaneId: newPane.id };
      }

      const parent = parentResult.parent;
      const index = parentResult.index;
      parent.children.splice(index, 1);
      parent.sizes.splice(index, 1);

      const total = parent.sizes.reduce((a, b) => a + b, 0);
      parent.sizes =
        total > 0
          ? parent.sizes.map((s) => (s / total) * 100)
          : parent.sizes.map(() => 100 / parent.sizes.length);

      let activePaneId = state.activePaneId;
      if (parent.children.length > 0) {
        const newIndex = Math.min(index, parent.children.length - 1);
        const nextPane = parent.children[newIndex]!;
        const panels = collectPanels(nextPane);
        if (panels.length > 0) activePaneId = panels[0]!.id;
      }

      const nextRoot = normalizePaneTree(root);
      const activePane = findPane(nextRoot, activePaneId);
      if (!activePane || activePane.type !== 'panel') {
        const panels = collectPanels(nextRoot);
        if (panels.length > 0) activePaneId = panels[0]!.id;
      }
      return { rootPane: nextRoot, activePaneId };
    }

    case 'addTab': {
      const root = structuredClone(state.rootPane);
      const pane = findPane(root, action.paneId);
      if (!pane || pane.type !== 'panel') return state;
      const newTab = createTab(action.opts.projectId, action.opts.projectPath, {
        cliTool: action.opts.cliTool,
        workspacePath: action.opts.workspacePath,
        title: action.opts.title,
        resumeId: action.opts.resumeId,
      });
      pane.tabs.push(newTab);
      pane.activeTabId = newTab.id;
      return { ...state, rootPane: root };
    }

    case 'closeTab': {
      const snapPane = findPane(state.rootPane, action.paneId);
      if (!snapPane || snapPane.type !== 'panel') return state;
      const snapTab = snapPane.tabs.find((t) => t.id === action.tabId);
      if (!snapTab) return state;

      // 最后一个 Tab → 关闭整个 Panel（复用 closePane 逻辑，含折叠）。
      if (snapPane.tabs.length <= 1) {
        return terminalTreeReducer(state, { type: 'closePane', paneId: action.paneId });
      }

      const root = structuredClone(state.rootPane);
      const p = findPane(root, action.paneId);
      if (!p || p.type !== 'panel') return state;
      const idx = p.tabs.findIndex((t) => t.id === action.tabId);
      if (idx === -1) return state;
      p.tabs.splice(idx, 1);
      if (p.activeTabId === action.tabId) {
        const newIdx = Math.min(idx, p.tabs.length - 1);
        p.activeTabId = p.tabs[newIdx]!.id;
      }
      return { ...state, rootPane: root };
    }

    case 'selectTab': {
      const root = structuredClone(state.rootPane);
      const pane = findPane(root, action.paneId);
      if (!pane || pane.type !== 'panel') return state;
      pane.activeTabId = action.tabId;
      return { rootPane: root, activePaneId: action.paneId };
    }

    case 'resizePanes': {
      const root = structuredClone(state.rootPane);
      const pane = findPane(root, action.paneId);
      if (!pane || pane.type !== 'split') return state;
      // 拷贝入参数组：reducer 须独占 state，不与调用方共享可变引用
      // （cc-pane 靠 immer 冻结产物达成同样保护；本实现弃 immer 故显式拷贝）。
      pane.sizes = [...action.sizes];
      return { ...state, rootPane: root };
    }

    case 'setTabSession': {
      const root = structuredClone(state.rootPane);
      const pane = findPane(root, action.paneId);
      if (!pane || pane.type !== 'panel') return state;
      const tab = pane.tabs.find((t) => t.id === action.tabId);
      if (!tab) return state;
      tab.sessionId = action.sessionId;
      // MVP：terminalRootPane 为单 leaf，回填其 sessionId（Tab 级与 leaf 级协同）。
      if (tab.terminalRootPane && tab.terminalRootPane.type === 'leaf') {
        tab.terminalRootPane.sessionId = action.sessionId;
      }
      return { ...state, rootPane: root };
    }

    default:
      return state;
  }
}
