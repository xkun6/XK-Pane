import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

// 用 SystemStyleObject（纯样式对象，非 SxProps 联合）：既让 `sx={[styles.tab, ...]}` 数组形式的
// 每个元素都是合法元素类型，又避免 Record 索引在 noUncheckedIndexedAccess 下带出的 `| undefined`。
type S = SystemStyleObject<Theme>;
export interface TabBarSx {
  bar: S;
  items: S;
  dragWrap: S;
  tab: S;
  tabActive: S;
  tabInactive: S;
  pinIcon: S;
  title: S;
  closeBtn: S;
  addBtn: S;
}

export const tabBarStyles = (theme: Theme): TabBarSx => {
  const fp = theme.palette.foundation;
  return {
    // 外壳：左侧可横向滚动的标签区 + 右侧固定「新建」按钮。
    bar: {
      display: 'flex',
      alignItems: 'stretch',
      height: 36,
      minWidth: 0,
      backgroundColor: fp.bg.sidebar,
      borderBottom: `1px solid ${fp.divider}`,
    },
    items: {
      flex: '1 1 auto',
      minWidth: 0,
      display: 'flex',
      alignItems: 'stretch',
      overflowX: 'auto',
      overflowY: 'hidden',
      '&::-webkit-scrollbar': { height: 0, display: 'none' },
    },
    // 拖拽外层包裹：@dnd-kit sortable 节点（承载 transform），不参与收缩，内部嵌真实 tab。
    dragWrap: {
      display: 'flex',
      alignItems: 'stretch',
      flexShrink: 0,
    },
    tab: {
      display: 'flex',
      alignItems: 'center',
      gap: 0.75,
      px: 1.5,
      maxWidth: 220,
      minWidth: 120,
      cursor: 'pointer',
      userSelect: 'none',
      fontSize: 13,
      fontWeight: 500,
      borderRight: `1px solid ${fp.divider}`,
      transition: 'background-color 120ms ease, color 120ms ease',
      '& .tab-close': { opacity: 0 },
      '&:hover .tab-close': { opacity: 1 },
    },
    tabActive: {
      backgroundColor: fp.bg.content,
      color: fp.text.primary,
      '& .tab-close': { opacity: 1 },
    },
    tabInactive: {
      backgroundColor: 'transparent',
      color: fp.text.secondary,
      '&:hover': { backgroundColor: fp.bg.hover, color: fp.text.primary },
    },
    pinIcon: {
      fontSize: 13,
      flexShrink: 0,
      color: fp.accent,
    },
    title: {
      flex: 1,
      minWidth: 0,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    closeBtn: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      width: 20,
      height: 20,
      borderRadius: 1,
      flexShrink: 0,
      color: 'inherit',
      transition: 'background-color 120ms ease, opacity 120ms ease',
      '&:hover': { backgroundColor: fp.bg.active },
    },
    addBtn: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      width: 34,
      flexShrink: 0,
      color: fp.text.muted,
      cursor: 'pointer',
      borderLeft: `1px solid ${fp.divider}`,
      transition: 'background-color 120ms ease, color 120ms ease',
      '&:hover': { backgroundColor: fp.bg.hover, color: fp.text.primary },
    },
  };
};
