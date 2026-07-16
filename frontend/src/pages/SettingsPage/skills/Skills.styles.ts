import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface SkillsSx {
  root: S;
  toolbar: S;
  toolbarBtn: S;
  list: S;
  empty: S;
  row: S;
  rowMain: S;
  rowTitleLine: S;
  rowName: S;
  rowId: S;
  rowDescription: S;
  rowToggles: S;
  toggleItem: S;
  toggleLabel: S;
  rowActions: S;
}

export const skillsStyles = (theme: Theme): SkillsSx => {
  const fp = theme.palette.foundation;
  return {
    root: { display: 'flex', flexDirection: 'column', gap: 2 },
    toolbar: { display: 'flex', justifyContent: 'flex-end', gap: 1 },
    toolbarBtn: { textTransform: 'none', borderRadius: 1.5 },
    list: { display: 'flex', flexDirection: 'column', gap: 1 },
    empty: {
      px: 2,
      py: 4,
      fontSize: 13,
      color: fp.text.muted,
      textAlign: 'center',
      border: `1px dashed ${fp.divider}`,
      borderRadius: 1,
    },
    row: {
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      px: 2,
      py: 1.25,
      borderRadius: 1,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
    },
    rowMain: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 0.25 },
    rowTitleLine: { display: 'flex', alignItems: 'baseline', gap: 1, minWidth: 0 },
    rowName: {
      fontSize: 14,
      fontWeight: 600,
      color: fp.text.primary,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    rowId: {
      fontSize: 11,
      color: fp.text.muted,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    rowDescription: {
      fontSize: 12,
      color: fp.text.secondary,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    rowToggles: { display: 'flex', alignItems: 'center', gap: 1.5, flexShrink: 0 },
    toggleItem: { display: 'flex', alignItems: 'center', gap: 0.25, m: 0 },
    toggleLabel: { fontSize: 11, color: fp.text.muted },
    rowActions: { display: 'flex', alignItems: 'center', gap: 0.5, flexShrink: 0 },
  };
};
