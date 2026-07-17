import type { SxProps, Theme } from '@mui/material';

// TitleBar：轻量标题栏，不抢主内容焦点。
// Windows Fluent caption buttons 保持 46×32、0 圆角。
export const titleBarStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  return {
    root: {
      height: 36,
      minHeight: 36,
      display: 'flex',
      alignItems: 'center',
      px: 1.5,
      backgroundColor: fp.bg.base,
      borderBottom: `1px solid ${fp.divider}`,
      flexShrink: 0,
      color: fp.text.muted,
      // 轻微底部分层，让标题栏与内容过渡更自然
      backdropFilter: 'saturate(1.1)',
    },
    macSpacer: {
      width: 72,
      flexShrink: 0,
    },
    title: {
      fontSize: 12.5,
      fontWeight: 500,
      letterSpacing: 0.15,
      color: fp.text.secondary,
      opacity: 0.9,
    },
    spacer: {
      flex: 1,
    },
    controls: {
      display: 'flex',
      alignItems: 'center',
      gap: 0,
      pl: 1,
    },
    // Windows Fluent caption buttons：46×32、0 圆角、透明
    controlBtn: {
      width: 46,
      height: 32,
      borderRadius: 0,
      fontSize: 14,
      backgroundColor: 'transparent',
      boxShadow: 'none',
      color: fp.text.secondary,
      transition: 'background-color 120ms ease, color 120ms ease',
      '&:hover': {
        backgroundColor: fp.bg.hover,
        color: fp.text.primary,
      },
    },
    closeBtn: {
      width: 46,
      height: 32,
      borderRadius: 0,
      fontSize: 14,
      backgroundColor: 'transparent',
      boxShadow: 'none',
      color: fp.text.secondary,
      transition: 'background-color 120ms ease, color 120ms ease',
      '&:hover': {
        backgroundColor: fp.status.danger,
        color: '#fff',
      },
    },
  };
};
