import type { Theme } from '@mui/material/styles';
import type { SystemStyleObject } from '@mui/system';

type Styles = Record<string, SystemStyleObject<Theme>>;

export type SessionsPageSx = Styles;

export const sessionsPageStyles = (theme: Theme): Styles => ({
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
  section: {
    display: 'flex',
    flexDirection: 'column',
    gap: 0.5,
  },
  sectionHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  sectionTitle: {
    fontSize: 13,
    fontWeight: 600,
    color: theme.palette.foundation.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: 1,
  },
  empty: {
    padding: '12px 0',
    color: theme.palette.foundation.text.muted,
    fontSize: 13,
  },
  // ── 活跃会话 ──
  activeRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '8px 12px',
    borderRadius: 8,
    cursor: 'pointer',
    color: theme.palette.foundation.text.secondary,
    '&:hover': {
      backgroundColor: theme.palette.foundation.bg.hover,
      color: theme.palette.foundation.text.primary,
    },
  },
  activeIconWrap: {
    position: 'relative',
    display: 'inline-flex',
    flexShrink: 0,
  },
  activeDot: {
    position: 'absolute',
    bottom: -2,
    right: -2,
    width: 8,
    height: 8,
    borderRadius: '50%',
    border: `1.5px solid ${theme.palette.foundation.bg.content}`,
  },
  activeDotOn: {
    backgroundColor: theme.palette.foundation.status.success,
  },
  activeDotIdle: {
    backgroundColor: theme.palette.foundation.text.muted,
  },
  activeTitle: {
    fontSize: 13,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  // ── 最近启动分组 ──
  groupHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: 0.75,
    width: '100%',
    padding: '6px 8px',
    borderRadius: 6,
    cursor: 'pointer',
    color: theme.palette.foundation.text.secondary,
    '&:hover': {
      backgroundColor: theme.palette.foundation.bg.hover,
      color: theme.palette.foundation.text.primary,
    },
  },
  groupName: {
    fontSize: 12,
    fontWeight: 600,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  countPill: {
    marginLeft: 'auto',
    flexShrink: 0,
    fontSize: 11,
    lineHeight: 1.6,
    minWidth: 20,
    textAlign: 'center',
    padding: '0 6px',
    borderRadius: 999,
    color: theme.palette.foundation.text.muted,
    backgroundColor: theme.palette.foundation.bg.hover,
  },
  launchRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '8px 12px 8px 28px',
    borderRadius: 8,
    cursor: 'pointer',
    color: theme.palette.foundation.text.secondary,
    '&:hover': {
      backgroundColor: theme.palette.foundation.bg.hover,
      color: theme.palette.foundation.text.primary,
    },
    '&:hover .sessions-row-actions': { opacity: 1 },
  },
  launchMain: {
    flex: 1,
    minWidth: 0,
    display: 'flex',
    flexDirection: 'column',
    gap: 0.25,
  },
  projectName: {
    fontSize: 13,
    fontWeight: 500,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  sessionId: {
    fontSize: 11,
    fontFamily: 'monospace',
    color: theme.palette.foundation.text.muted,
  },
  lastPrompt: {
    fontSize: 12,
    color: theme.palette.foundation.text.muted,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  launchMeta: {
    display: 'flex',
    alignItems: 'center',
    gap: 0.5,
    flexShrink: 0,
  },
  time: {
    fontSize: 11,
    color: theme.palette.foundation.text.muted,
  },
  rowActions: {
    display: 'flex',
    alignItems: 'center',
    gap: 0.25,
    opacity: 0,
    transition: 'opacity 0.15s',
  },
  // ── ResumeDetailPopover ──
  popover: {
    width: 288,
    padding: 1.5,
    display: 'flex',
    flexDirection: 'column',
    gap: 1.25,
    backgroundColor: theme.palette.foundation.bg.surface,
    color: theme.palette.foundation.text.primary,
  },
  popProjectName: {
    fontSize: 13,
    fontWeight: 600,
  },
  popProjectPath: {
    fontSize: 11,
    color: theme.palette.foundation.text.muted,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  popRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 0.75,
  },
  popLabel: {
    fontSize: 11,
    color: theme.palette.foundation.text.muted,
  },
  popCode: {
    fontSize: 11,
    fontFamily: 'monospace',
    padding: '1px 5px',
    borderRadius: 4,
    backgroundColor: theme.palette.foundation.bg.elevated,
    color: theme.palette.foundation.text.secondary,
  },
  popValue: {
    fontSize: 11,
    color: theme.palette.foundation.text.secondary,
  },
  popPrompt: {
    fontSize: 11,
    lineHeight: 1.5,
    color: theme.palette.foundation.text.secondary,
    display: '-webkit-box',
    WebkitLineClamp: 3,
    WebkitBoxOrient: 'vertical',
    overflow: 'hidden',
  },
  popActions: {
    display: 'flex',
    gap: 1,
    paddingTop: 0.5,
  },
});
