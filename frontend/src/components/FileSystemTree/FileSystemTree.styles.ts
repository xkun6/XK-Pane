import type { SxProps, Theme } from '@mui/material';

export const fileSystemTreeStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  return {
    root: {
      flex: 1,
      minHeight: 0,
      overflowY: 'auto',
      overflowX: 'hidden',
      py: 0.5,
    },
    row: {
      display: 'flex',
      alignItems: 'center',
      gap: 0.5,
      height: 26,
      pr: 1,
      borderRadius: 6,
      cursor: 'pointer',
      userSelect: 'none',
      color: fp.text.primary,
      '&:hover': { backgroundColor: fp.bg.hover },
    },
    rowActive: {
      display: 'flex',
      alignItems: 'center',
      gap: 0.5,
      height: 26,
      pr: 1,
      borderRadius: 6,
      cursor: 'pointer',
      userSelect: 'none',
      color: fp.text.primary,
      backgroundColor: fp.bg.active,
    },
    // 拖拽悬停到目录上的放置高亮（叠加在 row/rowActive 之上）。
    rowDropOverlay: {
      backgroundColor: fp.bg.active,
      outline: `1px solid ${fp.accent}`,
      outlineOffset: '-1px',
    },
    // 内联新建/重命名输入框（替代模态对话框）。
    inlineInput: {
      flex: 1,
      minWidth: 0,
      fontSize: 13,
      color: fp.text.primary,
      lineHeight: 1.4,
      px: 0.5,
      borderRadius: 4,
      backgroundColor: fp.bg.elevated,
      border: `1px solid ${fp.accent}`,
      '& input': { p: 0, height: 20 },
    },
    // 校验失败时输入框红框。
    inlineInputError: {
      border: `1px solid ${fp.status.danger}`,
    },
    chevron: {
      display: 'flex',
      flexShrink: 0,
      color: fp.text.muted,
      width: 18,
    },
    chevronSpacer: {
      flexShrink: 0,
      width: 18,
    },
    icon: {
      display: 'flex',
      flexShrink: 0,
      color: fp.text.secondary,
    },
    iconDir: {
      display: 'flex',
      flexShrink: 0,
      color: fp.accent,
    },
    name: {
      flex: 1,
      minWidth: 0,
      fontSize: 13,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    // 模糊过滤命中字符高亮
    matchHL: {
      color: fp.accent,
      fontWeight: 700,
    },
    hint: {
      px: 1.5,
      py: 1,
      fontSize: 12,
      color: fp.text.muted,
    },
    errorHint: {
      px: 1.5,
      py: 1,
      fontSize: 12,
      color: fp.status.danger,
    },
    // —— 右键上下文菜单 ——
    menuPaper: {
      borderRadius: 8,
      backgroundColor: fp.bg.elevated,
      border: `1px solid ${fp.divider}`,
      minWidth: 184,
      boxShadow: theme.shadows[4],
    },
    menuItem: {
      fontSize: 13,
      py: 0.5,
      color: fp.text.primary,
      '&:hover': { backgroundColor: fp.bg.hover },
    },
    menuItemDanger: {
      fontSize: 13,
      py: 0.5,
      color: fp.status.danger,
      '&:hover': { backgroundColor: fp.bg.hover },
    },
    menuIcon: {
      minWidth: 30,
      color: fp.text.secondary,
    },
    menuIconDanger: {
      minWidth: 30,
      color: fp.status.danger,
    },
    menuLabel: {
      fontSize: 13,
    },
    menuDivider: {
      borderColor: fp.divider,
      my: 0.5,
    },
  };
};
