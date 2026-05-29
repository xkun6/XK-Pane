import type { SxProps, Theme } from '@mui/material';

export const terminalViewStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  return {
    host: {
      position: 'relative',
      width: '100%',
      height: '100%',
      minHeight: 0,
      overflow: 'hidden',
      backgroundColor: fp.bg.content,
      // 视口内边距：给字符留呼吸位（fit addon 据容器内尺寸算列/行）。
      px: 1,
      py: 0.5,
      '& .xterm': { height: '100%' },
      '& .xterm-viewport': { backgroundColor: 'transparent !important' },
    },
  };
};
