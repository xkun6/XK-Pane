// DndPaneProvider 的 ViewModel：把 @dnd-kit 拖拽事件翻译成 TerminalContext 的 reorder/move 操作。
// 忠实移植 cc-pane `DndPaneProvider` 的 onDragEnd 决策（同面板→reorderTabs，跨面板→moveTab）。
// 仅消费 useTerminal()（Context，非 bindings）+ paneTree 纯函数；不返回 JSX。
import { useCallback, useState } from 'react';
import {
  closestCenter,
  pointerWithin,
  PointerSensor,
  useSensor,
  useSensors,
  type CollisionDetection,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core';
import { useTerminal, type SplitDirection, type Tab } from '@/terminal';
import { collectPanels } from '@/terminal/paneTree';

/** 拖拽源携带的数据（active.data.current）/ TabBar 内的可放置目标（over）。 */
export interface DndTabData {
  type: 'tab';
  paneId: string;
  tab: Tab;
}

/** 面板内容边缘的可放置目标（over.data.current）—— 落此触发拆分并把 Tab 移入新面板。 */
export interface DndEdgeData {
  type: 'paneEdge';
  paneId: string;
  direction: SplitDirection;
}

export interface DndPaneModel {
  sensors: ReturnType<typeof useSensors>;
  /** 当前被拖拽的 Tab（用于 DragOverlay 浮层预览）；未拖拽时 null。 */
  activeTab: Tab | null;
  handleDragStart: (event: DragStartEvent) => void;
  handleDragEnd: (event: DragEndEvent) => void;
  handleDragCancel: () => void;
}

function asTabData(data: unknown): DndTabData | null {
  const d = data as DndTabData | undefined;
  return d && d.type === 'tab' ? d : null;
}

/**
 * 面板树拖拽碰撞检测：
 *  ① 仅当指针精确落在「源 Tab **自己面板**的」边缘区时，采纳边缘分屏（pointerWithin）。
 *  ② 其余一律在 Tab 容器中按 closestCenter 取最近 Tab —— 宽容好命中，支撑同面板重排 +
 *     跨面板合并（拖到目标面板任意位置都并入其最近的 Tab）。
 * 不用全局 pointerWithin：它要求指针精确落在目标 Tab rect 内，跨面板合并极难命中（曾致「合并不了」）。
 * 跨面板的边缘区（paneId≠源）不视作边缘命中，自然落到 ②→合并，而非静默无反应。
 */
export const paneCollisionDetection: CollisionDetection = (args) => {
  const sourcePaneId = (args.active.data.current as DndTabData | undefined)?.paneId;
  const ownEdgeHits = pointerWithin(args).filter((collision) => {
    const container = args.droppableContainers.find((dc) => dc.id === collision.id);
    const data = container?.data.current as DndEdgeData | undefined;
    return data?.type === 'paneEdge' && data.paneId === sourcePaneId;
  });
  if (ownEdgeHits.length > 0) return ownEdgeHits;

  const tabContainers = args.droppableContainers.filter(
    (dc) => (dc.data.current as DndTabData | undefined)?.type === 'tab',
  );
  return closestCenter({ ...args, droppableContainers: tabContainers });
};

export function useDndPaneProvider(): DndPaneModel {
  const { rootPane, reorderTabs, moveTab, splitAndMoveTab } = useTerminal();
  const [activeTab, setActiveTab] = useState<Tab | null>(null);

  // 至少拖动 8px 才启动拖拽，避免与「点击选中 / 点击关闭」误触。
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 8 } }));

  const handleDragStart = useCallback((event: DragStartEvent) => {
    const data = asTabData(event.active.data.current);
    if (data) setActiveTab(data.tab);
  }, []);

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      setActiveTab(null);
      const { active, over } = event;
      if (!over) return;
      const activeData = asTabData(active.data.current);
      if (!activeData) return;

      const fromPaneId = activeData.paneId;
      const tabId = String(active.id);
      const overRaw = over.data.current as DndTabData | DndEdgeData | undefined;

      if (overRaw?.type === 'paneEdge') {
        // 边缘分屏：仅当拖的是「自己面板的 Tab」落到「自己面板的边缘」才拆分（跨面板移动请落目标 TabBar）。
        // splitAndMoveTab 内部要求源面板 >1 个 Tab，否则 no-op（绝不把唯一 Tab 移出）。
        if (overRaw.paneId === fromPaneId) {
          splitAndMoveTab(fromPaneId, tabId, overRaw.direction);
        }
        return;
      }

      if (overRaw?.type !== 'tab') return;
      const toPaneId = overRaw.paneId;
      const overTabId = String(over.id);
      const panels = collectPanels(rootPane);

      if (fromPaneId === toPaneId) {
        // 同面板内重排。
        const panel = panels.find((p) => p.id === fromPaneId);
        if (!panel) return;
        const fromIndex = panel.tabs.findIndex((t) => t.id === tabId);
        const toIndex = panel.tabs.findIndex((t) => t.id === overTabId);
        if (fromIndex !== -1 && toIndex !== -1 && fromIndex !== toIndex) {
          reorderTabs(fromPaneId, fromIndex, toIndex);
        }
      } else {
        // 跨面板移动，插到目标 Tab 的位置。
        const toPanel = panels.find((p) => p.id === toPaneId);
        if (!toPanel) return;
        const toIndex = toPanel.tabs.findIndex((t) => t.id === overTabId);
        moveTab(fromPaneId, toPaneId, tabId, toIndex >= 0 ? toIndex : undefined);
      }
    },
    [rootPane, reorderTabs, moveTab, splitAndMoveTab],
  );

  const handleDragCancel = useCallback(() => setActiveTab(null), []);

  return { sensors, activeTab, handleDragStart, handleDragEnd, handleDragCancel };
}
