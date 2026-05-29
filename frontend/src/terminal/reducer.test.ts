import { describe, expect, it } from 'vitest';
import { collectPanels } from './paneTree';
import {
  createInitialTreeState,
  terminalTreeReducer,
  type TerminalTreeState,
} from './reducer';
import type { Panel, SplitPane } from './types';

// ── 测试辅助 ──

function rootSplit(state: TerminalTreeState): SplitPane {
  if (state.rootPane.type !== 'split') throw new Error('rootPane is not a split');
  return state.rootPane;
}
function rootPanel(state: TerminalTreeState): Panel {
  if (state.rootPane.type !== 'panel') throw new Error('rootPane is not a panel');
  return state.rootPane;
}
function panels(state: TerminalTreeState): Panel[] {
  return collectPanels(state.rootPane);
}

// ── 初始状态 ──

describe('createInitialTreeState', () => {
  it('单面板 + 单空 Tab，活动面板即该面板', () => {
    const s = createInitialTreeState();
    const p = rootPanel(s);
    expect(p.tabs).toHaveLength(1);
    expect(s.activePaneId).toBe(p.id);
    expect(p.activeTabId).toBe(p.tabs[0]!.id);
  });
});

// ── Tab 操作 ──

describe('addTab', () => {
  it('追加 Tab 并激活，且不改动原状态（纯函数）', () => {
    const s0 = createInitialTreeState();
    const before = rootPanel(s0).tabs.length;
    const s1 = terminalTreeReducer(s0, {
      type: 'addTab',
      paneId: rootPanel(s0).id,
      opts: { projectId: 'proj1', projectPath: '/a/proj', cliTool: 'claude' },
    });
    expect(rootPanel(s1).tabs).toHaveLength(before + 1);
    expect(rootPanel(s1).activeTabId).toBe(rootPanel(s1).tabs[1]!.id);
    expect(rootPanel(s1).tabs[1]!.cliTool).toBe('claude');
    // 原状态不被改动
    expect(rootPanel(s0).tabs).toHaveLength(before);
    expect(s1).not.toBe(s0);
  });

  it('面板不存在时原样返回', () => {
    const s0 = createInitialTreeState();
    const s1 = terminalTreeReducer(s0, {
      type: 'addTab',
      paneId: 'nope',
      opts: { projectId: 'p', projectPath: '/a' },
    });
    expect(s1).toBe(s0);
  });
});

describe('selectTab / setActivePane', () => {
  it('selectTab 设置 activeTabId 并把面板设为活动面板', () => {
    const s0 = createInitialTreeState();
    const pid = rootPanel(s0).id;
    const withTab = terminalTreeReducer(s0, {
      type: 'addTab',
      paneId: pid,
      opts: { projectId: 'p', projectPath: '/a' },
    });
    const firstTabId = rootPanel(withTab).tabs[0]!.id;
    const s2 = terminalTreeReducer(withTab, { type: 'selectTab', paneId: pid, tabId: firstTabId });
    expect(rootPanel(s2).activeTabId).toBe(firstTabId);
    expect(s2.activePaneId).toBe(pid);
  });

  it('setActivePane 相同 id 返回同一引用（避免无谓重渲染）', () => {
    const s0 = createInitialTreeState();
    const same = terminalTreeReducer(s0, { type: 'setActivePane', paneId: s0.activePaneId });
    expect(same).toBe(s0);
  });
});

// ── 分屏 split ──

describe('split', () => {
  it('对根面板向右分屏 → 根变为水平 split[orig, new]，激活新面板', () => {
    const s0 = createInitialTreeState();
    const orig = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, { type: 'split', paneId: orig, direction: 'right' });
    const sp = rootSplit(s1);
    expect(sp.direction).toBe('horizontal');
    expect(sp.children).toHaveLength(2);
    expect(sp.sizes).toEqual([50, 50]);
    expect((sp.children[0] as Panel).id).toBe(orig);
    // 激活的是新面板（第二个 child）
    expect(s1.activePaneId).toBe((sp.children[1] as Panel).id);
  });

  it('同方向再次分屏 → 合并进同一 split（3 子等分），不再嵌套', () => {
    const s0 = createInitialTreeState();
    const orig = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, { type: 'split', paneId: orig, direction: 'right' });
    const s2 = terminalTreeReducer(s1, { type: 'split', paneId: orig, direction: 'right' });
    const sp = rootSplit(s2);
    expect(sp.children).toHaveLength(3);
    expect(sp.children.every((c) => c.type === 'panel')).toBe(true);
    sp.sizes.forEach((sz) => expect(sz).toBeCloseTo(100 / 3, 5));
  });

  it('异方向分屏 → 在该面板处嵌套一层新方向 split', () => {
    const s0 = createInitialTreeState();
    const orig = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, { type: 'split', paneId: orig, direction: 'right' });
    const s2 = terminalTreeReducer(s1, { type: 'split', paneId: orig, direction: 'down' });
    const sp = rootSplit(s2);
    expect(sp.direction).toBe('horizontal');
    const nested = sp.children[0];
    expect(nested!.type).toBe('split');
    expect((nested as SplitPane).direction).toBe('vertical');
  });
});

