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
    // 视图切换按钮（终端 ↔ 对话卡片）：常驻右上角，zIndex 高于 SessionTimeline 覆盖层(6)，
    // 两种模式下同位置同按钮（图标随当前态切换），双向可达且不受底部地址栏/键盘折叠影响。
    viewToggle: {
      position: 'absolute',
      top: 6,
      right: 8,
      zIndex: 8,
      borderRadius: 1.5, // 6px：方圆设计的按钮圆角
      backgroundColor: fp.bg.elevated,
      border: `1px solid ${fp.divider}`,
      color: fp.text.secondary,
      boxShadow: 2,
      '&:hover': { backgroundColor: fp.bg.hover },
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
