import { Box, useTheme } from '@mui/material';
import { DndContext, DragOverlay } from '@dnd-kit/core';
import type { ReactNode } from 'react';
import { paneCollisionDetection, useDndPaneProvider } from './useDndPaneProvider';
import { dragOverlayStyles } from './PaneTree.styles';

// 面板树拖拽上下文：包在整棵 PaneTree 外层，让 Tab 可在面板间拖拽（reorder / 跨面板 move）。
// 等价 cc-pane DndPaneProvider（D2：消费 TerminalContext 而非 Zustand store）。
export const DndPaneProvider = ({ children }: { children: ReactNode }) => {
  const theme = useTheme();
  const styles = dragOverlayStyles(theme);
  const { sensors, activeTab, handleDragStart, handleDragEnd, handleDragCancel } =
    useDndPaneProvider();

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={paneCollisionDetection}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
      onDragCancel={handleDragCancel}
    >
      {children}
      <DragOverlay dropAnimation={null}>
        {activeTab ? <Box sx={styles.overlay}>{activeTab.title}</Box> : null}
      </DragOverlay>
    </DndContext>
  );
};
