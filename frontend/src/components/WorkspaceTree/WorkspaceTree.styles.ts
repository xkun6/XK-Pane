import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
export interface WorkspaceTreeSx {
  root: S;
  header: S;
  title: S;
  list: S;
  empty: S;
  emptyText: S;
  emptyHint: S;
  wsRow: S;
  chevron: S;
  wsName: S;
  countPill: S;
  starBtn: S;
  starOn: S;
  deleteBtn: S;
  projectList: S;
  projectRow: S;
  projectChevron: S;
  projectIcon: S;
  projectText: S;
  projectName: S;
  projectPath: S;
  projectEmpty: S;
  newButton: S;
  sessionList: S;
  sessionRow: S;
  sessionGlyph: S;
  sessionDesc: S;
  sessionTime: S;
  sessionHint: S;
}

export const workspaceTreeStyles = (theme: Theme): WorkspaceTreeSx => {
  const fp = theme.palette.foundation;
  return {
    root: {
      width: 240,
      flexShrink: 0,
      display: 'flex',
      flexDirection: 'column',
      minHeight: 0,
      borderRight: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.sidebar,
    },
    header: {
      display: 'flex',
      alignItems: 'center',
      height: 40,
      px: 1.5,
      flexShrink: 0,
      borderBottom: `1px solid ${fp.divider}`,
    },
    title: {
      fontSize: 12,
      fontWeight: 600,
      color: fp.text.secondary,
      textTransform: 'uppercase',
      letterSpacing: 0.5,
    },
    list: { flex: 1, minHeight: 0, overflowY: 'auto', overflowX: 'hidden', py: 0.5 },
    empty: { px: 1.5, py: 3, textAlign: 'center' },
    emptyText: { fontSize: 13, color: fp.text.secondary },
    emptyHint: { fontSize: 11, color: fp.text.muted, mt: 0.5, lineHeight: 1.5 },
    wsRow: {
      display: 'flex',
      alignItems: 'center',
      gap: 0.5,
      px: 1,
      py: 0.75,
      cursor: 'pointer',
      '& .xk-ws-actions': { opacity: 0, transition: 'opacity 120ms ease' },
      '&:hover': { backgroundColor: fp.bg.hover },
      '&:hover .xk-ws-actions, &:focus-within .xk-ws-actions': { opacity: 1 },
      '&:focus-visible': { outline: `2px solid ${fp.accent}`, outlineOffset: '-2px' },
    },
    chevron: { color: fp.text.muted, flexShrink: 0, transition: 'transform 120ms ease' },
    wsName: {
      flex: 1,
      minWidth: 0,
      fontSize: 13,
      color: fp.text.primary,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    countPill: {
      flexShrink: 0,
      fontSize: 11,
      lineHeight: '18px',
      minWidth: 18,
      textAlign: 'center',
      px: 0.5,
      borderRadius: '6px',
      color: fp.text.muted,
      backgroundColor: fp.bg.elevated,
    },
    starBtn: { flexShrink: 0, color: fp.text.muted },
    starOn: { color: fp.accent },
    deleteBtn: {
      flexShrink: 0,
      color: fp.text.muted,
      '&:hover': { color: fp.status.danger },
    },
    projectList: {},
    projectRow: {
      display: 'flex',
      alignItems: 'center',
      gap: 0.5,
      pl: 2,
      pr: 1.5,
      py: 0.5,
      '&:hover': { backgroundColor: fp.bg.hover },
    },
    projectChevron: { flexShrink: 0, color: fp.text.muted, p: 0.25 },
    projectIcon: { color: fp.text.secondary, flexShrink: 0 },
    projectText: { flex: 1, minWidth: 0 },
    projectName: {
      fontSize: 12.5,
      color: fp.text.primary,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    projectPath: {
      fontSize: 11,
      color: fp.text.muted,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    projectEmpty: { pl: 4, py: 0.5, fontSize: 11, color: fp.text.muted },
    sessionList: { pl: 5.5, pr: 1.5, pb: 0.5 },
    sessionRow: {
      display: 'flex',
      alignItems: 'center',
      gap: 0.75,
      px: 0.75,
      py: 0.5,
      borderRadius: '6px',
      cursor: 'pointer',
      color: fp.text.secondary,
      '&:hover': { backgroundColor: fp.bg.hover, color: fp.text.primary },
    },
    sessionGlyph: { flexShrink: 0, color: fp.text.muted },
    sessionDesc: {
      flex: 1,
      minWidth: 0,
      fontSize: 11,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    sessionTime: { flexShrink: 0, fontSize: 10, color: fp.text.muted },
    sessionHint: { px: 0.75, py: 0.5, fontSize: 10, color: fp.text.muted },
    newButton: {
      m: 1,
      py: 0.75,
      gap: 0.5,
      flexShrink: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      border: `1px dashed ${fp.divider}`,
      borderRadius: '8px',
      fontSize: 12,
      color: fp.text.secondary,
      cursor: 'pointer',
      transition: 'all 120ms ease',
      '&:hover': { backgroundColor: fp.bg.hover, color: fp.text.primary, borderColor: fp.accent },
      '&:focus-visible': { outline: `2px solid ${fp.accent}`, outlineOffset: '-2px' },
    },
  };
};
