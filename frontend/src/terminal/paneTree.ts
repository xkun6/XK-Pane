// 面板树纯函数 —— 忠实移植 cc-pane `web/stores/paneTreeHelpers.ts`。
// 全部为无副作用纯函数（仅依赖入参 + randomId），便于将来 Vitest 覆盖。
// 与 cc-pane 的差异（MVP 有意调整，非 drift）：createTab 的可选参从 cc-pane 的
// resumeId/workspaceName/providerId（Phase 3 字段）换成 MVP 相关的 cliTool/workspacePath/title。
// 相对路径（非 @ 别名）：paneTree 在 vitest 的 reducer/sessionReconcile 测试图里被传递 import，
// 而 vitest 不对「传递 import 的源文件」套用 vite 的 @ 别名 → 用相对路径才能在 build 与 test 都解析。
import { randomId } from '../shared/uuid';
import type {
  CliTool,
  PaneNode,
  Panel,
  SplitPane,
  Tab,
  TerminalPaneLeaf,
  TerminalPaneNode,
  TerminalPaneSplit,
} from './types';

/**
 * 生成带前缀的唯一 id。用 randomId（安全上下文兜底）而非裸 crypto.randomUUID ——
 * 后者在 LAN IP + HTTP（P7 Web 访问，非安全上下文）为 undefined，会在 TerminalProvider
 * 初始化建树时抛 TypeError 致整页挂载失败（卡启动骨架）。见 @/shared/uuid。
 */
export function generateId(prefix: string): string {
  return `${prefix}-${randomId()}`;
}

/** 创建一个新面板（默认含一个空终端 Tab）。 */
export function createPanel(tab?: Tab): Panel {
  const id = generateId('pane');
  const panelTab = tab ?? createTab('', '');
  return {
    type: 'panel',
    id,
    tabs: [panelTab],
    activeTabId: panelTab.id,
  };
}

/**
 * 创建一个空面板（无 Tab）—— 终端页初始态 / 关掉最后一个会话后回到的「空态」。
 * 空面板由 PanelView 渲染为「从左侧启动会话」提示，不凭空开一个无意义的空 shell。
 */
export function createEmptyPanel(): Panel {
  return {
    type: 'panel',
    id: generateId('pane'),
    tabs: [],
    activeTabId: '',
  };
}

/** 创建一个终端 Tab（含单 leaf 的 terminalRootPane）。 */
export function createTab(
  projectId: string,
  projectPath: string,
  opts: {
    cliTool?: CliTool;
    workspacePath?: string;
    title?: string;
    resumeId?: string;
    /** 后端已建会话 id（编排 launch_task 场景）；非空 = leaf/Tab 直接绑定，reconcile 不再建会话。 */
    sessionId?: string;
  } = {},
): Tab {
  const name = projectPath.split(/[/\\]/).pop() || 'Terminal';
  let title = opts.title ?? name;
  if (!opts.title && opts.cliTool && opts.cliTool !== 'none') {
    const toolLabel = opts.cliTool.charAt(0).toUpperCase() + opts.cliTool.slice(1);
    title = `${name} (${toolLabel})`;
  }
  const sessionId = opts.sessionId ?? null;
  const leaf: TerminalPaneLeaf = {
    type: 'leaf',
    id: generateId('terminal-pane'),
    sessionId,
    cliTool: opts.cliTool,
    workspacePath: opts.workspacePath,
    resumeId: opts.resumeId,
  };
  return {
    id: generateId('tab'),
    title,
    contentType: 'terminal',
    projectId,
    projectPath,
    sessionId,
    cliTool: opts.cliTool,
    workspacePath: opts.workspacePath,
    resumeId: opts.resumeId,
    terminalRootPane: leaf,
    activeTerminalPaneId: leaf.id,
  };
}

/** 递归查找面板节点。 */
export function findPane(node: PaneNode, paneId: string): PaneNode | null {
  if (node.id === paneId) return node;
  if (node.type === 'split') {
    for (const child of node.children) {
      const found = findPane(child, paneId);
      if (found) return found;
    }
  }
  return null;
}

/** 查找某节点的父 split 与其在 children 中的下标。根节点返回 {parent:null,index:-1}。 */
export function findParent(
  node: PaneNode,
  paneId: string,
  parent: SplitPane | null = null,
): { parent: SplitPane | null; index: number } | null {
  if (node.id === paneId) {
    return { parent, index: parent ? parent.children.indexOf(node) : -1 };
  }
  if (node.type === 'split') {
    for (let i = 0; i < node.children.length; i++) {
      const child = node.children[i];
      if (!child) continue; // noUncheckedIndexedAccess 守门
      const result = findParent(child, paneId, node);
      if (result) return result;
    }
  }
  return null;
}

/** 扁平化收集所有 Panel（叶子面板）。 */
export function collectPanels(node: PaneNode): Panel[] {
  if (node.type === 'panel') return [node];
  return node.children.flatMap(collectPanels);
}

// ── 单个 Tab 内的「终端分屏树」助手（忠实移植 cc-pane usePanesStore 同名函数）──

/** 递归查找单个 Tab 内的终端分屏节点（leaf / split）。 */
export function findTerminalPane(node: TerminalPaneNode, paneId: string): TerminalPaneNode | null {
  if (node.id === paneId) return node;
  if (node.type === 'split') {
    for (const child of node.children) {
      const found = findTerminalPane(child, paneId);
      if (found) return found;
    }
  }
  return null;
}

/** 查找终端分屏节点的父 split 与其下标；根节点返回 {parent:null,index:-1}。 */
export function findTerminalPaneParent(
  node: TerminalPaneNode,
  paneId: string,
  parent: TerminalPaneSplit | null = null,
): { parent: TerminalPaneSplit | null; index: number } | null {
  if (node.id === paneId) {
    return { parent, index: parent ? parent.children.indexOf(node) : -1 };
  }
  if (node.type === 'split') {
    for (let i = 0; i < node.children.length; i++) {
      const child = node.children[i];
      if (!child) continue; // noUncheckedIndexedAccess 守门
      const result = findTerminalPaneParent(child, paneId, node);
      if (result) return result;
    }
  }
  return null;
}

/** 收集一个终端分屏树里的所有 leaf（含持有的 sessionId）。 */
export function collectTerminalLeaves(node?: TerminalPaneNode): TerminalPaneLeaf[] {
  if (!node) return [];
  if (node.type === 'leaf') return [node];
  return node.children.flatMap(collectTerminalLeaves);
}

/** 克隆一个 leaf 作为新分屏：新 id、清空 sessionId，保留 cliTool/workspacePath/resumeId 以启同类终端。 */
export function cloneTerminalLeaf(source: TerminalPaneLeaf): TerminalPaneLeaf {
  return { ...source, id: generateId('terminal-pane'), sessionId: null };
}

/**
 * 全树按 tabId 定位其所在 Panel 与 Tab（cc-pane `findTabLocation` 等价）。
 * 用于 setTabSession 等「只知 tabId、不知当前 pane」的场景 —— Tab 经 moveTab 跨面板迁移后，
 * 旧 paneId 已失效，必须按 tabId 重新定位，否则回填落到旧 pane 会 no-op（session 泄漏）。
 */
export function findTabLocation(node: PaneNode, tabId: string): { panel: Panel; tab: Tab } | null {
  for (const panel of collectPanels(node)) {
    const tab = panel.tabs.find((item) => item.id === tabId);
    if (tab) return { panel, tab };
  }
  return null;
}
