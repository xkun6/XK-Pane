// PTY 会话「期望态 vs 实际态」对账的纯函数 —— 供 TerminalProvider 的 reconcile effect 使用。
// 这里只做无副作用的集合/筛选计算（可 Vitest 覆盖）；真正的 create/kill/dispatch 在 Provider 里。
import { collectPanels } from './paneTree';
import type { PaneNode } from './types';
import type { SSHSpec } from '@/services/terminal/TerminalService';

export interface TabWithPane {
  paneId: string;
  tabId: string;
  projectPath: string;
  workspacePath?: string;
  cliTool?: string;
  resumeId?: string;
  runtimeKind?: string;
  ssh?: SSHSpec;
  sessionId: string | null;
}

/** 扁平化外层树为 (paneId, tab) 列表 —— reconcile 遍历入口。 */
export function collectTabsWithPane(root: PaneNode): TabWithPane[] {
  return collectPanels(root).flatMap((panel) =>
    panel.tabs.map((tab) => ({
      paneId: panel.id,
      tabId: tab.id,
      projectPath: tab.projectPath,
      workspacePath: tab.workspacePath,
      cliTool: tab.cliTool,
      resumeId: tab.resumeId,
      runtimeKind: tab.runtimeKind,
      ssh: tab.ssh,
      sessionId: tab.sessionId,
    })),
  );
}

/**
 * 需要创建 PTY 会话的 tab：有 projectPath（空 projectPath 后端 resolveLaunch 会报错，跳过）
 * 且尚无 session 且不在创建中。
 */
export function tabsNeedingSession(
  tabs: TabWithPane[],
  creating: ReadonlySet<string>,
): TabWithPane[] {
  return tabs.filter(
    (t) => t.projectPath !== '' && t.sessionId === null && !creating.has(t.tabId),
  );
}

/** 当前树中所有活动会话 id（非 null）。 */
export function treeSessionIds(tabs: TabWithPane[]): Set<string> {
  const out = new Set<string>();
  for (const t of tabs) if (t.sessionId) out.add(t.sessionId);
  return out;
}

/** 孤儿会话：上轮已知（曾出现在树里）但本轮已不在树中 → 应 kill。 */
export function orphanedSessionIds(
  prevKnown: ReadonlySet<string>,
  current: ReadonlySet<string>,
): string[] {
  const out: string[] = [];
  for (const sid of prevKnown) if (!current.has(sid)) out.push(sid);
  return out;
}
