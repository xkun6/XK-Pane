import type { Theme } from '@mui/material/styles';
import type { SystemStyleObject } from '@mui/system';

type Styles = Record<string, SystemStyleObject<Theme>>;

export const todoPageStyles = (theme: Theme): Styles => ({
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
  addRow: {
    display: 'flex',
    gap: 1,
    alignItems: 'center',
  },
  statsRow: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: 1,
    alignItems: 'center',
  },
  list: {
    display: 'flex',
    flexDirection: 'column',
    gap: 1,
  },
  empty: {
    padding: '32px 0',
    textAlign: 'center',
    color: theme.palette.foundation.text.secondary,
  },
  row: {
    display: 'flex',
    flexDirection: 'column',
    gap: 0.5,
    padding: '10px 12px',
    borderRadius: 8,
    border: `1px solid ${theme.palette.foundation.divider}`,
    backgroundColor: theme.palette.foundation.bg.elevated,
  },
  rowMain: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
  },
  rowTitle: {
    flex: 1,
    fontSize: 14,
  },
  rowTitleDone: {
    flex: 1,
    fontSize: 14,
    textDecoration: 'line-through',
    color: theme.palette.foundation.text.secondary,
  },
  priorityChip: {
    borderRadius: 6,
  },
  subList: {
    display: 'flex',
    flexDirection: 'column',
    gap: 0.25,
    paddingLeft: 4,
  },
  subRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 0.5,
  },
  subTitle: {
    flex: 1,
    fontSize: 13,
  },
  subTitleDone: {
    flex: 1,
    fontSize: 13,
    textDecoration: 'line-through',
    color: theme.palette.foundation.text.secondary,
  },
  subAddRow: {
    display: 'flex',
    gap: 0.5,
    alignItems: 'center',
    paddingLeft: 4,
    paddingTop: 0.25,
  },
});
