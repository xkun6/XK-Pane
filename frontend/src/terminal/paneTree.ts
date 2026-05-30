// 面板树纯函数 —— 忠实移植 cc-pane `web/stores/paneTreeHelpers.ts`。
// 全部为无副作用纯函数（仅依赖入参 + crypto.randomUUID），便于将来 Vitest 覆盖。
// 与 cc-pane 的差异（MVP 有意调整，非 drift）：createTab 的可选参从 cc-pane 的
// resumeId/workspaceName/providerId（Phase 3 字段）换成 MVP 相关的 cliTool/workspacePath/title。
import type {
  CliTool,
  PaneNode,
  Panel,
  SplitPane,
  Tab,
  TerminalPaneLeaf,
} from './types';

/** 生成带前缀的唯一 id。 */
export function generateId(prefix: string): string {
  return `${prefix}-${crypto.randomUUID()}`;
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
  opts: { cliTool?: CliTool; workspacePath?: string; title?: string } = {},
): Tab {
  const name = projectPath.split(/[/\\]/).pop() || 'Terminal';
  let title = opts.title ?? name;
  if (!opts.title && opts.cliTool && opts.cliTool !== 'none') {
    const toolLabel = opts.cliTool.charAt(0).toUpperCase() + opts.cliTool.slice(1);
    title = `${name} (${toolLabel})`;
  }
  const leaf: TerminalPaneLeaf = {
    type: 'leaf',
    id: generateId('terminal-pane'),
    sessionId: null,
    cliTool: opts.cliTool,
    workspacePath: opts.workspacePath,
  };
  return {
    id: generateId('tab'),
    title,
    contentType: 'terminal',
    projectId,
    projectPath,
    sessionId: null,
    cliTool: opts.cliTool,
    workspacePath: opts.workspacePath,
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
