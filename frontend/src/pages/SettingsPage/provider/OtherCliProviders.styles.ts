import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface OtherCliProvidersSx {
  root: S;
  hint: S;
  toolbar: S;
  addBtn: S;
  list: S;
  empty: S;
  row: S;
  rowMain: S;
  rowTitleLine: S;
  rowName: S;
  typeChip: S;
  defaultChip: S;
  rowSub: S;
  rowActions: S;
  formField: S;
}

export const otherCliProvidersStyles = (theme: Theme): OtherCliProvidersSx => {
  const fp = theme.palette.foundation;
  return {
    root: { display: 'flex', flexDirection: 'column', gap: 2, maxWidth: 720 },
    hint: { fontSize: 13, color: fp.text.muted, lineHeight: 1.6 },
    toolbar: { display: 'flex', justifyContent: 'flex-end' },
    addBtn: { textTransform: 'none', borderRadius: 1.5 },
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
    typeChip: { fontSize: 11, height: 20, color: fp.text.secondary, borderColor: fp.divider },
    defaultChip: { fontSize: 11, height: 20, color: fp.accent, borderColor: fp.accent },
    rowSub: {
      fontSize: 12,
      color: fp.text.muted,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    rowActions: { display: 'flex', alignItems: 'center', gap: 0.5, flexShrink: 0 },
    formField: { mt: 2 },
  };
};
