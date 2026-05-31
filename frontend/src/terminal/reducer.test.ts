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
  it('单空面板（无 Tab），活动面板即该面板', () => {
    const s = createInitialTreeState();
    const p = rootPanel(s);
    expect(p.tabs).toHaveLength(0);
    expect(s.activePaneId).toBe(p.id);
    expect(p.activeTabId).toBe('');
  });
});

// ── Tab 操作 ──

describe('addTab', () => {
  it('追加 Tab 并激活，且不改动原状态（纯函数）', () => {
    const s0 = createInitialTreeState();
    const pid = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, {
      type: 'addTab',
      paneId: pid,
      opts: { projectId: 'p0', projectPath: '/a/p0' },
    });
    const s2 = terminalTreeReducer(s1, {
      type: 'addTab',
      paneId: pid,
      opts: { projectId: 'proj1', projectPath: '/a/proj', cliTool: 'claude' },
    });
    // 追加到末尾并激活第二个
    expect(rootPanel(s2).tabs).toHaveLength(2);
    expect(rootPanel(s2).activeTabId).toBe(rootPanel(s2).tabs[1]!.id);
    expect(rootPanel(s2).tabs[1]!.cliTool).toBe('claude');
    // 原状态不被改动（s1 仍是 1 个 Tab）
    expect(rootPanel(s1).tabs).toHaveLength(1);
    expect(s2).not.toBe(s1);
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
    const s1 = terminalTreeReducer(s0, {
      type: 'addTab',
      paneId: pid,
      opts: { projectId: 'p1', projectPath: '/a' },
    });
    const firstTabId = rootPanel(s1).tabs[0]!.id;
    const s2 = terminalTreeReducer(s1, {
      type: 'addTab',
      paneId: pid,
      opts: { projectId: 'p2', projectPath: '/b' },
    });
    // 当前激活第二个，selectTab 切回第一个
    const s3 = terminalTreeReducer(s2, { type: 'selectTab', paneId: pid, tabId: firstTabId });
    expect(rootPanel(s3).activeTabId).toBe(firstTabId);
    expect(s3.activePaneId).toBe(pid);
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

  it('关闭根面板（无父）→ 重置为一个全新空面板（无 Tab）', () => {
    const s0 = createInitialTreeState();
    const orig = rootPanel(s0).id;
    const s1 = terminalTreeReducer(s0, { type: 'closePane', paneId: orig });
    expect(s1.rootPane.type).toBe('panel');
    expect(rootPanel(s1).id).not.toBe(orig);
    expect(rootPanel(s1).tabs).toHaveLength(0);
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
      opts: { projectId: 'p1', projectPath: '/a' },
    });
    const s2 = terminalTreeReducer(s1, {
      type: 'addTab',
      paneId: pid,
      opts: { projectId: 'p2', projectPath: '/b' },
    });
    const activeTabId = rootPanel(s2).activeTabId; // 第二个 Tab
    const s3 = terminalTreeReducer(s2, { type: 'closeTab', paneId: pid, tabId: activeTabId });
    expect(rootPanel(s3).tabs).toHaveLength(1);
    expect(rootPanel(s3).activeTabId).toBe(rootPanel(s3).tabs[0]!.id);
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
  it('回填 Tab 级与单 leaf 的 sessionId（按 tabId 全树定位）', () => {
    const base = createInitialTreeState();
    const pid = rootPanel(base).id;
    const s0 = terminalTreeReducer(base, {
      type: 'addTab',
      paneId: pid,
      opts: { projectId: 'p', projectPath: '/a' },
    });
    const tabId = rootPanel(s0).tabs[0]!.id;
    const s1 = terminalTreeReducer(s0, {
      type: 'setTabSession',
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

  it('tabId 不存在 → 原样返回', () => {
    const s0 = createInitialTreeState();
    const s1 = terminalTreeReducer(s0, { type: 'setTabSession', tabId: 'nope', sessionId: 'x' });
    expect(s1).toBe(s0);
  });
});

// ── P5 拖拽：reorderTabs / moveTab / splitAndMoveTab ──

describe('reorderTabs', () => {
  // 在单面板里放 3 个 Tab，返回 [面板 id, [tabId...]]。
  function threeTabs(): { state: TerminalTreeState; pid: string; ids: string[] } {
    let s = createInitialTreeState();
    const pid = rootPanel(s).id;
    for (const p of ['a', 'b', 'c']) {
      s = terminalTreeReducer(s, { type: 'addTab', paneId: pid, opts: { projectId: p, projectPath: `/${p}` } });
    }
    return { state: s, pid, ids: rootPanel(s).tabs.map((t) => t.id) };
  }

  it('把首个 Tab 移到末位 → 顺序变 [b,c,a]', () => {
    const { state, pid, ids } = threeTabs();
    const s1 = terminalTreeReducer(state, { type: 'reorderTabs', paneId: pid, fromIndex: 0, toIndex: 2 });
    expect(rootPanel(s1).tabs.map((t) => t.id)).toEqual([ids[1], ids[2], ids[0]]);
  });

  it('fromIndex === toIndex → 原样返回（同引用）', () => {
    const { state, pid } = threeTabs();
    const s1 = terminalTreeReducer(state, { type: 'reorderTabs', paneId: pid, fromIndex: 1, toIndex: 1 });
    expect(s1).toBe(state);
  });

  it('越界下标 → 原样返回', () => {
    const { state, pid } = threeTabs();
    expect(terminalTreeReducer(state, { type: 'reorderTabs', paneId: pid, fromIndex: 0, toIndex: 9 })).toBe(state);
    expect(terminalTreeReducer(state, { type: 'reorderTabs', paneId: pid, fromIndex: -1, toIndex: 0 })).toBe(state);
  });
});

describe('moveTab', () => {
  // 构造 hsplit[A, B]：A 含 2 Tab（a1/a2）；B 含 1 Tab（split 自动建的默认空 Tab）。
  function twoPanesWithTabs() {
    let s = createInitialTreeState();
    const a = rootPanel(s).id;
    s = terminalTreeReducer(s, { type: 'addTab', paneId: a, opts: { projectId: 'a1', projectPath: '/a1' } });
    s = terminalTreeReducer(s, { type: 'addTab', paneId: a, opts: { projectId: 'a2', projectPath: '/a2' } });
    s = terminalTreeReducer(s, { type: 'split', paneId: a, direction: 'right' });
    const b = s.activePaneId;
    const aPanel = panels(s).find((p) => p.id === a)!;
    const bPanel = panels(s).find((p) => p.id === b)!;
    return { state: s, a, b, aTabs: aPanel.tabs.map((t) => t.id), bTabs: bPanel.tabs.map((t) => t.id) };
  }

  it('跨面板移动：Tab 从 A 进 B 并激活，B 设为活动面板', () => {
    const { state, a, b, aTabs } = twoPanesWithTabs();
    const moved = aTabs[0]!;
    const s1 = terminalTreeReducer(state, { type: 'moveTab', fromPaneId: a, toPaneId: b, tabId: moved });
    const aPanel = panels(s1).find((p) => p.id === a)!;
    const bPanel = panels(s1).find((p) => p.id === b)!;
    expect(aPanel.tabs.map((t) => t.id)).not.toContain(moved);
    expect(bPanel.tabs.map((t) => t.id)).toContain(moved);
    expect(bPanel.activeTabId).toBe(moved);
    expect(s1.activePaneId).toBe(b);
    // 源面板活动 Tab 修正为剩余的
    expect(aPanel.activeTabId).toBe(aPanel.tabs[0]!.id);
  });

  it('toIndex 指定插入位置', () => {
    const { state, a, b, aTabs, bTabs } = twoPanesWithTabs();
    const moved = aTabs[1]!;
    const s1 = terminalTreeReducer(state, { type: 'moveTab', fromPaneId: a, toPaneId: b, tabId: moved, toIndex: 0 });
    const bPanel = panels(s1).find((p) => p.id === b)!;
    expect(bPanel.tabs.map((t) => t.id)).toEqual([moved, bTabs[0]]);
  });

  it('移走 B 唯一的 Tab → B 被掏空 → 折叠回单面板 A，活动面板=A', () => {
    const { state, a, b, bTabs } = twoPanesWithTabs();
    const moved = bTabs[0]!;
    const s1 = terminalTreeReducer(state, { type: 'moveTab', fromPaneId: b, toPaneId: a, tabId: moved });
    expect(s1.rootPane.type).toBe('panel'); // 折叠
    expect(rootPanel(s1).id).toBe(a);
    expect(s1.activePaneId).toBe(a);
    expect(rootPanel(s1).tabs.map((t) => t.id)).toContain(moved);
    expect(rootPanel(s1).activeTabId).toBe(moved);
    // B 已不存在
    expect(panels(s1).some((p) => p.id === b)).toBe(false);
  });

  it('源/目标面板不存在 → 原样返回', () => {
    const { state, a } = twoPanesWithTabs();
    expect(
      terminalTreeReducer(state, { type: 'moveTab', fromPaneId: a, toPaneId: 'nope', tabId: 'x' }),
    ).toBe(state);
  });
});

describe('splitAndMoveTab', () => {
  function paneWithTwoTabs() {
    let s = createInitialTreeState();
    const a = rootPanel(s).id;
    s = terminalTreeReducer(s, { type: 'addTab', paneId: a, opts: { projectId: 'a1', projectPath: '/a1' } });
    s = terminalTreeReducer(s, { type: 'addTab', paneId: a, opts: { projectId: 'a2', projectPath: '/a2' } });
    return { state: s, a, ids: rootPanel(s).tabs.map((t) => t.id) };
  }

  it('向右拆分并移 Tab → 根变水平 split[源, 新]，新面板含被移 Tab 且被激活', () => {
    const { state, a, ids } = paneWithTwoTabs();
    const moved = ids[1]!;
    const s1 = terminalTreeReducer(state, { type: 'splitAndMoveTab', paneId: a, tabId: moved, direction: 'right' });
    expect(s1.rootPane.type).toBe('split');
    const sp = rootSplit(s1);
    expect(sp.direction).toBe('horizontal');
    expect(sp.children).toHaveLength(2);
    const src = sp.children[0] as Panel;
    const dst = sp.children[1] as Panel;
    expect(src.id).toBe(a);
    expect(src.tabs.map((t) => t.id)).not.toContain(moved);
    expect(dst.tabs.map((t) => t.id)).toEqual([moved]);
    expect(dst.activeTabId).toBe(moved);
    expect(s1.activePaneId).toBe(dst.id);
  });

  it('面板只有 1 个 Tab → 拒绝（原样返回，绝不移出唯一 Tab）', () => {
    let s = createInitialTreeState();
    const a = rootPanel(s).id;
    s = terminalTreeReducer(s, { type: 'addTab', paneId: a, opts: { projectId: 'a1', projectPath: '/a1' } });
    const only = rootPanel(s).tabs[0]!.id;
    const s1 = terminalTreeReducer(s, { type: 'splitAndMoveTab', paneId: a, tabId: only, direction: 'right' });
    expect(s1).toBe(s);
  });
});

describe('setTabSession 跨 moveTab 后仍能回填（隐患回归）', () => {
  it('Tab 移到新面板后，按 tabId 回填落到当前 pane（不 no-op）', () => {
    // A 含 2 Tab，B 含 1 Tab；把 A 的一个 Tab 移到 B，再对其回填 sessionId。
    let s = createInitialTreeState();
    const a = rootPanel(s).id;
    s = terminalTreeReducer(s, { type: 'addTab', paneId: a, opts: { projectId: 'a1', projectPath: '/a1' } });
    s = terminalTreeReducer(s, { type: 'addTab', paneId: a, opts: { projectId: 'a2', projectPath: '/a2' } });
    s = terminalTreeReducer(s, { type: 'split', paneId: a, direction: 'right' });
    const b = s.activePaneId;
    s = terminalTreeReducer(s, { type: 'addTab', paneId: b, opts: { projectId: 'b1', projectPath: '/b1' } });
    const movedTab = panels(s).find((p) => p.id === a)!.tabs[0]!.id;
    s = terminalTreeReducer(s, { type: 'moveTab', fromPaneId: a, toPaneId: b, tabId: movedTab });

    const s2 = terminalTreeReducer(s, { type: 'setTabSession', tabId: movedTab, sessionId: 'sess-moved' });
    const bPanel = panels(s2).find((p) => p.id === b)!;
    const tab = bPanel.tabs.find((t) => t.id === movedTab)!;
    expect(tab.sessionId).toBe('sess-moved');
    if (tab.terminalRootPane?.type === 'leaf') {
      expect(tab.terminalRootPane.sessionId).toBe('sess-moved');
    }
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
