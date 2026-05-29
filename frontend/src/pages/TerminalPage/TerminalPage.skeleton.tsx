import { Box, useTheme } from '@mui/material';

// 终端页骨架：版式占位（工具条 + 主体），不出文字（骨架不消费 t()）。
export const TerminalPageSkeleton = () => {
  const theme = useTheme();
  const fp = theme.palette.foundation;
  return (
    <Box sx={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column', backgroundColor: fp.bg.content }}>
      <Box sx={{ height: 40, flexShrink: 0, borderBottom: `1px solid ${fp.divider}`, backgroundColor: fp.bg.sidebar }} />
      <Box sx={{ flex: 1, minHeight: 0 }} />
    </Box>
  );
};
