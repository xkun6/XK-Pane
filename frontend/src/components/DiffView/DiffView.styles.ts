import type { Theme } from '@mui/material/styles';
import type { SystemStyleObject } from '@mui/system';

type Styles = Record<string, SystemStyleObject<Theme>>;

export const diffViewStyles = (theme: Theme): Styles => ({
  root: {
    border: `1px solid ${theme.palette.foundation.divider}`,
    borderRadius: 8,
    overflow: 'hidden',
    fontFamily: 'monospace',
    fontSize: 12.5,
    backgroundColor: theme.palette.foundation.bg.base,
  },
  message: {
    padding: '20px 16px',
    textAlign: 'center',
    color: theme.palette.foundation.text.secondary,
  },
  stats: {
    display: 'flex',
    gap: 1.5,
    padding: '6px 12px',
    borderBottom: `1px solid ${theme.palette.foundation.divider}`,
    backgroundColor: theme.palette.foundation.bg.surface,
    fontSize: 12,
  },
  statAdd: { color: theme.palette.foundation.status.success, fontWeight: 600 },
  statDel: { color: theme.palette.foundation.status.danger, fontWeight: 600 },
  hunkHeader: {
    padding: '3px 12px',
    backgroundColor: theme.palette.foundation.bg.surface,
    color: theme.palette.foundation.text.muted,
    borderTop: `1px solid ${theme.palette.foundation.divider}`,
  },
  line: {
    display: 'flex',
    alignItems: 'stretch',
    minHeight: 18,
  },
  gutter: {
    width: 44,
    flexShrink: 0,
    textAlign: 'right',
    padding: '0 6px',
    color: theme.palette.foundation.text.muted,
    userSelect: 'none',
    borderRight: `1px solid ${theme.palette.foundation.divider}`,
  },
  sign: {
    width: 16,
    flexShrink: 0,
    textAlign: 'center',
    userSelect: 'none',
    color: theme.palette.foundation.text.muted,
  },
  content: {
    flex: 1,
    whiteSpace: 'pre',
    padding: '0 8px',
    overflowX: 'auto',
  },
});
