import { Box, useTheme } from '@mui/material';
import { Skeleton } from '@/components/Skeleton';
import { sshPageStyles } from './SshPage.styles';

// SSH 页骨架屏：复刻标题头 + 工具栏 + 机器行的版式节奏（不消费 i18n）。
export const SshPageSkeleton = () => {
  const theme = useTheme();
  const styles = sshPageStyles(theme);

  return (
    <Box sx={styles.root}>
      <Box sx={styles.body}>
        <Box sx={styles.header}>
          <Skeleton variant="rect" width="30%" height={26} sx={{ borderRadius: 1 }} />
          <Skeleton variant="text" width="70%" height={14} />
        </Box>
        <Box sx={styles.toolbar}>
          <Skeleton variant="rect" width={120} height={32} sx={{ borderRadius: 1.5 }} />
        </Box>
        <Box sx={styles.list}>
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} variant="rect" height={64} sx={{ borderRadius: 1 }} />
          ))}
        </Box>
      </Box>
    </Box>
  );
};
