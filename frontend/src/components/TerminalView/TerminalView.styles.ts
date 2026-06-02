import type { SxProps, Theme } from '@mui/material';

export const terminalViewStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  return {
    // 桌面（owner）外壳：相对定位锚点，host 跑 xterm，SessionTimeline 作绝对覆盖层叠其上（卡片态）。
    ownerRoot: {
      position: 'relative',
      width: '100%',
      height: '100%',
      minHeight: 0,
      overflow: 'hidden',
      backgroundColor: fp.bg.content,
    },
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
    // viewer（Web/移动）外壳：竖向 = 终端占满 + 底部虚拟键栏（P7 M6）。host 改 flex:1 取剩余高，
    // fit addon 的 ResizeObserver 据缩小后的 host 重算列/行。
    viewerRoot: {
      position: 'relative', // 作为 SessionTimeline 覆盖层 / 对话 FAB 的定位锚点
      display: 'flex',
      flexDirection: 'column',
      width: '100%',
      height: '100%',
      minHeight: 0,
    },
    viewerHost: {
      position: 'relative',
      flex: 1,
      minHeight: 0,
      overflow: 'hidden',
      backgroundColor: fp.bg.content,
      px: 1,
      py: 0.5,
      '& .xterm': { height: '100%' },
      '& .xterm-viewport': { backgroundColor: 'transparent !important' },
    },
  };
};
