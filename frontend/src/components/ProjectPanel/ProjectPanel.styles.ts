import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface ProjectPanelSx {
  root: S;
  header: S;
  title: S;
  addBtn: S;
  list: S;
  empty: S;
  row: S;
  rowIcon: S;
  rowText: S;
  rowName: S;
  rowPath: S;
  removeBtn: S;
}

export const projectPanelStyles = (theme: Theme): ProjectPanelSx => {
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
      justifyContent: 'space-between',
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
    addBtn: { borderRadius: 1.5, color: fp.text.secondary },
    list: { flex: 1, minHeight: 0, overflowY: 'auto', py: 0.5 },
    empty: {
      px: 1.5,
      py: 3,
      fontSize: 12,
      lineHeight: 1.6,
      color: fp.text.muted,
      textAlign: 'center',
    },
    row: {
      display: 'flex',
      alignItems: 'center',
      gap: 1,
      px: 1.5,
      py: 0.75,
      cursor: 'pointer',
      '& .xk-project-remove': { opacity: 0, transition: 'opacity 120ms ease' },
      '&:hover': { backgroundColor: fp.bg.hover },
      '&:hover .xk-project-remove, &:focus-within .xk-project-remove': { opacity: 1 },
      '&:focus-visible': { outline: `2px solid ${fp.accent}`, outlineOffset: '-2px' },
    },
    rowIcon: { color: fp.text.secondary, flexShrink: 0 },
    rowText: { flex: 1, minWidth: 0 },
    rowName: {
      fontSize: 13,
      color: fp.text.primary,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    rowPath: {
      fontSize: 11,
      color: fp.text.muted,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    removeBtn: {
      flexShrink: 0,
      color: fp.text.muted,
      '&:hover': { color: fp.status.danger },
    },
  };
};
