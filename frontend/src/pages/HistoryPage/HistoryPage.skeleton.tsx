import { Box, useTheme } from '@mui/material';
import { Skeleton } from '@/components/Skeleton';
import { historyPageStyles } from './HistoryPage.styles';

// HistoryPage 骨架屏：复刻标题头 + 项目下拉 + 文件列表的版式节奏。不消费 i18n（占位无文字）。
export const HistoryPageSkeleton = () => {
  const theme = useTheme();
  const styles = historyPageStyles(theme);

  return (
    <Box sx={styles.root}>
      <Box sx={styles.header}>
        <Skeleton variant="text" width={80} height={12} />
        <Skeleton variant="rect" width="35%" height={28} sx={{ borderRadius: 1 }} />
      </Box>
      <Box sx={styles.pickerRow}>
        <Skeleton variant="rect" height={40} sx={{ flex: 1, borderRadius: 8 }} />
        <Skeleton variant="circle" width={36} height={36} />
      </Box>
      <Box sx={styles.fileList}>
        {[0, 1, 2, 3, 4].map((i) => (
          <Skeleton key={i} variant="rect" height={44} sx={{ borderRadius: 8 }} />
        ))}
      </Box>
    </Box>
  );
};
