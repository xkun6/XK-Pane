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
  viewToggle: S;
  viewBtn: S;
  viewBtnActive: S;
  viewBtnIdle: S;
  viewBtnLabel: S;
}

export const tabBarStyles = (theme: Theme): TabBarSx => {
  const fp = theme.palette.foundation;
  const isDark = theme.palette.mode === 'dark';
  return {
    // 外壳：左侧可横向滚动的标签区 + 右侧固定视图切换。
    bar: {
      display: 'flex',
      alignItems: 'stretch',
      height: 38,
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
      gap: 0,
      px: 0.5,
      pt: 0.5,
      '&::-webkit-scrollbar': { height: 0, display: 'none' },
    },
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
      height: '100%',
      cursor: 'pointer',
      userSelect: 'none',
      fontSize: 12.5,
      fontWeight: 500,
      borderRadius: '8px 8px 0 0',
      borderRight: 'none',
      marginRight: 1,
      transition: 'background-color 140ms ease, color 140ms ease, box-shadow 140ms ease',
      '& .tab-close': { opacity: 0 },
      '&:hover .tab-close': { opacity: 1 },
    },
    tabActive: {
      backgroundColor: fp.bg.content,
      color: fp.text.primary,
      boxShadow: isDark
        ? '0 -1px 0 rgba(255,255,255,0.04) inset'
        : '0 1px 0 rgba(255,255,255,0.8) inset, 0 -1px 0 transparent',
      borderTop: `1px solid ${fp.divider}`,
      borderLeft: `1px solid ${fp.divider}`,
      borderRight: `1px solid ${fp.divider}`,
      // 底部与 content 相连，消掉下边框感
      borderBottom: `1px solid ${fp.bg.content}`,
      marginBottom: -1,
      '& .tab-close': { opacity: 1 },
    },
    tabInactive: {
      backgroundColor: 'transparent',
      color: fp.text.secondary,
      borderTop: '1px solid transparent',
      borderLeft: '1px solid transparent',
      borderRight: '1px solid transparent',
      borderBottom: '1px solid transparent',
      '&:hover': {
        backgroundColor: fp.bg.hover,
        color: fp.text.primary,
      },
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
      width: 18,
      height: 18,
      borderRadius: '5px',
      flexShrink: 0,
      color: 'inherit',
      transition: 'background-color 120ms ease, opacity 120ms ease',
      '&:hover': { backgroundColor: fp.bg.active },
    },
    viewToggle: {
      display: 'flex',
      alignItems: 'center',
      flexShrink: 0,
      gap: 0.35,
      px: 0.75,
      my: 0.5,
      mr: 0.5,
      borderRadius: '8px',
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.elevated,
    },
    viewBtn: {
      display: 'flex',
      alignItems: 'center',
      gap: 0.4,
      height: 24,
      px: 1,
      borderRadius: '6px',
      cursor: 'pointer',
      fontSize: 12,
      fontWeight: 500,
      userSelect: 'none',
      transition: 'background-color 120ms ease, color 120ms ease',
    },
    viewBtnActive: {
      backgroundColor: fp.bg.surface,
      color: fp.accent,
      boxShadow: isDark
        ? '0 1px 2px rgba(0,0,0,0.3)'
        : '0 1px 2px rgba(15,23,42,0.06)',
    },
    viewBtnIdle: {
      color: fp.text.muted,
      '&:hover': { backgroundColor: fp.bg.hover, color: fp.text.primary },
    },
    viewBtnLabel: {
      lineHeight: 1,
      whiteSpace: 'nowrap',
    },
  };
};
