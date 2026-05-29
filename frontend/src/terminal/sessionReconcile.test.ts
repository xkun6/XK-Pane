import { describe, expect, it } from 'vitest';
import {
  collectTabsWithPane,
  orphanedSessionIds,
  tabsNeedingSession,
  treeSessionIds,
  type TabWithPane,
} from './sessionReconcile';
import type { PaneNode, Panel, Tab } from './types';

function mkTab(id: string, over: Partial<Tab> = {}): Tab {
  return {
    id,
    title: id,
    contentType: 'terminal',
    projectId: 'p',
    projectPath: '/proj',
    sessionId: null,
    ...over,
  };
}
function mkPanel(id: string, tabs: Tab[]): Panel {
  return { type: 'panel', id, tabs, activeTabId: tabs[0]?.id ?? '' };
}

describe('collectTabsWithPane', () => {
  it('扁平化 split 树为 (paneId, tabId) 列表，保持顺序', () => {
    const tree: PaneNode = {
      type: 'split',
      id: 's1',
      direction: 'horizontal',
      children: [mkPanel('pA', [mkTab('t1'), mkTab('t2')]), mkPanel('pB', [mkTab('t3')])],
      sizes: [50, 50],
    };
    const tabs = collectTabsWithPane(tree);
    expect(tabs.map((x) => x.tabId)).toEqual(['t1', 't2', 't3']);
    expect(tabs.map((x) => x.paneId)).toEqual(['pA', 'pA', 'pB']);
  });
});

describe('tabsNeedingSession', () => {
  it('仅取 有 projectPath + 无 session + 不在创建中 的 tab', () => {
    const tabs: TabWithPane[] = [
      { paneId: 'p', tabId: 'a', projectPath: '/x', sessionId: null }, // 需要
      { paneId: 'p', tabId: 'b', projectPath: '', sessionId: null }, // 空 projectPath → 跳过
      { paneId: 'p', tabId: 'c', projectPath: '/y', sessionId: 's1' }, // 已有 session → 跳过
      { paneId: 'p', tabId: 'd', projectPath: '/z', sessionId: null }, // 创建中 → 跳过
    ];
    const need = tabsNeedingSession(tabs, new Set(['d']));
    expect(need.map((t) => t.tabId)).toEqual(['a']);
  });
});

describe('treeSessionIds', () => {
  it('收集所有非 null sessionId', () => {
    const tabs: TabWithPane[] = [
      { paneId: 'p', tabId: 'a', projectPath: '/x', sessionId: 's1' },
      { paneId: 'p', tabId: 'b', projectPath: '/y', sessionId: null },
      { paneId: 'p', tabId: 'c', projectPath: '/z', sessionId: 's2' },
    ];
    expect([...treeSessionIds(tabs)].sort()).toEqual(['s1', 's2']);
  });
});

describe('orphanedSessionIds', () => {
  it('上轮已知但本轮不在 → 孤儿', () => {
    expect(orphanedSessionIds(new Set(['a', 'b', 'c']), new Set(['b', 'c', 'd'])).sort()).toEqual([
      'a',
    ]);
  });
  it('无孤儿 → 空', () => {
    expect(orphanedSessionIds(new Set(['a']), new Set(['a', 'b']))).toEqual([]);
    expect(orphanedSessionIds(new Set(), new Set(['a']))).toEqual([]);
  });
});
