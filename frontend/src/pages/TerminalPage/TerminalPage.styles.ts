import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface TerminalPageSx {
  root: S;
  body: S;
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
  };
};
