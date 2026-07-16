import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface InstructionsSx {
  root: S;
  tabs: S;
  tab: S;
  toolbar: S;
  pathLabel: S;
  toolbarActions: S;
  unsavedBadge: S;
  loadErrorText: S;
  editorContainer: S;
}

export const instructionsStyles = (theme: Theme): InstructionsSx => {
  const fp = theme.palette.foundation;
  return {
    root: { display: 'flex', flexDirection: 'column', gap: 2, maxWidth: 900 },
    tabs: { minHeight: 34, flexShrink: 0, borderBottom: `1px solid ${fp.divider}` },
    tab: { minHeight: 34, textTransform: 'none', fontSize: 13 },
    toolbar: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1.5 },
    pathLabel: {
      fontSize: 12,
      color: fp.text.muted,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
    },
    toolbarActions: { display: 'flex', alignItems: 'center', gap: 1 },
    unsavedBadge: { fontSize: 12, color: fp.status.warning, fontWeight: 600 },
    loadErrorText: { fontSize: 12, color: fp.status.danger },
    editorContainer: {
      height: 480,
      display: 'flex',
      flexDirection: 'column',
      border: `1px solid ${fp.divider}`,
      borderRadius: 1,
      overflow: 'hidden',
    },
  };
};