// ── 关闭面板 closePane（含折叠） ──

describe('closePane', () => {
  it('关闭 split 的一个兄弟 → 折叠回单面板（normalize 单子 split）', () => {
    const s0 = createInitialTreeState();
    const orig = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, { type: 'split', paneId: orig, direction: 'right' });
    const newPaneId = s1.activePaneId;
    const s2 = terminalTreeReducer(s1, { type: 'closePane', paneId: newPaneId });
    expect(s2.rootPane.type).toBe('panel');
    expect(rootPanel(s2).id).toBe(orig);
    expect(s2.activePaneId).toBe(orig);
  });

  it('关闭根面板（无父）→ 重置为一个全新空面板', () => {
    const s0 = createInitialTreeState();
    const orig = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, { type: 'closePane', paneId: orig });
    expect(s1.rootPane.type).toBe('panel');
    expect(rootPanel(s1).id).not.toBe(orig);
    expect(s1.activePaneId).toBe(rootPanel(s1).id);
  });

  it('面板不存在时原样返回', () => {
    const s0 = createInitialTreeState();
    const s1 = terminalTreeReducer(s0, { type: 'closePane', paneId: 'nope' });
    expect(s1).toBe(s0);
  });
});

// ── 关闭 Tab ──

describe('closeTab', () => {
  it('非最后一个 Tab → 仅移除该 Tab 并修正 activeTabId', () => {
    const s0 = createInitialTreeState();
    const pid = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, {
      type: 'addTab',
      paneId: pid,
      opts: { projectId: 'p', projectPath: '/a' },
    });
    const activeTabId = rootPanel(s1).activeTabId; // 第二个 Tab
    const s2 = terminalTreeReducer(s1, { type: 'closeTab', paneId: pid, tabId: activeTabId });
    expect(rootPanel(s2).tabs).toHaveLength(1);
    expect(rootPanel(s2).activeTabId).toBe(rootPanel(s2).tabs[0]!.id);
  });

  it('最后一个 Tab → 连带关闭面板（split 内则折叠）', () => {
    const s0 = createInitialTreeState();
    const orig = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, { type: 'split', paneId: orig, direction: 'right' });
    const newPaneId = s1.activePaneId;
    const newPanel = panels(s1).find((p) => p.id === newPaneId)!;
    const onlyTab = newPanel.tabs[0]!.id;
    const s2 = terminalTreeReducer(s1, { type: 'closeTab', paneId: newPaneId, tabId: onlyTab });
    expect(s2.rootPane.type).toBe('panel');
    expect(rootPanel(s2).id).toBe(orig);
  });
});

// ── resize / 会话回填 ──

describe('resizePanes', () => {
  it('设置 split 子占比；对非 split 原样返回', () => {
    const s0 = createInitialTreeState();
    const orig = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, { type: 'split', paneId: orig, direction: 'right' });
    const splitId = rootSplit(s1).id;
    const s2 = terminalTreeReducer(s1, { type: 'resizePanes', paneId: splitId, sizes: [30, 70] });
    expect(rootSplit(s2).sizes).toEqual([30, 70]);

    const noop = terminalTreeReducer(s2, { type: 'resizePanes', paneId: s2.activePaneId, sizes: [1, 2] });
    expect(noop).toBe(s2); // 活动节点是 panel 非 split
  });
});

describe('setTabSession', () => {
  it('回填 Tab 级与单 leaf 的 sessionId', () => {
    const s0 = createInitialTreeState();
    const pid = rootPanel(s0).id;
    const tabId = rootPanel(s0).tabs[0]!.id;
    const s1 = terminalTreeReducer(s0, {
      type: 'setTabSession',
      paneId: pid,
      tabId,
      sessionId: 'sess-123',
    });
    const tab = rootPanel(s1).tabs[0]!;
    expect(tab.sessionId).toBe('sess-123');
    expect(tab.terminalRootPane?.type).toBe('leaf');
    if (tab.terminalRootPane?.type === 'leaf') {
      expect(tab.terminalRootPane.sessionId).toBe('sess-123');
    }
    // 原状态未被改动
    expect(rootPanel(s0).tabs[0]!.sessionId).toBeNull();
  });
});

// ── 深层/多子树覆盖（保真度审查 coverage 补强：cc-pane 移植最易藏 bug 的路径）──

