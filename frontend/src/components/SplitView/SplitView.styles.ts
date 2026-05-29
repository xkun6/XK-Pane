import type { CSSProperties } from 'react';
import type { SxProps, Theme } from '@mui/material';

// 容器 / pane 为纯布局（无主题色）→ 用内联 CSSProperties；sash 需主题色 hover → 用 sx。
export const splitContainerStyle = (vertical: boolean): CSSProperties => ({
  display: 'flex',
  flexDirection: vertical ? 'column' : 'row',
  height: '100%',
  width: '100%',
  minWidth: 0,
  minHeight: 0,
});

export const splitPaneStyle = (basisPercent: number): CSSProperties => ({
  flexBasis: `${basisPercent}%`,
  flexGrow: 0,
  flexShrink: 0,
  overflow: 'hidden',
  minWidth: 0,
  minHeight: 0,
});

export const splitSashSx = (theme: Theme, vertical: boolean): SxProps<Theme> => {
  const fp = theme.palette.foundation;
  return {
    flex: '0 0 auto',
    position: 'relative',
    zIndex: 1,
    backgroundColor: 'transparent',
    transition: 'background-color 120ms ease',
    ...(vertical
      ? { height: '4px', width: '100%', cursor: 'row-resize' }
      : { width: '4px', height: '100%', cursor: 'col-resize' }),
    '&:hover': { backgroundColor: fp.accent },
  };
};
