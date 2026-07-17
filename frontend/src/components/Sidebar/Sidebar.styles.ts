import type { SxProps, Theme } from '@mui/material';

// Sidebar 现代扁平化：
//  - 未选中：透明底 + secondary 图标
//  - hover：soft hover 底 + primary 图标
//  - 选中：accent 轻底 + accent 图标 + 左侧胶囊 indicator
//  - 品牌块：accent 轻 tint + 柔和方圆
//  - 颜色全部从 theme.palette.foundation.* 取
export const sidebarStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  const isDark = theme.palette.mode === 'dark';
  return {
    root: {
      width: 64,
      flexShrink: 0,
      backgroundColor: fp.bg.sidebar,
      borderRight: `1px solid ${fp.divider}`,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      py: 1.5,
      gap: 0.75,
      // 与主内容轻微分层（亮色更明显）
      boxShadow: isDark ? 'none' : '1px 0 0 transparent, 4px 0 24px rgba(15, 23, 42, 0.03)',
    },
    brand: {
      width: 40,
      height: 40,
      borderRadius: '11px',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      backgroundColor: fp.bg.active,
      color: fp.accent,
      fontSize: 15,
      fontWeight: 700,
      letterSpacing: 0.3,
      mb: 0.25,
      border: `1px solid ${isDark ? 'rgba(255,255,255,0.06)' : 'rgba(59, 130, 246, 0.14)'}`,
      boxShadow: isDark
        ? 'none'
        : '0 1px 2px rgba(15, 23, 42, 0.04)',
    },
    brandDivider: {
      width: 28,
      height: '1px',
      backgroundColor: fp.divider,
      mb: 0.75,
      mt: 0.25,
    },
    section: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: 0.5,
      py: 0.25,
    },
    spacer: {
      flex: 1,
    },
    itemWrap: {
      position: 'relative',
      width: 48,
      height: 44,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    indicator: {
      position: 'absolute',
      left: -8,
      top: '50%',
      transform: 'translateY(-50%)',
      width: 3,
      height: 18,
      borderRadius: 999,
      backgroundColor: fp.accent,
      boxShadow: isDark ? `0 0 8px ${fp.accent}` : 'none',
    },
    item: {
      width: 44,
      height: 44,
      borderRadius: '12px',
      color: fp.text.secondary,
      backgroundColor: 'transparent',
      transition:
        'background-color 160ms cubic-bezier(0.4, 0, 0.2, 1), color 160ms cubic-bezier(0.4, 0, 0.2, 1), transform 100ms ease',
      '&:hover': {
        backgroundColor: fp.bg.hover,
        color: fp.text.primary,
      },
      '&:active': {
        transform: 'scale(0.96)',
      },
    },
    itemActive: {
      width: 44,
      height: 44,
      borderRadius: '12px',
      color: fp.accent,
      backgroundColor: fp.bg.active,
      transition:
        'background-color 160ms cubic-bezier(0.4, 0, 0.2, 1), color 160ms cubic-bezier(0.4, 0, 0.2, 1)',
      '&:hover': {
        backgroundColor: fp.bg.active,
        color: fp.accent,
      },
    },
  };
};
