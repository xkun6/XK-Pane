import type { SxProps, Theme } from '@mui/material';

// 设置页两列布局（最外侧的 Sidebar 菜单按钮不算）：
//  ┌─────────────┬─────────────────────────────┐
//  │  选项列表    │        详细配置             │
//  │ (232px 固定) │       (flex 占满)           │
//  └─────────────┴─────────────────────────────┘
//
// 这里只放"框架样式"：
//   - root / list / detail 容器
//   - 共享的 section 卡片标题（子页面内部分组用 sectionTitle / sectionHint / section）
//
// 子页面专属样式请在 SettingsPage/<name>/<Name>.styles.ts 里维护。
export const settingsPageStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  const isDark = theme.palette.mode === 'dark';
  return {
    root: {
      flex: 1,
      minHeight: 0,
      display: 'flex',
      flexDirection: 'row',
      backgroundColor: fp.bg.content,
      overflow: 'hidden',
    },

    // —— 左列：选项列表 ——
    list: {
      width: 232,
      flexShrink: 0,
      borderRight: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.sidebar,
      display: 'flex',
      flexDirection: 'column',
      overflowY: 'auto',
    },
    listHeader: {
      px: 2.5,
      pt: 3,
      pb: 1.5,
    },
    listEyebrow: {
      fontSize: 11,
      fontWeight: 600,
      letterSpacing: 1.2,
      textTransform: 'uppercase',
      color: fp.text.muted,
    },
    listTitle: {
      fontSize: 20,
      fontWeight: 700,
      color: fp.text.primary,
      mt: 0.5,
      letterSpacing: '-0.02em',
    },
    listItems: {
      px: 1.25,
      py: 0.75,
      display: 'flex',
      flexDirection: 'column',
      gap: 0.35,
    },
    listItem: {
      position: 'relative',
      width: '100%',
      px: 1.75,
      py: 1.15,
      borderRadius: '10px',
      cursor: 'pointer',
      textAlign: 'left',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'flex-start',
      justifyContent: 'flex-start',
      gap: 0.2,
      color: fp.text.secondary,
      backgroundColor: 'transparent',
      transition:
        'background-color 160ms cubic-bezier(0.4, 0, 0.2, 1), color 160ms cubic-bezier(0.4, 0, 0.2, 1)',
      '&:hover': {
        backgroundColor: fp.bg.hover,
        color: fp.text.primary,
      },
    },
    listItemActive: {
      position: 'relative',
      width: '100%',
      px: 1.75,
      py: 1.15,
      borderRadius: '10px',
      cursor: 'pointer',
      textAlign: 'left',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'flex-start',
      justifyContent: 'flex-start',
      gap: 0.2,
      color: fp.accent,
      backgroundColor: fp.bg.active,
      transition:
        'background-color 160ms cubic-bezier(0.4, 0, 0.2, 1), color 160ms cubic-bezier(0.4, 0, 0.2, 1)',
      '&:hover': {
        backgroundColor: fp.bg.active,
        color: fp.accent,
      },
    },
    listItemIndicator: {
      position: 'absolute',
      left: 0,
      top: '50%',
      transform: 'translateY(-50%)',
      width: 3,
      height: 16,
      borderRadius: 999,
      backgroundColor: fp.accent,
      boxShadow: isDark ? `0 0 6px ${fp.accent}` : 'none',
    },
    listItemLabel: {
      fontSize: 13.5,
      fontWeight: 600,
      color: 'inherit',
      letterSpacing: '-0.01em',
    },
    listItemDesc: {
      fontSize: 11.5,
      color: fp.text.muted,
      lineHeight: 1.4,
    },

    // —— 右列：详细配置 ——
    detail: {
      flex: 1,
      minWidth: 0,
      overflowY: 'auto',
      px: { xs: 3, sm: 5, md: 6 },
      py: { xs: 3.5, sm: 5 },
      display: 'flex',
      flexDirection: 'column',
      gap: 4,
      maxWidth: 880,
    },
    detailHeader: {
      display: 'flex',
      flexDirection: 'column',
      gap: 0.75,
      pb: 0.5,
    },
    detailEyebrow: {
      fontSize: 11,
      fontWeight: 600,
      letterSpacing: 1.2,
      textTransform: 'uppercase',
      color: fp.text.muted,
    },
    detailTitle: {
      fontSize: 26,
      fontWeight: 700,
      color: fp.text.primary,
      lineHeight: 1.15,
      letterSpacing: '-0.025em',
      m: 0,
    },
    detailDesc: {
      fontSize: 14,
      color: fp.text.secondary,
      lineHeight: 1.55,
      maxWidth: 560,
    },

    // —— 子页面共享：section 容器 + 标题 ——
    section: {
      display: 'flex',
      flexDirection: 'column',
      gap: 1.5,
    },
    sectionHeader: {
      display: 'flex',
      flexDirection: 'column',
      gap: 0.25,
    },
    sectionTitle: {
      fontSize: 14.5,
      fontWeight: 600,
      color: fp.text.primary,
      letterSpacing: '-0.01em',
    },
    sectionHint: {
      fontSize: 12.5,
      color: fp.text.muted,
      lineHeight: 1.55,
    },
  };
};
