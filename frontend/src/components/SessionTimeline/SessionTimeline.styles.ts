import type { SxProps, Theme } from '@mui/material';

// 等宽字体栈（工具输入/结果用，与终端观感一致；不依赖 xterm 字体）。
const MONO = 'ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace';

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
