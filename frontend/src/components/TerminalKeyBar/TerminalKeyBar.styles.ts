import type { Theme } from '@mui/material';

// 虚拟键栏样式：终端底部一条横向可滚动的小键条。配色取 foundation 槽位，圆角按钮 6（方圆）。
export const terminalKeyBarStyles = (theme: Theme) => ({
  bar: {
    flexShrink: 0,
    display: 'flex',
    alignItems: 'center',
    gap: 0.5,
    px: 0.75,
    py: 0.5,
    overflowX: 'auto' as const,
    borderTop: `1px solid ${theme.palette.foundation.divider}`,
    backgroundColor: theme.palette.foundation.bg.surface,
    // 触摸滚动顺滑，隐藏滚动条视觉噪声。
    scrollbarWidth: 'none' as const,
    '&::-webkit-scrollbar': { display: 'none' },
  },
  key: {
    flexShrink: 0,
    minWidth: 40,
    height: 34,
    px: 1,
    borderRadius: 1.5, // 6px 按钮
    border: `1px solid ${theme.palette.foundation.divider}`,
    backgroundColor: theme.palette.foundation.bg.elevated,
    color: theme.palette.foundation.text.primary,
    fontSize: 13,
    lineHeight: 1,
    textTransform: 'none' as const,
    '&:hover': { backgroundColor: theme.palette.foundation.bg.hover },
    '&.Mui-disabled': { opacity: 0.4 },
  },
});
