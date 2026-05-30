import type { Theme } from '@mui/material/styles';
import type { SystemStyleObject } from '@mui/system';

type Styles = Record<string, SystemStyleObject<Theme>>;

export const specsPageStyles = (theme: Theme): Styles => ({
  root: {
    height: '100%',
    overflowY: 'auto',
    padding: '24px 28px',
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
    backgroundColor: theme.palette.foundation.bg.content,
    color: theme.palette.foundation.text.primary,
  },
  header: {
    display: 'flex',
    flexDirection: 'column',
    gap: 0.5,
  },
  eyebrow: {
    fontSize: 12,
    letterSpacing: 1.5,
    textTransform: 'uppercase',
    color: theme.palette.foundation.text.secondary,
  },
  title: {
    fontSize: 22,
    fontWeight: 600,
  },
  pickerRow: {
    display: 'flex',
    gap: 1,
    alignItems: 'center',
  },
  createRow: {
    display: 'flex',
    gap: 1,
    alignItems: 'center',
  },
  empty: {
    padding: '32px 0',
    textAlign: 'center',
    color: theme.palette.foundation.text.secondary,
  },
  sectionTitle: {
    fontSize: 13,
    fontWeight: 600,
    color: theme.palette.foundation.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: 1,
  },
  list: {
    display: 'flex',
    flexDirection: 'column',
    gap: 0.5,
  },
  row: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '8px 12px',
    borderRadius: 8,
    border: `1px solid ${theme.palette.foundation.divider}`,
    backgroundColor: theme.palette.foundation.bg.elevated,
  },
  rowSelected: {
    borderColor: theme.palette.foundation.accent,
  },
  rowMain: {
    flex: 1,
    minWidth: 0,
    display: 'flex',
    flexDirection: 'column',
    gap: 0.25,
    cursor: 'pointer',
  },
  specTitle: {
    fontSize: 14,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  fileName: {
    fontSize: 12,
    fontFamily: 'monospace',
    color: theme.palette.foundation.text.secondary,
  },
  editorSection: {
    display: 'flex',
    flexDirection: 'column',
    gap: 1,
  },
  editorHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
  },
  editorTitle: {
    flex: 1,
    fontSize: 14,
    fontWeight: 600,
  },
  editor: {
    '& textarea': {
      fontFamily: 'monospace',
      fontSize: 13,
      lineHeight: 1.6,
    },
  },
});
