import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface JsonEditorFieldSx {
  root: S;
  headerRow: S;
  label: S;
  formatButton: S;
  editorContainer: S;
  editorContainerError: S;
  helperText: S;
  helperTextError: S;
  placeholder: S;
}

export const jsonEditorFieldStyles = (theme: Theme): JsonEditorFieldSx => {
  const fp = theme.palette.foundation;
  const editorContainerBase: S = {
    position: 'relative',
    borderRadius: 1,
    overflow: 'hidden',
  };
  return {
    root: { display: 'flex', flexDirection: 'column', mt: 2 },
    headerRow: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 0.5 },
    label: { fontSize: 12, color: fp.text.muted },
    formatButton: { textTransform: 'none', fontSize: 12, py: 0.25, minWidth: 0 },
    editorContainer: { ...editorContainerBase, border: `1px solid ${fp.divider}` },
    editorContainerError: { ...editorContainerBase, border: `1px solid ${fp.status.danger}` },
    helperText: { mt: 0.5, fontSize: 12, color: fp.text.muted },
    helperTextError: { mt: 0.5, fontSize: 12, color: fp.status.danger },
    placeholder: {
      position: 'absolute',
      top: 0,
      left: 0,
      right: 0,
      pl: '52px',
      pt: '2px',
      fontSize: 12.5,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      color: fp.text.muted,
      pointerEvents: 'none',
      whiteSpace: 'pre',
      overflow: 'hidden',
    },
  };
};
