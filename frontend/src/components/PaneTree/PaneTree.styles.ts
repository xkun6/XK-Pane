import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;

// 显式 SystemStyleObject 接口（非 Record）：tabPane 走 sx 数组形式合并 active/hidden，规避
// noUncheckedIndexedAccess 的 `|undefined` 与 SxProps 联合含数组的坑（同 TabBar）。
interface PanelViewSx {
  panel: S;
  content: S;
  tabPane: S;
  tabPaneActive: S;
  tabPaneHidden: S;
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
  };
};
