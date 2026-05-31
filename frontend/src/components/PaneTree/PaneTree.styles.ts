import { alpha, type Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';
import type { PaneEdge } from '@/terminal';

type S = SystemStyleObject<Theme>;

// 显式 SystemStyleObject 接口（非 Record）：tabPane 走 sx 数组形式合并 active/hidden，规避
// noUncheckedIndexedAccess 的 `|undefined` 与 SxProps 联合含数组的坑（同 TabBar）。
interface PanelViewSx {
  panel: S;
  content: S;
  tabPane: S;
  tabPaneActive: S;
  tabPaneHidden: S;
  empty: S;
  emptyIcon: S;
  emptyTitle: S;
  emptyHint: S;
}

export const panelViewStyles = (theme: Theme): PanelViewSx => {
  const fp = theme.palette.foundation;
  return {
    panel: {
      display: 'flex',
      flexDirection: 'column',
      height: '100%',
      width: '100%',
      minHeight: 0,
      minWidth: 0,
      backgroundColor: fp.bg.content,
    },
    content: {
      flex: 1,
      minHeight: 0,
      position: 'relative',
      overflow: 'hidden',
    },
    // 每个 Tab 的内容铺满 content；活动显示、其余 display:none（隐藏时 xterm host 0 尺寸，
    // layoutScheduler 自动延后 fit，切回可见时 active-effect flush）。保持挂载以保留终端状态。
    tabPane: {
      position: 'absolute',
      inset: 0,
    },
    tabPaneActive: { display: 'block' },
    tabPaneHidden: { display: 'none' },
    // 空面板（无 Tab）占位：居中提示从左侧启动会话。
    empty: {
      flex: 1,
      minHeight: 0,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 1,
      px: 3,
      textAlign: 'center',
    },
    emptyIcon: { fontSize: 40, color: fp.text.muted, opacity: 0.6 },
    emptyTitle: { fontSize: 14, fontWeight: 600, color: fp.text.secondary },
    emptyHint: { fontSize: 12, color: fp.text.muted, maxWidth: 280, lineHeight: 1.6 },
  };
};

// 拖拽 Tab 时跟随光标的浮层预览（@dnd-kit DragOverlay 内容）。
export const dragOverlayStyles = (theme: Theme): { overlay: S } => {
  const fp = theme.palette.foundation;
  return {
    overlay: {
      display: 'flex',
      alignItems: 'center',
      height: 30,
      px: 1.5,
      maxWidth: 220,
      borderRadius: 1,
      fontSize: 13,
      fontWeight: 500,
      cursor: 'grabbing',
      color: fp.text.primary,
      backgroundColor: fp.bg.elevated,
      border: `1px solid ${fp.accent}`,
      boxShadow: theme.shadows[6],
      pointerEvents: 'none',
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
  };
};

// 面板内容区的四条边放置区（拖 Tab 到某边 → 在该面板旁拆出新面板，VSCode 式十字布局）。
// 左/右各占侧边 25% 满高；上/下占中间 50% 宽 × 25% 高（避开左右区，角落归左右区，无重叠）。
// 中心 50%×50% 无 zone → 落到 closestCenter→合并。pointerEvents:none 平时不挡终端，
// 拖拽时 @dnd-kit 按已注册 rect 命中、isOver 才显高亮。
type EdgeDropSx = Record<PaneEdge, S> & { zoneActive: S };

export const edgeDropStyles = (theme: Theme): EdgeDropSx => {
  const fp = theme.palette.foundation;
  const common: S = {
    position: 'absolute',
    pointerEvents: 'none',
    zIndex: 5,
    backgroundColor: 'transparent',
    transition: 'background-color 120ms ease, box-shadow 120ms ease',
  };
  return {
    left: { ...common, top: 0, left: 0, bottom: 0, width: '25%' },
    right: { ...common, top: 0, right: 0, bottom: 0, width: '25%' },
    top: { ...common, top: 0, left: '25%', right: '25%', height: '25%' },
    bottom: { ...common, bottom: 0, left: '25%', right: '25%', height: '25%' },
    zoneActive: {
      backgroundColor: alpha(fp.accent, 0.18),
      boxShadow: `inset 0 0 0 2px ${fp.accent}`,
    },
  };
};
