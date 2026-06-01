import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface TerminalPageSx {
  root: S;
  body: S;
  mobileRoot: S;
  mobileBar: S;
  mobileBarLabel: S;
  drawerPaper: S;
}

export const terminalPageStyles = (theme: Theme): TerminalPageSx => {
  const fp = theme.palette.foundation;
  return {
    root: {
      flex: 1,
      minHeight: 0,
      display: 'flex',
      flexDirection: 'row', // 左：项目面板（固定宽）｜右：PaneTree（flex:1）
      backgroundColor: fp.bg.content,
      overflow: 'hidden',
    },
    body: {
      flex: 1,
      minWidth: 0,
      minHeight: 0,
      position: 'relative',
    },
    // 手机/窄屏（down('sm')）：工作区栏收进抽屉，主体铺满 → 终端/对话拿到全宽。
    mobileRoot: {
      flex: 1,
      minHeight: 0,
      display: 'flex',
      flexDirection: 'column',
      backgroundColor: fp.bg.content,
      overflow: 'hidden',
    },
    mobileBar: {
      flexShrink: 0,
      minHeight: 40,
      display: 'flex',
      alignItems: 'center',
      gap: 0.5,
      px: 1,
      borderBottom: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.base,
      color: fp.text.secondary,
    },
    mobileBarLabel: {
      fontSize: 13,
      fontWeight: 600,
      color: fp.text.secondary,
    },
    // 抽屉宽度对齐工作区栏 272；窄屏再缩到 85vw（极窄机）。
    drawerPaper: {
      width: 272,
      maxWidth: '85vw',
      display: 'flex',
    },
  };
};
