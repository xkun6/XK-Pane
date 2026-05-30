import type { Theme } from '@mui/material/styles';
import type { SystemStyleObject } from '@mui/system';

type Styles = Record<string, SystemStyleObject<Theme>>;

export const gitPageStyles = (theme: Theme): Styles => ({
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
  empty: {
    padding: '32px 0',
    textAlign: 'center',
    color: theme.palette.foundation.text.secondary,
  },
  // —— 状态卡 ——
  statusCard: {
    display: 'flex',
    alignItems: 'center',
    gap: 1.5,
    padding: '12px 14px',
    borderRadius: 8,
    border: `1px solid ${theme.palette.foundation.divider}`,
    backgroundColor: theme.palette.foundation.bg.elevated,
  },
  branchName: {
    fontSize: 15,
    fontWeight: 600,
    fontFamily: 'monospace',
  },
  toolbar: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: 1,
  },
  // —— 分区 ——
  section: {
    display: 'flex',
    flexDirection: 'column',
    gap: 0.75,
  },
  sectionTitle: {
    fontSize: 13,
    fontWeight: 600,
    color: theme.palette.foundation.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: 1,
  },
  branchList: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: 0.75,
  },
  commitList: {
    display: 'flex',
    flexDirection: 'column',
    gap: 0.5,
  },
  commitRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '8px 12px',
    borderRadius: 8,
    border: `1px solid ${theme.palette.foundation.divider}`,
    backgroundColor: theme.palette.foundation.bg.elevated,
  },
  commitHash: {
    fontSize: 12,
    fontFamily: 'monospace',
    color: theme.palette.foundation.text.secondary,
    whiteSpace: 'nowrap',
  },
  commitSubject: {
    flex: 1,
    fontSize: 13,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  commitMeta: {
    fontSize: 12,
    color: theme.palette.foundation.text.secondary,
    whiteSpace: 'nowrap',
  },
});
