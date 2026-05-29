import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface TerminalPageSx {
  root: S;
  toolbar: S;
  openBtn: S;
  body: S;
}

export const terminalPageStyles = (theme: Theme): TerminalPageSx => {
  const fp = theme.palette.foundation;
  return {
    root: {
      flex: 1,
      minHeight: 0,
      display: 'flex',
      flexDirection: 'column',
      backgroundColor: fp.bg.content,
      overflow: 'hidden',
    },
    toolbar: {
      display: 'flex',
      alignItems: 'center',
      gap: 1,
      px: 1.5,
      height: 40,
      flexShrink: 0,
      borderBottom: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.sidebar,
    },
    openBtn: {
      textTransform: 'none',
      borderRadius: 1.5,
    },
    body: {
      flex: 1,
      minHeight: 0,
      position: 'relative',
    },
  };
};
