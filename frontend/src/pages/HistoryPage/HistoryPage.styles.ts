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
  header: { display: 'flex', flexDirection: 'column', gap: 0.5 },
  eyebrow: {
    fontSize: 12,
    letterSpacing: 1.5,
    textTransform: 'uppercase',
    color: theme.palette.foundation.text.secondary,
  },
  title: { fontSize: 22, fontWeight: 600 },
  pickerRow: { display: 'flex', gap: 1, alignItems: 'center' },
  empty: { padding: '32px 0', textAlign: 'center', color: theme.palette.foundation.text.secondary },
  hint: { fontSize: 12, color: theme.palette.foundation.text.muted },

  // —— 阶段 1：最近变更文件列表 ——
  fileList: { display: 'flex', flexDirection: 'column', gap: 0.5 },
  fileRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '8px 12px',
    borderRadius: 8,
    cursor: 'pointer',
    border: `1px solid transparent`,
    '&:hover': { backgroundColor: theme.palette.foundation.bg.hover },
  },
  fileIcon: { color: theme.palette.foundation.text.muted, flexShrink: 0 },
  fileMain: { display: 'flex', flexDirection: 'column', minWidth: 0, flex: 1 },
  fileName: { fontSize: 14, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  fileDir: {
    fontSize: 12,
    color: theme.palette.foundation.text.muted,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  fileMeta: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    flexShrink: 0,
    fontSize: 12,
    color: theme.palette.foundation.text.secondary,
  },

  // —— 阶段 2：版本时间线 + 4-tab 预览 ——
  stageHeader: { display: 'flex', alignItems: 'center', gap: 1 },
  stagePath: { fontSize: 14, fontWeight: 600, fontFamily: 'monospace', wordBreak: 'break-all' },
  // 两栏固定高度，子项填满并各自滚动 —— 给确定高度避免 flex 在版本数少时把预览栏压扁、内容溢出叠到下方。
  stage2: { display: 'flex', gap: 1.5, height: 460, minHeight: 0 },
  timeline: {
    width: 260,
    flexShrink: 0,
    display: 'flex',
    flexDirection: 'column',
    gap: 0.5,
    height: '100%',
    minHeight: 0,
    overflowY: 'auto',
    paddingRight: 0.5,
  },
  versionRow: {
    display: 'flex',
    flexDirection: 'column',
    gap: 0.5,
    padding: '8px 10px',
    borderRadius: 8,
    cursor: 'pointer',
    borderLeft: `3px solid transparent`,
    backgroundColor: theme.palette.foundation.bg.elevated,
    '&:hover': { backgroundColor: theme.palette.foundation.bg.hover },
  },
  versionRowSelected: {
    borderLeft: `3px solid ${theme.palette.foundation.accent}`,
    backgroundColor: theme.palette.foundation.bg.hover,
  },
  versionTime: { fontSize: 12, color: theme.palette.foundation.text.primary },
  versionMeta: {
    display: 'flex',
    alignItems: 'center',
    gap: 0.75,
    flexWrap: 'wrap',
    fontSize: 11,
    color: theme.palette.foundation.text.secondary,
  },
  versionBadges: { display: 'flex', alignItems: 'center', gap: 0.5, flexWrap: 'wrap' },

  preview: {
    flex: 1,
    minWidth: 0,
    minHeight: 0,
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden', // 裁剪：内容只在 previewBody 内滚动，绝不溢出到面板外
    border: `1px solid ${theme.palette.foundation.divider}`,
    borderRadius: 8,
    backgroundColor: theme.palette.foundation.bg.elevated,
  },
  previewToolbar: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '6px 10px',
    borderBottom: `1px solid ${theme.palette.foundation.divider}`,
    flexWrap: 'wrap',
    flexShrink: 0,
  },
  previewBody: { flex: 1, minWidth: 0, minHeight: 0, overflow: 'auto', padding: '4px 10px' },
  previewCaption: { fontSize: 12, color: theme.palette.foundation.text.muted, padding: '6px 10px 0' },
  contentPre: {
    margin: 0,
    fontFamily: 'monospace',
    fontSize: 12,
    lineHeight: 1.5,
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word',
    color: theme.palette.foundation.text.primary,
  },

  // —— deleted / project-restore 列表 ——
  list: { display: 'flex', flexDirection: 'column', gap: 0.5, padding: '6px 0' },
  listRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    padding: '8px 12px',
    borderRadius: 8,
    backgroundColor: theme.palette.foundation.bg.content,
    border: `1px solid ${theme.palette.foundation.divider}`,
  },
  listMain: { display: 'flex', flexDirection: 'column', minWidth: 0, flex: 1, gap: 0.25 },
  listTitle: { fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  listMeta: { display: 'flex', alignItems: 'center', gap: 0.75, fontSize: 11, color: theme.palette.foundation.text.secondary },

  // —— 操作区：手动快照 / 配置 / 统计 ——
  section: { display: 'flex', flexDirection: 'column', gap: 0.75 },
  sectionTitle: {
    fontSize: 13,
    fontWeight: 600,
    color: theme.palette.foundation.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: 1,
  },
  snapshotRow: { display: 'flex', gap: 1, alignItems: 'center' },
  configGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
    gap: 1.5,
  },
  configActions: { display: 'flex', gap: 1, alignItems: 'center', flexWrap: 'wrap' },
  statsSummary: { fontSize: 13, color: theme.palette.foundation.text.secondary },
});
