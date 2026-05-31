// 终端外层面板树归约器 —— 忠实移植 cc-pane `web/stores/usePanesStore.ts` 的面板树操作。
//
// cc-pane 用 Zustand + immer（可变 draft 风格）；本实现遵循 D2（弃 Zustand，用纯 useReducer）。
// 移植手法 = clone-then-mutate：对 rootPane 做 structuredClone 后，套用 cc-pane 的可变逻辑，
// 再返回新树。这样 1:1 复刻 immer 的可变语义，同时保持 reducer 为纯函数（可 Vitest 覆盖）。
//
// 已移植的外层操作：split / closePane / addTab / closeTab / selectTab / setActivePane /
// resizePanes / setTabSession（生命周期回填钩子）+ 拖拽三件套 reorderTabs / moveTab /
// moveTabToEdge（P5 Tab 拖拽：任意 Tab 拖到任意面板的边 → 在其旁拆出新面板）+ 内层终端分屏
// splitTerminalPane / closeTerminalPane / resizeTerminalPanes / setActiveTerminalPane（单 Tab 内多 PTY）。
// 刻意未移植（依赖未落地子系统，勿现在硬塞——CLAUDE.md 简洁铁律）：
//   - closedTabs 可恢复快照 + syncPtyRegistryUnregister（会话回放/恢复子功能，后续）
//   - pinned 校验（UI 细节，后续）

import {
  cloneTerminalLeaf,
  collectPanels,
  collectTerminalLeaves,
  createEmptyPanel,
  createPanel,
  createTab,
  findPane,
  findParent,
  findTabLocation,
  findTerminalPane,
  findTerminalPaneParent,
  generateId,
} from './paneTree';
import type { CliTool, PaneEdge, PaneNode, Panel, SplitDirection, Tab } from './types';

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
  | { type: 'togglePinTab'; paneId: string; tabId: string }
  | { type: 'renameTab'; paneId: string; tabId: string; title: string }
  | { type: 'setActivePane'; paneId: string }
  | { type: 'resizePanes'; paneId: string; sizes: number[] }
  // —— P5 拖拽三件套 ——
  | { type: 'reorderTabs'; paneId: string; fromIndex: number; toIndex: number }
  | { type: 'moveTab'; fromPaneId: string; toPaneId: string; tabId: string; toIndex?: number }
  | {
      type: 'moveTabToEdge';
      fromPaneId: string;
      tabId: string;
      targetPaneId: string;
      edge: PaneEdge;
    }
  // —— P5 内层终端分屏（单 Tab 内多 PTY；操作其 terminalRootPane 子树）——
  | { type: 'splitTerminalPane'; tabId: string; terminalPaneId: string; direction: SplitDirection }
  | { type: 'closeTerminalPane'; tabId: string; terminalPaneId: string }
  | { type: 'resizeTerminalPanes'; tabId: string; terminalPaneId: string; sizes: number[] }
  | { type: 'setActiveTerminalPane'; tabId: string; terminalPaneId: string }
  // setTabSession 按 tabId 全树查找（不再依赖 paneId）—— Tab 可经 moveTab 跨面板迁移。
  // terminalPaneId 缺省回填活动 leaf；内层分屏后由 reconcile 指定具体 leaf。
  | { type: 'setTabSession'; tabId: string; sessionId: string; terminalPaneId?: string };

const DIRECTION_MAP: Record<SplitDirection, 'horizontal' | 'vertical'> = {
  right: 'horizontal',
  down: 'vertical',
};

