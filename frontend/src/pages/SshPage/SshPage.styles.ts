import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface SshPageSx {
  root: S;
  body: S;
  header: S;
  title: S;
  hint: S;
  toolbar: S;
  toolbarBtn: S;
  list: S;
  empty: S;
  emptyIcon: S;
  emptyText: S;
  emptyAction: S;
  row: S;
  rowIconWrap: S;
  rowIcon: S;
  statusDot: S;
  rowMain: S;
  rowTitleLine: S;
  rowName: S;
  typeChip: S;
  pwChip: S;
  tagChip: S;
  rowSub: S;
  rowDesc: S;
  rowActions: S;
  connectBtn: S;
  spinner: S;
}

// SSH 连接中心页（路由页，全屏）：复用 sshmachine 设置子页的行 / 表单视觉语言，
// 额外加状态点、连接按钮、CLI 启动菜单。配色全取 theme.palette.foundation.*。
export const sshPageStyles = (theme: Theme): SshPageSx => {
  const fp = theme.palette.foundation;
  return {
    root: {
      height: '100%',
      overflowY: 'auto',
      backgroundColor: fp.bg.content,
    },
    body: {
      maxWidth: 860,
      mx: 'auto',
      px: 4,
      py: 4,
      display: 'flex',
      flexDirection: 'column',
      gap: 2.5,
    },
    header: { display: 'flex', flexDirection: 'column', gap: 0.75 },
    title: { fontSize: 22, fontWeight: 700, color: fp.text.primary },
    hint: { fontSize: 13, color: fp.text.muted, lineHeight: 1.6 },
    toolbar: { display: 'flex', justifyContent: 'flex-end', gap: 1 },
    toolbarBtn: { textTransform: 'none', borderRadius: 1.5 },
    list: { display: 'flex', flexDirection: 'column', gap: 1 },
    empty: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: 1.5,
      px: 2,
      py: 6,
      border: `1px dashed ${fp.divider}`,
      borderRadius: 1,
    },
    emptyIcon: { fontSize: 40, color: fp.text.muted },
    emptyText: { fontSize: 13, color: fp.text.muted },
    emptyAction: { textTransform: 'none', borderRadius: 1.5 },
    row: {
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      px: 2,
      py: 1.25,
      borderRadius: 1,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
    },
    rowIconWrap: { position: 'relative', flexShrink: 0, display: 'flex' },
    rowIcon: { color: fp.text.secondary },
    statusDot: {
      position: 'absolute',
      right: -2,
      bottom: -2,
      width: 9,
      height: 9,
      borderRadius: '50%',
      border: `2px solid ${fp.bg.surface}`,
      boxSizing: 'content-box',
    },
    rowMain: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 0.5 },
    rowTitleLine: { display: 'flex', alignItems: 'center', gap: 0.75, minWidth: 0, flexWrap: 'wrap' },
    rowName: {
      fontSize: 14,
      fontWeight: 600,
      color: fp.text.primary,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      maxWidth: 260,
    },
    typeChip: { fontSize: 11, height: 20, color: fp.text.secondary, borderColor: fp.divider },
    pwChip: { fontSize: 11, height: 20, color: fp.accent, borderColor: fp.accent },
    tagChip: { fontSize: 11, height: 20, color: fp.text.muted, borderColor: fp.divider },
    rowSub: {
      fontSize: 12,
      color: fp.text.muted,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    rowDesc: {
      fontSize: 12,
      color: fp.text.muted,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    rowActions: { display: 'flex', alignItems: 'center', gap: 0.25, flexShrink: 0 },
    connectBtn: { textTransform: 'none', borderRadius: 1.5, mr: 0.5 },
    spinner: { color: fp.text.muted, mx: 0.5 },
  };
};
