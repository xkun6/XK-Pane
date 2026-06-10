import type { SxProps, Theme } from '@mui/material';

export const composerStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  return {
    // 输入条（自 SessionTimeline composeBar 抽出）：贴卡片流底部，安全区垫高由下方
    // StatusLine 承担。
    root: {
      flexShrink: 0,
      display: 'flex',
      alignItems: 'flex-end',
      gap: 1,
      p: 1,
      borderTop: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.base,
    },
    field: {
      flex: 1,
      '& .MuiOutlinedInput-root': {
        borderRadius: 2,
        backgroundColor: fp.bg.content,
        fontSize: 14,
      },
    },
  };
};
