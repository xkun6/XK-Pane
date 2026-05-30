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
  // —— SCM 源代码管理 ——
  scmBlock: {
    display: 'flex',
    flexDirection: 'column',
    gap: 1.25,
  },
  commitBox: {
    display: 'flex',
    flexDirection: 'column',
    gap: 1,
  },
  commitButtonRow: {
    display: 'flex',
    justifyContent: 'flex-end',
  },
  fileGroup: {
    display: 'flex',
    flexDirection: 'column',
    gap: 0.25,
    border: `1px solid ${theme.palette.foundation.divider}`,
    borderRadius: 8,
    overflow: 'hidden',
  },
  fileGroupHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: 0.5,
    padding: '6px 6px 6px 12px',
    backgroundColor: theme.palette.foundation.bg.elevated,
    borderBottom: `1px solid ${theme.palette.foundation.divider}`,
  },
  fileGroupTitle: {
    flex: 1,
    fontSize: 12,
    fontWeight: 600,
    color: theme.palette.foundation.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: 1,
  },
  fileRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '5px 8px 5px 12px',
    cursor: 'pointer',
    '&:hover': { backgroundColor: theme.palette.foundation.bg.hover },
  },
  fileRowSelected: {
    backgroundColor: theme.palette.foundation.bg.hover,
  },
  fileBadge: {
    width: 16,
    textAlign: 'center',
    fontSize: 13,
    fontWeight: 700,
    fontFamily: 'monospace',
    flexShrink: 0,
  },
  filePath: {
    flex: 1,
    fontSize: 13,
    fontFamily: 'monospace',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  fileActions: {
    display: 'flex',
    alignItems: 'center',
    gap: 0.25,
    flexShrink: 0,
  },
  emptyScm: {
    padding: '14px 12px',
    fontSize: 13,
    color: theme.palette.foundation.text.secondary,
  },
  diffPanel: {
    display: 'flex',
    flexDirection: 'column',
    border: `1px solid ${theme.palette.foundation.divider}`,
    borderRadius: 8,
    overflow: 'hidden',
  },
  diffPanelHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '6px 6px 6px 12px',
    backgroundColor: theme.palette.foundation.bg.elevated,
    borderBottom: `1px solid ${theme.palette.foundation.divider}`,
  },
  diffPanelPath: {
    flex: 1,
    fontSize: 13,
    fontFamily: 'monospace',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  diffPanelBody: {
    maxHeight: 420,
    overflow: 'auto',
  },
  // —— worktree ——
  worktreeAddRow: {
    display: 'flex',
    gap: 1,
    alignItems: 'center',
  },
  worktreeList: {
    display: 'flex',
    flexDirection: 'column',
    gap: 0.5,
  },
  worktreeRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '8px 12px',
    borderRadius: 8,
    border: `1px solid ${theme.palette.foundation.divider}`,
    backgroundColor: theme.palette.foundation.bg.elevated,
  },
  worktreePath: {
    flex: 1,
    fontSize: 13,
    fontFamily: 'monospace',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
});
