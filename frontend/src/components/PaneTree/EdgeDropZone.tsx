import { Box, useTheme } from '@mui/material';
import { useDroppable } from '@dnd-kit/core';
import type { SplitDirection } from '@/terminal';
import { edgeDropStyles } from './PaneTree.styles';
import type { DndEdgeData } from './useDndPaneProvider';

// 面板内容区的单个边缘放置区：拖自己面板的 Tab 落到此处 → DndPaneProvider 触发 splitAndMoveTab。
// data 形如 DndEdgeData；isOver 时显示半透明高亮预览拆分位置。
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
  const { setNodeRef, isOver } = useDroppable({ id: `edge:${paneId}:${direction}`, data });

  return (
    <Box
      ref={setNodeRef}
      sx={[direction === 'right' ? styles.zoneRight : styles.zoneBottom, isOver && styles.zoneActive]}
    />
  );
};
