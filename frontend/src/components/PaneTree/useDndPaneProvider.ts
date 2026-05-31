// DndPaneProvider 的 ViewModel：把 @dnd-kit 拖拽事件翻译成 TerminalContext 的 reorder/move 操作。
// 忠实移植 cc-pane `DndPaneProvider` 的 onDragEnd 决策（同面板→reorderTabs，跨面板→moveTab）。
// 仅消费 useTerminal()（Context，非 bindings）+ paneTree 纯函数；不返回 JSX。
import { useCallback, useState } from 'react';
import {
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core';
import { useTerminal, type Tab } from '@/terminal';
import { collectPanels } from '@/terminal/paneTree';

/** 拖拽源/目标携带的数据（active/over.data.current）。 */
export interface DndTabData {
  type: 'tab';
  paneId: string;
  tab: Tab;
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

export function useDndPaneProvider(): DndPaneModel {
  const { rootPane, reorderTabs, moveTab } = useTerminal();
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
      const overData = asTabData(over.data.current);
      if (!overData) return; // P5b-1 仅处理「落到另一 Tab 上」；边缘分屏落 P5b-2。

      const fromPaneId = activeData.paneId;
      const toPaneId = overData.paneId;
      const tabId = String(active.id);
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
    [rootPane, reorderTabs, moveTab],
  );

  const handleDragCancel = useCallback(() => setActiveTab(null), []);

  return { sensors, activeTab, handleDragStart, handleDragEnd, handleDragCancel };
}
