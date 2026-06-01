import type { Theme } from '@mui/material';

// WebWorkspaceTree（Web viewer 左栏：工作区→项目→会话三层树）样式。
// 配色取 foundation 槽位；圆角遵循方圆设计（行/容器 8、按钮 6）。移动端无 hover，故行尾 ⋮
// 常驻可见（淡显），并保证行与 ⋮ 触摸目标 ≥36px。
export const webWorkspaceTreeStyles = (theme: Theme) => ({
  rail: {
    width: 272,
    flexShrink: 0,
    height: '100%',
    display: 'flex',
    flexDirection: 'column',
    borderRight: `1px solid ${theme.palette.foundation.divider}`,
    backgroundColor: theme.palette.foundation.bg.surface,
    boxSizing: 'border-box',
  },
  header: {
    px: 2,
    pt: 2,
    pb: 1,
    display: 'flex',
    flexDirection: 'column',
    gap: 0.5,
  },
  railTitle: {
    fontSize: 13,
    fontWeight: 600,
    color: theme.palette.foundation.text.secondary,
    textTransform: 'uppercase' as const,
    letterSpacing: 0.5,
  },
  hint: {
    fontSize: 12,
    lineHeight: 1.5,
    color: theme.palette.foundation.text.secondary,
  },
  list: {
    flex: 1,
    overflowY: 'auto',
    px: 1,
    pb: 2,
  },
  sectionHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    px: 1,
    pt: 0.5,
    pb: 0.25,
  },
  sectionDivider: {
    my: 1,
    borderColor: theme.palette.foundation.divider,
  },
  workspacesHeader: {
    px: 1,
    pt: 0.5,
    pb: 0.25,
  },
  subHintFlush: {
    px: 1,
    py: 0.75,
    fontSize: 12,
    color: theme.palette.foundation.text.secondary,
  },
  centerBox: {
    display: 'flex',
    justifyContent: 'center',
    py: 3,
  },
  emptyText: {
    px: 2,
    py: 3,
    fontSize: 13,
    color: theme.palette.foundation.text.secondary,
    textAlign: 'center' as const,
  },
  errorText: {
    px: 2,
    py: 3,
    fontSize: 13,
    color: theme.palette.foundation.status.danger,
    textAlign: 'center' as const,
  },

  // ── 行（workspace / project / session 共用骨架）──
  row: {
    display: 'flex',
    alignItems: 'center',
    gap: 0.75,
    minHeight: 38,
    pr: 0.5,
    borderRadius: 2, // 8px
    cursor: 'pointer',
    userSelect: 'none' as const,
    color: theme.palette.foundation.text.primary,
    '&:hover': { backgroundColor: theme.palette.foundation.bg.elevated },
    '&:focus-visible': {
      outline: `2px solid ${theme.palette.foundation.accent}`,
      outlineOffset: -2,
    },
  },
  wsRow: { pl: 1 },
  projectRow: { pl: 2.5 },
  sessionRow: { pl: 4 },

  chevron: {
    fontSize: 18,
    flexShrink: 0,
    color: theme.palette.foundation.text.secondary,
    transition: 'transform 120ms ease',
  },
  chevronOpen: { transform: 'rotate(90deg)' },
  rowGlyph: {
    fontSize: 16,
    flexShrink: 0,
    color: theme.palette.foundation.text.secondary,
  },
  rowLabel: {
    flex: 1,
    minWidth: 0,
    fontSize: 13,
    whiteSpace: 'nowrap' as const,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  wsLabel: { fontWeight: 600 },
  countPill: {
    flexShrink: 0,
    fontSize: 11,
    lineHeight: 1,
    px: 0.75,
    py: 0.25,
    borderRadius: 1.5,
    color: theme.palette.foundation.text.secondary,
    backgroundColor: theme.palette.foundation.bg.elevated,
  },
  // 行尾 ⋮ 按钮：移动端无 hover，故常驻淡显，hover/focus 时加深。
  moreBtn: {
    flexShrink: 0,
    p: 0.5,
    opacity: 0.6,
    color: theme.palette.foundation.text.secondary,
    '&:hover, &:focus-visible': {
      opacity: 1,
      backgroundColor: theme.palette.foundation.bg.hover,
    },
  },

  // ── 会话子项 ──
  sessionGlyph: {
    fontSize: 14,
    flexShrink: 0,
    color: theme.palette.foundation.text.secondary,
  },
  sessionDesc: {
    flex: 1,
    minWidth: 0,
    fontSize: 12.5,
    whiteSpace: 'nowrap' as const,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    color: theme.palette.foundation.text.primary,
  },
  sessionTime: {
    flexShrink: 0,
    fontSize: 11,
    color: theme.palette.foundation.text.secondary,
  },
  subHint: {
    pl: 4,
    py: 0.75,
    fontSize: 12,
    color: theme.palette.foundation.text.secondary,
  },

  // ── 新建会话对话框 ──
  dialogField: { mt: 1.5 },
  dialogTarget: {
    fontSize: 12.5,
    color: theme.palette.foundation.text.secondary,
    wordBreak: 'break-all' as const,
  },
  dialogError: {
    mt: 1,
    fontSize: 13,
    color: theme.palette.foundation.status.danger,
  },
});

export type WebWorkspaceTreeSx = ReturnType<typeof webWorkspaceTreeStyles>;