describe('深层与多子分屏', () => {
  // 构造 3 子水平 split：children = [A, P2, P1]，A=orig，P2=第二次 split 的新面板（中间）。
  function threePaneSplit(): { state: TerminalTreeState; orig: string; middle: string; last: string } {
    const s0 = createInitialTreeState();
    const orig = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, { type: 'split', paneId: orig, direction: 'right' });
    const last = s1.activePaneId; // P1 —— 同方向再分屏后被挤到末位
    const s2 = terminalTreeReducer(s1, { type: 'split', paneId: orig, direction: 'right' });
    const middle = s2.activePaneId; // P2 —— splice 到 index 1
    return { state: s2, orig, middle, last };
  }

  it('关闭 3 子 split 的中间子 → 余 2 子且 sizes 重整到和为 100（3→2 renorm 路径）', () => {
    const { state, middle } = threePaneSplit();
    const splitId = rootSplit(state).id;
    const resized = terminalTreeReducer(state, {
      type: 'resizePanes',
      paneId: splitId,
      sizes: [20, 50, 30],
    });
    const closed = terminalTreeReducer(resized, { type: 'closePane', paneId: middle });
    const sp = rootSplit(closed);
    expect(sp.children).toHaveLength(2);
    expect(collectPanels(closed.rootPane).some((p) => p.id === middle)).toBe(false);
    expect(sp.sizes.reduce((a, b) => a + b, 0)).toBeCloseTo(100, 5);
    expect(sp.sizes).toEqual([40, 60]); // [20,30] 归一化
  });

  it('关闭 3 子 split 的末位子 → activePaneId 钳到新末位（Math.min 分支）', () => {
    const { state, middle, last } = threePaneSplit();
    const closed = terminalTreeReducer(state, { type: 'closePane', paneId: last });
    expect(rootSplit(closed).children).toHaveLength(2);
    expect(closed.activePaneId).toBe(middle); // newIndex=min(2,1)=1 → 新末位
  });

  it('activePaneId 兜底：关闭后焦点兄弟是 split → collectPanels 下钻到首个叶子面板', () => {
    // 构造 hsplit[A, vsplit(P1,P2)]，关闭 A → 折叠为 vsplit，活动面板须落在真实面板 P1。
    const s0 = createInitialTreeState();
    const a = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, { type: 'split', paneId: a, direction: 'right' });
    const p1 = s1.activePaneId;
    const s2 = terminalTreeReducer(s1, { type: 'split', paneId: p1, direction: 'down' });
    const closed = terminalTreeReducer(s2, { type: 'closePane', paneId: a });
    expect(closed.rootPane.type).toBe('split');
    expect((closed.rootPane as SplitPane).direction).toBe('vertical');
    // 活动面板是真实 panel（不是 split id / 悬空 id）
    const active = collectPanels(closed.rootPane).find((p) => p.id === closed.activePaneId);
    expect(active).toBeDefined();
    expect(active!.type).toBe('panel');
    expect(closed.activePaneId).toBe(p1);
  });

  it('非根父节点的同方向合并：深层面板再分屏并入同一 split（不嵌套第 4 层）', () => {
    const s0 = createInitialTreeState();
    const a = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, { type: 'split', paneId: a, direction: 'right' });
    const p1 = s1.activePaneId;
    const s2 = terminalTreeReducer(s1, { type: 'split', paneId: p1, direction: 'down' }); // vsplit[P1,P2]
    const p2 = s2.activePaneId;
    const s3 = terminalTreeReducer(s2, { type: 'split', paneId: p2, direction: 'down' }); // 应并入 vsplit
    const outer = rootSplit(s3);
    const inner = outer.children[1];
    expect(inner!.type).toBe('split');
    expect((inner as SplitPane).direction).toBe('vertical');
    expect((inner as SplitPane).children).toHaveLength(3); // 合并而非再嵌套
    (inner as SplitPane).sizes.forEach((sz) => expect(sz).toBeCloseTo(100 / 3, 5));
  });

  it('closeTab 非折叠路径：split 内多 Tab 面板关闭一个 Tab，树结构不变', () => {
    const s0 = createInitialTreeState();
    const a = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, { type: 'split', paneId: a, direction: 'right' });
    const p1 = s1.activePaneId;
    const s2 = terminalTreeReducer(s1, {
      type: 'addTab',
      paneId: p1,
      opts: { projectId: 'p', projectPath: '/a' },
    });
    const p1panel = collectPanels(s2.rootPane).find((p) => p.id === p1)!;
    const activeTab = p1panel.activeTabId;
    const s3 = terminalTreeReducer(s2, { type: 'closeTab', paneId: p1, tabId: activeTab });
    expect(s3.rootPane.type).toBe('split'); // 未折叠
    const after = collectPanels(s3.rootPane).find((p) => p.id === p1)!;
    expect(after.tabs).toHaveLength(1);
    expect(after.activeTabId).toBe(after.tabs[0]!.id);
  });

  it('resizePanes 作用于嵌套 split 不影响兄弟 split 占比', () => {
    const s0 = createInitialTreeState();
    const a = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, { type: 'split', paneId: a, direction: 'right' });
    const p1 = s1.activePaneId;
    const s2 = terminalTreeReducer(s1, { type: 'split', paneId: p1, direction: 'down' });
    const outerId = rootSplit(s2).id;
    const innerId = (rootSplit(s2).children[1] as SplitPane).id;
    const s3 = terminalTreeReducer(s2, { type: 'resizePanes', paneId: innerId, sizes: [25, 75] });
    const inner = rootSplit(s3).children[1] as SplitPane;
    expect(inner.sizes).toEqual([25, 75]);
    expect(rootSplit(s3).id).toBe(outerId);
    expect(rootSplit(s3).sizes).toEqual([50, 50]); // 外层未受影响
  });
});
