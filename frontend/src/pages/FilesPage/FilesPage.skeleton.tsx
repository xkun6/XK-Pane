import { Box, useTheme } from '@mui/material';
import { Skeleton } from '@/components/Skeleton';
import { filesPageStyles } from './FilesPage.styles';

// 版式占位：左目录树骨架 + 右编辑器骨架（lazy 加载 Monaco chunk 期间显示）。
export const FilesPageSkeleton = () => {
  const theme = useTheme();
  const styles = filesPageStyles(theme);
  return (
    <Box sx={styles.root}>
      <Box sx={styles.left}>
        <Box sx={styles.leftHeader}>
          <Skeleton variant="text" width="55%" height={16} />
        </Box>
        <Box sx={{ p: 1, display: 'flex', flexDirection: 'column', gap: 0.75 }}>
          {Array.from({ length: 12 }).map((_, i) => (
            <Skeleton key={i} variant="rect" width={`${72 - (i % 5) * 8}%`} height={18} />
          ))}
        </Box>
      </Box>
      <Box sx={styles.right}>
        <Box sx={styles.toolbar}>
          <Skeleton variant="text" width={180} height={16} />
        </Box>
        <Box sx={{ flex: 1, p: 2 }}>
          <Skeleton variant="rect" width="100%" height="92%" />
        </Box>
      </Box>
    </Box>
  );
};
