import { alpha } from '@mui/material/styles';
import type { SxProps, Theme } from '@mui/material';
import type { ToolTone } from './messages/toolMeta';

// 等宽字体栈（工具输入/结果用，与终端观感一致；不依赖 xterm 字体）。
const MONO = 'ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace';

// 卡片头部图标色调（折叠卡 / 工具卡）：accent=强调、danger=失败、success/warning=语义、muted=低调灰。
// 工具卡按 toolMeta.tone 取色，让「会改东西/执行类」凸显、「只读类」低调（避免一排卡片成无差别横杠）。
export type BlockTone = ToolTone | 'danger' | 'success' | 'warning';

export function resolveTone(theme: Theme, tone?: BlockTone): string | undefined {
  const fp = theme.palette.foundation;
  switch (tone) {
    case 'accent':
      return fp.accent;
    case 'danger':
      return fp.status.danger;
    case 'success':
      return fp.status.success;
    case 'warning':
      return fp.status.warning;
    case 'muted':
      return fp.text.muted;
    default:
      return undefined;
  }
}

export const sessionTimelineStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  return {
    // 覆盖在 viewerHost 之上的结构化视图（xterm 在底下继续跑，不受影响）。
    overlayRoot: {
      position: 'absolute',
      inset: 0,
      zIndex: 6,
      display: 'flex',
      flexDirection: 'column',
      minHeight: 0,
      backgroundColor: fp.bg.content,
    },
    header: {
      flexShrink: 0,
      minHeight: 44,
      display: 'flex',
      alignItems: 'center',
      gap: 1,
      pl: 1.5,
      pr: 1.5, // 视图切换控件已上移到 TabBar，覆盖层右上角无需再留位
      borderBottom: `1px solid ${fp.divider}`,
      color: fp.text.secondary,
    },
    headerTitle: {
      flex: 1,
      minWidth: 0,
      fontSize: 13,
      fontWeight: 600,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      color: fp.text.primary,
    },
    scroll: {
      flex: 1,
      minHeight: 0,
      overflowY: 'auto',
      overflowX: 'hidden',
      px: 1.5,
      py: 1.5,
      display: 'flex',
      flexDirection: 'column',
      gap: 1.25,
      WebkitOverflowScrolling: 'touch',
      // flex column 子项默认 flex-shrink:1；带 overflow:hidden 的折叠卡片(block)其 min-height:auto
      // 坍缩为 0 → 卡片一多就被压扁成一条 bg 横线（而非滚动）。钉死所有子项不压缩，超出走滚动。
      '& > *': { flexShrink: 0 },
    },
    empty: {
      flex: 1,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: fp.text.muted,
      fontSize: 13,
    },
    // —— 气泡 / 卡片 ——
    userRow: { display: 'flex', justifyContent: 'flex-end' },
    userBubble: {
      position: 'relative',
      maxWidth: '88%',
      backgroundColor: fp.bg.active,
      color: fp.text.primary,
      borderRadius: 2, // 8px
      px: 1.5,
      py: 1,
      fontSize: 14,
      lineHeight: 1.5,
      whiteSpace: 'pre-wrap',
      overflowWrap: 'anywhere',
      userSelect: 'text', // 解除全局 body userSelect:none —— 气泡正文可拖选复制
      '@media (hover: hover)': { '&:hover .xk-copy-btn': { opacity: 1 } },
    },
    assistantBubble: {
      position: 'relative',
      alignSelf: 'stretch',
      backgroundColor: fp.bg.elevated,
      color: fp.text.primary,
      borderRadius: 2,
      px: 1.5,
      py: 1,
      fontSize: 14,
      lineHeight: 1.55,
      whiteSpace: 'pre-wrap',
      overflowWrap: 'anywhere',
      userSelect: 'text', // 解除全局 body userSelect:none —— 气泡正文可拖选复制
      '@media (hover: hover)': { '&:hover .xk-copy-btn': { opacity: 1 } },
    },
    // 思考 / 工具：可折叠。填充 bg.elevated（与 content 区分，避免折叠态变成"透明横杠"）。
    block: {
      alignSelf: 'stretch',
      borderRadius: 2,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.elevated,
      overflow: 'hidden',
      '@media (hover: hover)': { '&:hover .xk-copy-btn': { opacity: 1 } },
    },
    blockHeader: {
      display: 'flex',
      alignItems: 'center',
      gap: 1,
      px: 1.25,
      py: 0.75,
      cursor: 'pointer',
      color: fp.text.secondary,
      userSelect: 'none',
      '&:hover': { backgroundColor: fp.bg.hover },
    },
    blockHeaderLabel: {
      flex: 1,
      minWidth: 0,
      fontSize: 13,
      fontWeight: 600,
      color: fp.text.primary,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    blockSummary: {
      flexShrink: 1,
      minWidth: 0,
      fontSize: 12,
      fontFamily: MONO,
      color: fp.text.muted,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    pre: {
      m: 0,
      px: 1.25,
      py: 1,
      borderTop: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.base,
      color: fp.text.secondary,
      fontFamily: MONO,
      fontSize: 12.5,
      lineHeight: 1.5,
      whiteSpace: 'pre-wrap',
      overflowWrap: 'anywhere',
      userSelect: 'text', // 工具输入/结果正文可选中复制
      maxHeight: 'min(50vh, 420px)',
      overflowY: 'auto',
    },
    thinkingText: {
      px: 1.25,
      py: 1,
      borderTop: `1px solid ${fp.divider}`,
      color: fp.text.muted,
      fontSize: 13,
      fontStyle: 'italic',
      lineHeight: 1.5,
      whiteSpace: 'pre-wrap',
      overflowWrap: 'anywhere',
      userSelect: 'text', // 思考正文可选中复制
      maxHeight: 'min(40vh, 320px)',
      overflowY: 'auto',
    },
    // 时间戳：卡片 header / 气泡角落的小号灰字（HH:mm:ss）。
    timestamp: {
      flexShrink: 0,
      fontSize: 11,
      color: fp.text.muted,
      fontVariantNumeric: 'tabular-nums',
      userSelect: 'none',
    },
    // —— 折叠态内容预览（首 N 行，让折叠卡不只剩标题）——
    previewWrap: {
      px: 1.25,
      pb: 0.75,
      pt: 0.25,
    },
    previewText: {
      m: 0,
      pl: 1,
      borderLeft: `2px dashed ${fp.divider}`,
      color: fp.text.muted,
      fontFamily: MONO,
      fontSize: 12,
      lineHeight: 1.5,
      whiteSpace: 'pre-wrap',
      overflowWrap: 'anywhere',
      userSelect: 'text',
    },
    moreIndicator: {
      mt: 0.5,
      pl: 1,
      fontSize: 11,
      fontStyle: 'italic',
      color: fp.text.muted,
    },
    // —— Edit 差异卡（structuredPatch）——
    diffSummaryRow: { display: 'flex', alignItems: 'center', gap: 0.75, flexShrink: 0 },
    diffStatAdd: { fontSize: 11.5, fontWeight: 700, color: fp.status.success, fontVariantNumeric: 'tabular-nums' },
    diffStatDel: { fontSize: 11.5, fontWeight: 700, color: fp.status.danger, fontVariantNumeric: 'tabular-nums' },
    diffBody: {
      m: 0,
      borderTop: `1px solid ${fp.divider}`,
      fontFamily: MONO,
      fontSize: 12,
      lineHeight: 1.55,
      userSelect: 'text',
      maxHeight: 'min(50vh, 460px)',
      overflow: 'auto',
    },
    diffLine: {
      display: 'block',
      px: 1.25,
      whiteSpace: 'pre-wrap',
      overflowWrap: 'anywhere',
      color: fp.text.secondary,
    },
    diffLineAdd: {
      backgroundColor: alpha(fp.status.success, 0.14),
      color: fp.status.success,
    },
    diffLineDel: {
      backgroundColor: alpha(fp.status.danger, 0.14),
      color: fp.status.danger,
    },
    diffTruncated: {
      px: 1.25,
      py: 0.5,
      borderTop: `1px solid ${fp.divider}`,
      fontSize: 11,
      fontStyle: 'italic',
      color: fp.text.muted,
    },
    // —— Bash 错误输出块（stderr，danger 调）——
    bashStderr: {
      m: 0,
      mt: 0.5,
      px: 1.25,
      py: 0.75,
      borderRadius: 1.5,
      border: `1px solid ${alpha(fp.status.danger, 0.4)}`,
      backgroundColor: alpha(fp.status.danger, 0.08),
      color: fp.status.danger,
      fontFamily: MONO,
      fontSize: 12,
      lineHeight: 1.5,
      whiteSpace: 'pre-wrap',
      overflowWrap: 'anywhere',
      userSelect: 'text',
      maxHeight: 'min(30vh, 240px)',
      overflowY: 'auto',
    },
    bashStderrLabel: {
      display: 'block',
      mb: 0.25,
      fontSize: 10.5,
      fontWeight: 700,
      letterSpacing: 0.4,
      textTransform: 'uppercase',
      opacity: 0.85,
    },
    // —— TodoWrite 清单卡 ——
    todoCard: {
      alignSelf: 'stretch',
      borderRadius: 2,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.elevated,
      overflow: 'hidden',
    },
    todoHeader: {
      display: 'flex',
      alignItems: 'center',
      gap: 1,
      px: 1.25,
      py: 0.75,
      color: fp.accent,
      borderBottom: `1px solid ${fp.divider}`,
    },
    todoHeaderText: { flex: 1, minWidth: 0, fontSize: 13, fontWeight: 600, color: fp.text.primary },
    todoCount: { flexShrink: 0, fontSize: 11.5, fontWeight: 600, color: fp.text.muted },
    todoList: { display: 'flex', flexDirection: 'column', px: 1.25, py: 0.75, gap: 0.5 },
    todoRow: { display: 'flex', alignItems: 'flex-start', gap: 0.75 },
    todoStatusIcon: { flexShrink: 0, mt: '1px', fontSize: 18 },
    todoStatusDone: { color: fp.status.success },
    todoStatusActive: { color: fp.accent },
    todoStatusPending: { color: fp.text.muted },
    todoBody: { flex: 1, minWidth: 0 },
    todoText: {
      fontSize: 13,
      lineHeight: 1.4,
      color: fp.text.primary,
      overflowWrap: 'anywhere',
      userSelect: 'text',
    },
    todoTextDone: { color: fp.text.muted, textDecoration: 'line-through' },
    todoActiveForm: { fontSize: 11.5, fontStyle: 'italic', color: fp.text.muted, lineHeight: 1.35 },
    // —— ExitPlanMode 计划卡 ——
    planCard: {
      alignSelf: 'stretch',
      borderRadius: 2,
      border: `1px solid ${alpha(fp.accent, 0.5)}`,
      backgroundColor: fp.bg.elevated,
      overflow: 'hidden',
    },
    planHeader: {
      display: 'flex',
      alignItems: 'center',
      gap: 1,
      px: 1.25,
      py: 0.75,
      color: fp.accent,
      borderBottom: `1px solid ${fp.divider}`,
    },
    planHeaderText: { flex: 1, minWidth: 0, fontSize: 13, fontWeight: 700, color: fp.text.primary },
    planBody: {
      m: 0,
      px: 1.25,
      py: 1,
      color: fp.text.primary,
      fontSize: 13.5,
      lineHeight: 1.55,
      whiteSpace: 'pre-wrap',
      overflowWrap: 'anywhere',
      userSelect: 'text',
      maxHeight: 'min(60vh, 520px)',
      overflowY: 'auto',
    },
    // —— AskUserQuestion 交互卡（可点选项 + 作答）——
    // accent 边框凸显「需要你操作」，与普通工具卡（divider 灰边）区分。
    askCard: {
      alignSelf: 'stretch',
      borderRadius: 2,
      border: `1px solid ${fp.accent}`,
      backgroundColor: fp.bg.elevated,
      p: 1.25,
      display: 'flex',
      flexDirection: 'column',
      gap: 1,
    },
    askCardHeader: { display: 'flex', alignItems: 'center', gap: 0.75, color: fp.accent },
    askCardHeaderText: { flex: 1, minWidth: 0, fontSize: 13, fontWeight: 700, color: fp.text.primary },
    askCardHint: { flexShrink: 0, fontSize: 11.5, fontWeight: 500, color: fp.text.muted },
    askQ: {
      display: 'flex',
      flexDirection: 'column',
      gap: 0.75,
      '&:not(:first-of-type)': { pt: 0.75, borderTop: `1px solid ${fp.divider}` },
    },
    askQHeaderRow: { display: 'flex', alignItems: 'center', gap: 0.75, flexWrap: 'wrap' },
    askChip: {
      fontSize: 11,
      fontWeight: 600,
      color: fp.accent,
      backgroundColor: fp.bg.active,
      borderRadius: 1.5,
      px: 0.75,
      py: 0.25,
      whiteSpace: 'nowrap',
    },
    askMultiHint: { fontSize: 11, color: fp.text.muted },
    askQText: {
      fontSize: 13.5,
      fontWeight: 600,
      color: fp.text.primary,
      lineHeight: 1.45,
      whiteSpace: 'pre-wrap',
      overflowWrap: 'anywhere',
      userSelect: 'text',
    },
    askOptions: { display: 'flex', flexDirection: 'column', gap: 0.75 },
    askOption: {
      display: 'flex',
      alignItems: 'flex-start',
      gap: 1,
      width: '100%',
      textAlign: 'left',
      m: 0,
      appearance: 'none',
      font: 'inherit',
      border: `1px solid ${fp.divider}`,
      borderRadius: 1.5,
      backgroundColor: fp.bg.content,
      color: fp.text.primary,
      px: 1.25,
      py: 1,
      cursor: 'pointer',
      transition: 'border-color .15s ease, background-color .15s ease',
      '&:hover': { borderColor: fp.accent, backgroundColor: fp.bg.hover },
    },
    askOptionSelected: { borderColor: fp.accent, backgroundColor: fp.bg.active },
    askOptionTick: { flexShrink: 0, mt: '1px', color: fp.text.muted },
    askOptionTickOn: { color: fp.accent },
    askOptionReadonly: {
      cursor: 'default',
      opacity: 0.7,
      '&:hover': { borderColor: fp.divider, backgroundColor: fp.bg.content },
    },
    askOptionBody: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 0.25 },
    askOptionLabel: { fontSize: 13, fontWeight: 600, lineHeight: 1.35, overflowWrap: 'anywhere' },
    askOptionDesc: {
      fontSize: 12,
      color: fp.text.secondary,
      lineHeight: 1.4,
      whiteSpace: 'pre-wrap',
      overflowWrap: 'anywhere',
    },
    askConfirmRow: { display: 'flex', justifyContent: 'flex-end', pt: 0.25 },
    askWaitingHint: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 0.5,
      fontSize: 12,
      fontStyle: 'italic',
      color: fp.text.muted,
    },
    // —— 输入条 ——
    composeBar: {
      flexShrink: 0,
      display: 'flex',
      alignItems: 'flex-end',
      gap: 1,
      p: 1,
      borderTop: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.base,
      // 安全区：底部 home indicator 不遮输入条（mindfs 同款）。
      paddingBottom: 'calc(8px + env(safe-area-inset-bottom, 0px))',
    },
    composeField: {
      flex: 1,
      '& .MuiOutlinedInput-root': {
        borderRadius: 2,
        backgroundColor: fp.bg.content,
        fontSize: 14,
      },
    },
    // —— 复制按钮（hover 显示；touch 设备无 hover → opacity 常驻 1）——
    // 折叠卡：贴在 header 内（展开图标左侧），跟随 flex 布局。
    copyBtnInline: {
      flexShrink: 0,
      p: 0.25,
      borderRadius: 1.5, // 6px，对齐方圆按钮规范
      fontSize: 16,
      color: fp.text.muted,
      opacity: 1,
      transition: 'opacity .15s ease, color .15s ease',
      '@media (hover: hover)': { opacity: 0 },
      '&:hover': { color: fp.text.primary, backgroundColor: fp.bg.hover },
    },
    // 气泡：绝对定位浮在右上角，带不透明底（盖在正文上仍清晰）。
    copyBtnFloat: {
      position: 'absolute',
      top: 4,
      right: 4,
      p: 0.5,
      borderRadius: 1.5,
      fontSize: 16,
      color: fp.text.muted,
      backgroundColor: fp.bg.elevated,
      opacity: 1,
      transition: 'opacity .15s ease, color .15s ease',
      '@media (hover: hover)': { opacity: 0 },
      '&:hover': { color: fp.text.primary, backgroundColor: fp.bg.hover },
    },
  };
};
