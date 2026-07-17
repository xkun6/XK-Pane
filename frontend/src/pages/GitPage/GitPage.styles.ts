import type { Theme } from '@mui/material/styles';
import type { SystemStyleObject } from '@mui/system';

type Styles = Record<string, SystemStyleObject<Theme>>;

export const gitPageStyles = (theme: Theme): Styles => {
  const fp = theme.palette.foundation;
  const isDark = theme.palette.mode === 'dark';
  const cardShadow = isDark
    ? '0 1px 2px rgba(0,0,0,0.28)'
    : '0 1px 2px rgba(15, 23, 42, 0.04)';
  return {
  root: {
    height: '100%',
    overflowY: 'auto',
    padding: '28px 32px',
    display: 'flex',
    flexDirection: 'column',
    gap: 2.5,
    backgroundColor: fp.bg.content,
    color: fp.text.primary,
  },
  header: {
    display: 'flex',
    flexDirection: 'column',
    gap: 0.5,
  },
  eyebrow: {
    fontSize: 11,
    fontWeight: 600,
    letterSpacing: 1.2,
    textTransform: 'uppercase',
    color: fp.text.muted,
  },
  title: {
    fontSize: 26,
    fontWeight: 700,
    letterSpacing: '-0.025em',
    lineHeight: 1.15,
  },
  pickerRow: {
    display: 'flex',
    gap: 1,
    alignItems: 'center',
  },
  empty: {
    padding: '40px 16px',
    textAlign: 'center',
    color: fp.text.muted,
    borderRadius: '14px',
    border: `1px dashed ${fp.divider}`,
    backgroundColor: fp.bg.surface,
  },
  // —— 状态卡 ——
  statusCard: {
    display: 'flex',
    alignItems: 'center',
    gap: 1.5,
    padding: '14px 16px',
    borderRadius: '12px',
    border: `1px solid ${fp.divider}`,
    backgroundColor: fp.bg.surface,
    boxShadow: cardShadow,
  },
  branchName: {
    fontSize: 15,
    fontWeight: 600,
    fontFamily: 'ui-monospace, "JetBrains Mono", monospace',
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
    gap: 1,
  },
  sectionTitle: {
    fontSize: 11.5,
    fontWeight: 600,
    color: fp.text.muted,
    textTransform: 'uppercase',
    letterSpacing: 1.1,
  },
  branchList: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: 0.75,
  },
  commitList: {
    display: 'flex',
    flexDirection: 'column',
    gap: 0.65,
  },
  commitRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '10px 14px',
    borderRadius: '10px',
    border: `1px solid ${fp.divider}`,
    backgroundColor: fp.bg.surface,
    boxShadow: cardShadow,
  },
  commitHash: {
    fontSize: 12,
    fontFamily: 'ui-monospace, "JetBrains Mono", monospace',
    color: fp.text.secondary,
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
    color: fp.text.secondary,
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
    border: `1px solid ${fp.divider}`,
    borderRadius: '12px',
    overflow: 'hidden',
    backgroundColor: fp.bg.surface,
    boxShadow: cardShadow,
  },
  fileGroupHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: 0.5,
    padding: '8px 8px 8px 14px',
    backgroundColor: fp.bg.elevated,
    borderBottom: `1px solid ${fp.divider}`,
  },
  fileGroupTitle: {
    flex: 1,
    fontSize: 11.5,
    fontWeight: 600,
    color: fp.text.muted,
    textTransform: 'uppercase',
    letterSpacing: 1,
  },
  fileRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '6px 10px 6px 14px',
    cursor: 'pointer',
    transition: 'background-color 120ms ease',
    '&:hover': { backgroundColor: fp.bg.hover },
  },
  fileRowSelected: {
    backgroundColor: fp.bg.active,
  },
  fileBadge: {
    width: 16,
    textAlign: 'center',
    fontSize: 13,
    fontWeight: 700,
    fontFamily: 'ui-monospace, "JetBrains Mono", monospace',
    flexShrink: 0,
  },
  filePath: {
    flex: 1,
    fontSize: 13,
    fontFamily: 'ui-monospace, "JetBrains Mono", monospace',
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
    padding: '16px 14px',
    fontSize: 13,
    color: fp.text.muted,
  },
  diffPanel: {
    display: 'flex',
    flexDirection: 'column',
    border: `1px solid ${fp.divider}`,
    borderRadius: '12px',
    overflow: 'hidden',
    backgroundColor: fp.bg.surface,
    boxShadow: cardShadow,
  },
  diffPanelHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '8px 8px 8px 14px',
    backgroundColor: fp.bg.elevated,
    borderBottom: `1px solid ${fp.divider}`,
  },
  diffPanelPath: {
    flex: 1,
    fontSize: 13,
    fontFamily: 'ui-monospace, "JetBrains Mono", monospace',
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
    gap: 0.65,
  },
  worktreeRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '10px 14px',
    borderRadius: '10px',
    border: `1px solid ${fp.divider}`,
    backgroundColor: fp.bg.surface,
    boxShadow: cardShadow,
  },
  worktreePath: {
    flex: 1,
    fontSize: 13,
    fontFamily: 'ui-monospace, "JetBrains Mono", monospace',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
};
};