// 四条边 → 分屏方向 + 新面板插在目标的前(after=false)/后(after=true)。
const EDGE_TO_SPLIT: Record<PaneEdge, { direction: 'horizontal' | 'vertical'; after: boolean }> = {
  left: { direction: 'horizontal', after: false },
  right: { direction: 'horizontal', after: true },
  top: { direction: 'vertical', after: false },
  bottom: { direction: 'vertical', after: true },
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

// 内层分屏后保持 Tab 级状态与「活动 leaf」一致：activeTerminalPaneId 指向存在的 leaf，
// 且 Tab 级 sessionId 镜像活动 leaf 的 sessionId（PanelView 当前按 tab.sessionId 渲染，需同步）。
// 对齐 cc-pane syncTabTerminalState 的核心（XK 无 provider/ssh 等 Phase 3/4 字段，故只同步 session）。
function syncTabActiveLeaf(tab: Tab): void {
  if (!tab.terminalRootPane) return;
  const leaves = collectTerminalLeaves(tab.terminalRootPane);
  if (leaves.length === 0) return;
  const active =
    (tab.activeTerminalPaneId
      ? leaves.find((l) => l.id === tab.activeTerminalPaneId)
      : undefined) ?? leaves[0]!;
  tab.activeTerminalPaneId = active.id;
  tab.sessionId = active.sessionId;
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
      if (!snapTab || snapTab.pinned) return state; // 置顶 Tab 不可关闭（对齐 cc-pane）

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

    case 'togglePinTab': {
      const root = structuredClone(state.rootPane);
      const pane = findPane(root, action.paneId);
      if (!pane || pane.type !== 'panel') return state;
      const tab = pane.tabs.find((t) => t.id === action.tabId);
      if (!tab) return state;
      tab.pinned = !tab.pinned;
      return { ...state, rootPane: root };
    }

    case 'renameTab': {
      const title = action.title.trim();
      if (!title) return state; // 空标题不改（由 UI 兜底，这里再守一道）
      const root = structuredClone(state.rootPane);
      const pane = findPane(root, action.paneId);
      if (!pane || pane.type !== 'panel') return state;
      const tab = pane.tabs.find((t) => t.id === action.tabId);
      if (!tab) return state;
      tab.title = title;
      return { ...state, rootPane: root };
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

    case 'reorderTabs': {
      // 同面板内重排（拖拽落到同面板另一 Tab 上）—— 忠实移植 usePanesStore.reorderTabs。
      const { fromIndex, toIndex } = action;
      if (fromIndex === toIndex) return state;
      const root = structuredClone(state.rootPane);
      const pane = findPane(root, action.paneId);
      if (!pane || pane.type !== 'panel') return state;
      if (fromIndex < 0 || fromIndex >= pane.tabs.length) return state;
      if (toIndex < 0 || toIndex >= pane.tabs.length) return state;
      const [moved] = pane.tabs.splice(fromIndex, 1);
      pane.tabs.splice(toIndex, 0, moved!);
      return { ...state, rootPane: root };
    }

    case 'moveTab': {
      // 跨面板移动一个 Tab —— 忠实移植 usePanesStore.moveTab（含「源面板掏空 → 折叠」，
      // 但不做 closedTabs 快照：那属会话回放子功能，本期未落地）。
      const { fromPaneId, toPaneId, tabId, toIndex } = action;
      const root = structuredClone(state.rootPane);
      const fromPane = findPane(root, fromPaneId);
      const toPane = findPane(root, toPaneId);
      if (!fromPane || fromPane.type !== 'panel') return state;
      if (!toPane || toPane.type !== 'panel') return state;
      const tabIndex = fromPane.tabs.findIndex((t) => t.id === tabId);
      if (tabIndex === -1) return state;

      const [tab] = fromPane.tabs.splice(tabIndex, 1);
      const insertAt =
        toIndex !== undefined && toIndex >= 0
          ? Math.min(toIndex, toPane.tabs.length)
          : toPane.tabs.length;
      toPane.tabs.splice(insertAt, 0, tab!);
      toPane.activeTabId = tab!.id;

      // 修正源面板活动 Tab（仅跨面板时；同面板即重排，活动态已由上一行定为被移 Tab）。
      if (fromPane !== toPane && fromPane.tabs.length > 0) {
        const newIdx = Math.min(tabIndex, fromPane.tabs.length - 1);
        fromPane.activeTabId = fromPane.tabs[newIdx]!.id;
      }

      let nextRoot: PaneNode = root;
      // 源面板被掏空（且非同一面板）→ 从父 split 摘除 + 重整 sizes + 折叠单子 split。
      // 活动面板保持指向目标面板（区别于 closePane 的「焦点落到兄弟」语义）。
      if (fromPane !== toPane && fromPane.tabs.length === 0) {
        const pr = findParent(root, fromPaneId);
        if (pr && pr.parent) {
          const parent = pr.parent;
          parent.children.splice(pr.index, 1);
          parent.sizes.splice(pr.index, 1);
          const total = parent.sizes.reduce((a, b) => a + b, 0);
          parent.sizes =
            total > 0
              ? parent.sizes.map((s) => (s / total) * 100)
              : parent.sizes.map(() => 100 / parent.sizes.length);
        }
        nextRoot = normalizePaneTree(root);
      }
      return { rootPane: nextRoot, activePaneId: toPaneId };
    }

    case 'moveTabToEdge': {
      // VSCode 式：把任意 Tab 拖到「任意目标面板」的某条边 → 在该面板旁拆出新面板装它。
      // 同面板（from===target）= 从自己拽一个 Tab 出来分屏（要求 >1 个 Tab）；
      // 跨面板 = 从源面板移出（源被掏空则折叠消失），在目标面板旁新建。
      const { fromPaneId, tabId, targetPaneId, edge } = action;
      const root = structuredClone(state.rootPane);
      const fromPane = findPane(root, fromPaneId);
      if (!fromPane || fromPane.type !== 'panel') return state;
      const tabIndex = fromPane.tabs.findIndex((t) => t.id === tabId);
      if (tabIndex === -1) return state;
      // 同面板时绝不移出唯一 Tab（移出后源空了即折叠，等价无意义的原地分屏）。
      if (fromPaneId === targetPaneId && fromPane.tabs.length <= 1) return state;

      const [tab] = fromPane.tabs.splice(tabIndex, 1);
      if (fromPane.activeTabId === tabId && fromPane.tabs.length > 0) {
        const newIdx = Math.min(tabIndex, fromPane.tabs.length - 1);
        fromPane.activeTabId = fromPane.tabs[newIdx]!.id;
      }

      const newPane: Panel = {
        type: 'panel',
        id: generateId('pane'),
        tabs: [tab!],
        activeTabId: tab!.id,
      };
      const { direction: splitDirection, after } = EDGE_TO_SPLIT[edge];

      const targetPane = findPane(root, targetPaneId);
      if (!targetPane || targetPane.type !== 'panel') return state;
      const tpr = findParent(root, targetPaneId);
      if (!tpr) return state;
      const ordered: PaneNode[] = after ? [targetPane, newPane] : [newPane, targetPane];

      let nextRoot: PaneNode = root;
      if (tpr.parent === null) {
        // 目标是根面板（整棵树仅此一个面板）→ from 必 === target，升级为一层 split。
        nextRoot = {
          type: 'split',
          id: generateId('split'),
          direction: splitDirection,
          children: ordered,
          sizes: [50, 50],
        };
      } else if (tpr.parent.direction === splitDirection) {
        const at = after ? tpr.index + 1 : tpr.index;
        tpr.parent.children.splice(at, 0, newPane);
        const newSize = 100 / tpr.parent.children.length;
        tpr.parent.sizes = tpr.parent.children.map(() => newSize);
      } else {
        tpr.parent.children[tpr.index] = {
          type: 'split',
          id: generateId('split'),
          direction: splitDirection,
          children: ordered,
          sizes: [50, 50],
        };
      }

      // 跨面板且源面板被掏空 → 从其父 split 摘除 + 重整 sizes（随后 normalize 折叠单子 split）。
      if (fromPaneId !== targetPaneId && fromPane.tabs.length === 0) {
        const fpr = findParent(nextRoot, fromPaneId);
        if (fpr && fpr.parent) {
          const parent = fpr.parent;
          parent.children.splice(fpr.index, 1);
          parent.sizes.splice(fpr.index, 1);
          const total = parent.sizes.reduce((a, b) => a + b, 0);
          parent.sizes =
            total > 0
              ? parent.sizes.map((s) => (s / total) * 100)
              : parent.sizes.map(() => 100 / parent.sizes.length);
        }
      }

      return { rootPane: normalizePaneTree(nextRoot), activePaneId: newPane.id };
    }

    case 'splitTerminalPane': {
      // 在单个 Tab 内把某终端 leaf 拆成两个（克隆出新 leaf，新 leaf 起同类终端）—— 忠实移植 usePanesStore。
      const root = structuredClone(state.rootPane);
      const loc = findTabLocation(root, action.tabId);
      if (!loc) return state;
      const tab = loc.tab;
      if (!tab.terminalRootPane) return state;
      const target = findTerminalPane(tab.terminalRootPane, action.terminalPaneId);
      if (!target || target.type !== 'leaf') return state;

      const newLeaf = cloneTerminalLeaf(target);
      const splitDirection = DIRECTION_MAP[action.direction];
      const pr = findTerminalPaneParent(tab.terminalRootPane, action.terminalPaneId);
      if (!pr || pr.parent === null) {
        tab.terminalRootPane = {
          type: 'split',
          id: generateId('terminal-split'),
          direction: splitDirection,
          children: [target, newLeaf],
          sizes: [50, 50],
        };
      } else if (pr.parent.direction === splitDirection) {
        pr.parent.children.splice(pr.index + 1, 0, newLeaf);
        const newSize = 100 / pr.parent.children.length;
        pr.parent.sizes = pr.parent.children.map(() => newSize);
      } else {
        pr.parent.children[pr.index] = {
          type: 'split',
          id: generateId('terminal-split'),
          direction: splitDirection,
          children: [target, newLeaf],
          sizes: [50, 50],
        };
      }
      tab.activeTerminalPaneId = newLeaf.id;
      syncTabActiveLeaf(tab);
      return { ...state, rootPane: root };
    }

    case 'closeTerminalPane': {
      // 关闭单个 Tab 内的一个终端 leaf（折叠单子 split）—— 忠实移植 usePanesStore；至少留一个 leaf。
      const root = structuredClone(state.rootPane);
      const loc = findTabLocation(root, action.tabId);
      if (!loc) return state;
      const tab = loc.tab;
      if (!tab.terminalRootPane) return state;
      if (collectTerminalLeaves(tab.terminalRootPane).length <= 1) return state;
      const pr = findTerminalPaneParent(tab.terminalRootPane, action.terminalPaneId);
      if (!pr || pr.parent === null) return state;

      const parent = pr.parent;
      parent.children.splice(pr.index, 1);
      parent.sizes.splice(pr.index, 1);
      if (parent.children.length === 1) {
        const gp = findTerminalPaneParent(tab.terminalRootPane, parent.id);
        if (!gp || gp.parent === null) {
          tab.terminalRootPane = parent.children[0]!;
        } else {
          gp.parent.children[gp.index] = parent.children[0]!;
        }
      } else {
        const total = parent.sizes.reduce((a, b) => a + b, 0);
        parent.sizes =
          total > 0
            ? parent.sizes.map((s) => (s / total) * 100)
            : parent.sizes.map(() => 100 / parent.sizes.length);
      }
      const nextLeaves = collectTerminalLeaves(tab.terminalRootPane);
      tab.activeTerminalPaneId = nextLeaves[Math.min(pr.index, nextLeaves.length - 1)]?.id;
      syncTabActiveLeaf(tab);
      return { ...state, rootPane: root };
    }

    case 'resizeTerminalPanes': {
      const root = structuredClone(state.rootPane);
      const loc = findTabLocation(root, action.tabId);
      if (!loc || !loc.tab.terminalRootPane) return state;
      const node = findTerminalPane(loc.tab.terminalRootPane, action.terminalPaneId);
      if (!node || node.type !== 'split') return state;
      node.sizes = [...action.sizes];
      return { ...state, rootPane: root };
    }

    case 'setActiveTerminalPane': {
      const root = structuredClone(state.rootPane);
      const loc = findTabLocation(root, action.tabId);
      if (!loc) return state;
      const tab = loc.tab;
      if (!tab.terminalRootPane) return state;
      if (!findTerminalPane(tab.terminalRootPane, action.terminalPaneId)) return state;
      tab.activeTerminalPaneId = action.terminalPaneId;
      syncTabActiveLeaf(tab);
      return { ...state, rootPane: root };
    }

    case 'setTabSession': {
      // 按 tabId 全树定位（不依赖 paneId）—— Tab 经 moveTab 迁移后旧 paneId 失效，
      // 按 tabId 查当前 pane 才能正确回填，避免 session 泄漏（cc-pane updateTabSession 即如此）。
      const root = structuredClone(state.rootPane);
      const loc = findTabLocation(root, action.tabId);
      if (!loc) return state;
      const tab = loc.tab;
      if (tab.terminalRootPane) {
        // 回填到指定 leaf（reconcile 按 leaf 创建会话时指定）；缺省回填活动 leaf。
        const leaves = collectTerminalLeaves(tab.terminalRootPane);
        const targetId = action.terminalPaneId ?? tab.activeTerminalPaneId;
        const leaf =
          (targetId ? leaves.find((l) => l.id === targetId) : undefined) ??
          (tab.terminalRootPane.type === 'leaf' ? tab.terminalRootPane : leaves[0]);
        if (leaf && leaf.type === 'leaf') leaf.sessionId = action.sessionId;
        syncTabActiveLeaf(tab);
      } else {
        tab.sessionId = action.sessionId;
      }
      return { ...state, rootPane: root };
    }

    default:
      return state;
  }
}
