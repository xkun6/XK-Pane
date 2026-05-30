import type { Theme } from '@mui/material/styles';
import type { SystemStyleObject } from '@mui/system';

type Styles = Record<string, SystemStyleObject<Theme>>;

export const historyPageStyles = (theme: Theme): Styles => ({
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
  snapshotRow: {
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
  diffSection: {
    display: 'flex',
    flexDirection: 'column',
    gap: 0.75,
  },
  diffTitle: {
    fontSize: 13,
    fontFamily: 'monospace',
    color: theme.palette.foundation.text.primary,
  },
  filePath: {
    fontSize: 13,
    fontFamily: 'monospace',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  timestamp: {
    fontSize: 12,
    color: theme.palette.foundation.text.secondary,
  },
});
