import type { Theme } from '@mui/material/styles';
import type { SystemStyleObject } from '@mui/system';

type Styles = Record<string, SystemStyleObject<Theme>>;

// MC PE 页面 + 子面板共享样式工厂。配色只取 theme.palette.foundation.*，圆角容器 8 / 按钮 6。
export const mcpeStyles = (theme: Theme): Styles => ({
  root: {
    height: '100%',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
    backgroundColor: theme.palette.foundation.bg.content,
    color: theme.palette.foundation.text.primary,
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '8px 12px',
    borderBottom: `1px solid ${theme.palette.foundation.divider}`,
    flexShrink: 0,
  },
  headerTitle: { display: 'flex', alignItems: 'center', gap: 1, fontSize: 13, fontWeight: 600 },
  headerActions: { display: 'flex', alignItems: 'center', gap: 0.5 },
  platformWarn: {
    padding: '10px 12px',
    fontSize: 12,
    color: theme.palette.foundation.status.warning,
    borderBottom: `1px solid ${theme.palette.foundation.divider}`,
  },
  listContainer: {
    flexShrink: 0,
    maxHeight: '40%',
    overflowY: 'auto',
    borderBottom: `1px solid ${theme.palette.foundation.divider}`,
  },
  listEmpty: { padding: '12px', fontSize: 12, color: theme.palette.foundation.text.muted },
  row: {
    display: 'flex',
    alignItems: 'center',
    padding: '6px 10px',
    cursor: 'pointer',
    fontSize: 12,
    '&:hover': { backgroundColor: theme.palette.foundation.bg.hover },
    '&:hover .mcpe-row-del': { opacity: 1 },
  },
  rowActive: { backgroundColor: theme.palette.foundation.bg.active },
  rowName: { fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  rowPath: {
    fontSize: 10,
    color: theme.palette.foundation.text.muted,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  rowDel: { opacity: 0, transition: 'opacity 120ms', flexShrink: 0 },
  runBadge: {
    marginLeft: 1,
    fontSize: 10,
    color: theme.palette.foundation.status.success,
    flexShrink: 0,
    whiteSpace: 'nowrap',
  },
  detail: { flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column' },
  emptyState: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    height: '100%',
    padding: 3,
    gap: 1,
    fontSize: 12,
    textAlign: 'center',
    color: theme.palette.foundation.text.muted,
  },
  // 详情双 Tab
  tabsRow: {
    display: 'flex',
    borderBottom: `1px solid ${theme.palette.foundation.divider}`,
    paddingX: 0.5,
    flexShrink: 0,
    minHeight: 0,
  },
  detailBody: { flex: 1, overflowY: 'auto' },
  // 配置表单
  form: { padding: 1.5, fontSize: 12, display: 'flex', flexDirection: 'column' },
  field: { display: 'flex', flexDirection: 'column', gap: 0.5, marginBottom: 1.5 },
  fieldLabel: { fontSize: 12, color: theme.palette.foundation.text.secondary },
  fieldHint: { fontSize: 10, color: theme.palette.foundation.text.muted },
  readonlyPath: {
    fontSize: 11,
    fontFamily: 'monospace',
    padding: '6px 8px',
    borderRadius: 6,
    backgroundColor: theme.palette.foundation.bg.elevated,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  sectionTitle: { fontSize: 11, fontWeight: 600, marginBottom: 1 },
  divider: { borderTop: `1px solid ${theme.palette.foundation.divider}`, marginY: 1.5 },
  scanList: {
    marginTop: 0.5,
    maxHeight: 128,
    overflowY: 'auto',
    borderRadius: 6,
    border: `1px solid ${theme.palette.foundation.divider}`,
  },
  scanItem: {
    display: 'block',
    width: '100%',
    textAlign: 'left',
    padding: '4px 8px',
    fontSize: 10,
    fontFamily: 'monospace',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    border: 'none',
    background: 'transparent',
    color: theme.palette.foundation.text.primary,
    cursor: 'pointer',
    '&:hover': { backgroundColor: theme.palette.foundation.bg.hover },
  },
  bridgeLine: {
    fontSize: 10,
    marginTop: 1,
    marginLeft: 2,
    fontFamily: 'monospace',
    color: theme.palette.foundation.text.muted,
  },
  // 运行面板
  launchToolbar: {
    display: 'flex',
    alignItems: 'center',
    gap: 1,
    rowGap: 0.5,
    padding: 1,
    borderBottom: `1px solid ${theme.palette.foundation.divider}`,
    flexWrap: 'wrap',
    flexShrink: 0,
  },
  pidInfo: { fontSize: 10, whiteSpace: 'nowrap', flexShrink: 0, color: theme.palette.foundation.text.muted },
  bridgeBadge: { display: 'flex', alignItems: 'center', gap: 0.5, fontSize: 10, flexShrink: 0, whiteSpace: 'nowrap' },
  bridgeDot: { width: 6, height: 6, borderRadius: '50%' },
  logArea: {
    flex: 1,
    overflowY: 'auto',
    fontFamily: 'monospace',
    fontSize: 11,
    padding: 1,
    lineHeight: 1.4,
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word',
    userSelect: 'text', // 覆盖全局 body 的 userSelect:none —— 日志要可选中复制
    backgroundColor: theme.palette.foundation.bg.elevated,
  },
  logEmpty: { color: theme.palette.foundation.text.muted, fontStyle: 'italic' },
});

// severity → 颜色（取 theme 语义色）。对应 cc-pane SEVERITY_CLASS。
export function severityColor(theme: Theme, severity: string): string {
  switch (severity) {
    case 'error':
      return theme.palette.foundation.status.danger;
    case 'warn':
      return theme.palette.foundation.status.warning;
    case 'success':
      return theme.palette.foundation.status.success;
    case 'info':
      return theme.palette.foundation.text.primary;
    case 'verbose':
      return theme.palette.foundation.text.muted;
    default:
      return theme.palette.foundation.text.secondary; // developer / plain
  }
}
