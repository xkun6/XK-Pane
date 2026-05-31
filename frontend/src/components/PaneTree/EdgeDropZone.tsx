import { Box, useTheme } from '@mui/material';
import { useDroppable } from '@dnd-kit/core';
import type { PaneEdge } from '@/terminal';
import { edgeDropStyles } from './PaneTree.styles';
import type { DndEdgeData, DndTabData } from './useDndPaneProvider';

// 面板内容区的单条边放置区：把任意 Tab 落到此处 → DndPaneProvider 在该面板旁拆出新面板装它。
// 高亮只在「这次拖拽真的会触发拆分」时显示，避免给出无效承诺：
//   - 跨面板（拖的是别的面板的 Tab）：总会拆分（移入新面板，源掏空则折叠）。
//   - 同面板：仅当本面板有 >1 个 Tab（否则移出唯一 Tab = 原地分屏，无意义，moveTabToEdge 会 no-op）。
export const EdgeDropZone = ({
  paneId,
  edge,
  paneTabCount,
}: {
  paneId: string;
  edge: PaneEdge;
  paneTabCount: number;
}) => {
  const theme = useTheme();
  const styles = edgeDropStyles(theme);
  const data: DndEdgeData = { type: 'paneEdge', paneId, edge };
  const { setNodeRef, isOver, active } = useDroppable({ id: `edge:${paneId}:${edge}`, data });

  const sourcePaneId = (active?.data.current as DndTabData | undefined)?.paneId;
  const wouldSplit = sourcePaneId != null && (sourcePaneId !== paneId || paneTabCount > 1);
  const showActive = isOver && wouldSplit;

  return <Box ref={setNodeRef} sx={[styles[edge], showActive && styles.zoneActive]} />;
};
