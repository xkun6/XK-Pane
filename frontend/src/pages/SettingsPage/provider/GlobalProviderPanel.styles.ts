import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface GlobalProviderPanelSx {
  root: S;
  toolbar: S;
  toolbarBtn: S;
  list: S;
  empty: S;
  row: S;
  rowCurrent: S;
  rowMain: S;
  rowTitleLine: S;
  rowName: S;
  categoryChip: S;
  currentChip: S;
  currentIcon: S;
  rowSub: S;
  rowActions: S;
  bottomHint: S;
  formField: S;
  currentEditWarning: S;
  baseCard: S;
  baseCardMain: S;
  baseCardTitle: S;
  baseCardDesc: S;
}

export const globalProviderPanelStyles = (theme: Theme): GlobalProviderPanelSx => {
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
    rowCurrent: {
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      px: 2,
      py: 1.25,
      borderRadius: 1,
      border: `1px solid ${fp.accent}`,
      backgroundColor: fp.bg.active,
    },
    rowMain: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 0.5 },
    rowTitleLine: { display: 'flex', alignItems: 'center', gap: 1, minWidth: 0 },
    rowName: {
      fontSize: 14,
      fontWeight: 600,
      color: fp.text.primary,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    categoryChip: { fontSize: 11, height: 20, color: fp.text.secondary, borderColor: fp.divider },
    currentChip: {
      fontSize: 11,
      height: 20,
      color: fp.accent,
      borderColor: fp.accent,
    },
    currentIcon: { color: fp.accent, flexShrink: 0 },
    rowSub: {
      fontSize: 12,
      color: fp.text.muted,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    rowActions: { display: 'flex', alignItems: 'center', gap: 0.5, flexShrink: 0 },
    bottomHint: { fontSize: 12, color: fp.text.muted, lineHeight: 1.6 },
    formField: { mt: 2 },
    currentEditWarning: {
      mt: 2,
      px: 1.5,
      py: 1,
      fontSize: 12,
      color: fp.text.secondary,
      borderRadius: 1,
      backgroundColor: fp.bg.elevated,
      border: `1px solid ${fp.divider}`,
    },
    baseCard: {
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      px: 2,
      py: 1.5,
      borderRadius: 1,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.elevated,
    },
    baseCardMain: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 0.25 },
    baseCardTitle: { fontSize: 14, fontWeight: 600, color: fp.text.primary },
    baseCardDesc: { fontSize: 12, color: fp.text.muted, lineHeight: 1.5 },
  };
};
