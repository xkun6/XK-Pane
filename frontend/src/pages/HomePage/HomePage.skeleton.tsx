import { Box, useTheme } from '@mui/material';
import { Skeleton } from '@/components/Skeleton';
import { homePageStyles } from './HomePage.styles';

// HomePage 仪表盘骨架屏：复刻问候头 + 快捷操作网格 + 最近启动列表的版式节奏。
// 不消费 i18n（加载占位无文字）。
export const HomePageSkeleton = () => {
  const theme = useTheme();
  const styles = homePageStyles(theme);

  return (
    <Box sx={styles.root}>
      <Box sx={styles.body}>
        {/* 问候头 */}
        <Box sx={styles.section}>
          <Skeleton variant="rect" width="40%" height={32} sx={{ borderRadius: 1 }} />
          <Skeleton variant="text" width="60%" height={15} />
        </Box>

        {/* 快捷操作网格 */}
        <Box sx={styles.section}>
          <Skeleton variant="text" width={100} height={12} />
          <Box sx={styles.actionsGrid}>
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} variant="rect" height={56} sx={{ borderRadius: 8 }} />
            ))}
          </Box>
        </Box>

        {/* 最近启动 */}
        <Box sx={styles.section}>
          <Skeleton variant="text" width={100} height={12} />
          <Box sx={styles.recentList}>
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} variant="rect" height={44} sx={{ borderRadius: 8 }} />
            ))}
          </Box>
        </Box>
      </Box>
    </Box>
  );
};
