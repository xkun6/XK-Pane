import type { SxProps, Theme } from '@mui/material';

// 右侧抽屉（Git / 本地历史）：固定宽度、左描边、整列纵向；头部一行 + 主体填满并各自滚动。
// 宽度取 520 —— 给本地历史的「时间线 260 + 预览」两栏留出舒适空间。
export const rightPanelDrawerStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  return {
    root: {
      width: 520,
      flexShrink: 0,
      minHeight: 0,
      display: 'flex',
      flexDirection: 'column',
      backgroundColor: fp.bg.content,
      borderLeft: `1px solid ${fp.divider}`,
    },
    header: {
      display: 'flex',
      alignItems: 'center',
      gap: 1,
      flexShrink: 0,
      height: 44,
      padding: '0 6px 0 14px',
      borderBottom: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
    },
    headerIcon: {
      color: fp.accent,
      flexShrink: 0,
    },
    title: {
      fontSize: 13,
      fontWeight: 600,
      textTransform: 'uppercase',
      letterSpacing: 1,
      color: fp.text.primary,
      flexShrink: 0,
    },
    path: {
      flex: 1,
      minWidth: 0,
      fontSize: 12,
      fontFamily: 'monospace',
      color: fp.text.secondary,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      textAlign: 'right',
    },
    body: {
      flex: 1,
      minHeight: 0,
      minWidth: 0,
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
    },
    loading: {
      flex: 1,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: fp.text.secondary,
      fontSize: 13,
    },
  };
};
