import { Box, useTheme } from '@mui/material';
import { useDroppable } from '@dnd-kit/core';
import type { SplitDirection } from '@/terminal';
import { edgeDropStyles } from './PaneTree.styles';
import type { DndEdgeData, DndTabData } from './useDndPaneProvider';

// 面板内容区的单个边缘放置区：拖自己面板的 Tab 落到此处 → DndPaneProvider 触发 splitAndMoveTab。
// 边缘分屏仅作用于「源 Tab 所在的面板」（splitAndMoveTab 拆的是 Tab 自己的 pane；跨面板移动请落目标 TabBar）。
// 因此高亮只在「当前拖拽的 Tab 来自本面板」时显示，避免对跨面板拖拽给出「可放置」的误导承诺却静默无反应。
export const EdgeDropZone = ({
  paneId,
  direction,
}: {
  paneId: string;
  direction: SplitDirection;
}) => {
  const theme = useTheme();
  const styles = edgeDropStyles(theme);
  const data: DndEdgeData = { type: 'paneEdge', paneId, direction };
  const { setNodeRef, isOver, active } = useDroppable({ id: `edge:${paneId}:${direction}`, data });

  const fromSamePane = (active?.data.current as DndTabData | undefined)?.paneId === paneId;
  const showActive = isOver && fromSamePane;

  return (
    <Box
      ref={setNodeRef}
      sx={[direction === 'right' ? styles.zoneRight : styles.zoneBottom, showActive && styles.zoneActive]}
    />
  );
};
