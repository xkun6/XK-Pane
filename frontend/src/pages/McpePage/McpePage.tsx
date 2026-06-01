import { Box, IconButton, Tooltip, Typography, useTheme } from '@mui/material';
import VideogameAssetRoundedIcon from '@mui/icons-material/VideogameAssetRounded';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import Inventory2RoundedIcon from '@mui/icons-material/Inventory2Rounded';
import { useT } from '@/i18n';
import { isWindows } from '@/shared/platform';
import { isDesktop } from '@/services/transport';
import { mcpeStyles } from './McpePage.styles';
import { McpeProvider } from './McpeContext';
import { useMcpePage } from './useMcpePage';
import { PackageList } from './PackageList';
import { PackageDetail } from './PackageDetail';

const McpePageBody = () => {
  const theme = useTheme();
  const styles = mcpeStyles(theme);
  const t = useT();
  const vm = useMcpePage();
  const canCreate = !vm.busy && isWindows && isDesktop;

  return (
    <Box sx={styles.root}>
      <Box sx={styles.header}>
        <Box sx={styles.headerTitle}>
          <VideogameAssetRoundedIcon fontSize="small" />
          <span>{t('mcpe.title')}</span>
        </Box>
        <Box sx={styles.headerActions}>
          <Tooltip title={t('mcpe.action.refresh')}>
            <IconButton size="small" aria-label={t('mcpe.action.refresh')} onClick={() => void vm.refresh(t('mcpe.error.title'))}>
              <RefreshRoundedIcon fontSize="small" />
            </IconButton>
          </Tooltip>
          <Tooltip title={t('mcpe.action.newPackage')}>
            <span>
              <IconButton
                size="small"
                aria-label={t('mcpe.action.newPackage')}
                disabled={!canCreate}
                onClick={() => void vm.create(t('mcpe.dialog.pickProject'), t('mcpe.error.title'))}
              >
                <AddRoundedIcon fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
        </Box>
      </Box>

      {!isWindows && <Box sx={styles.platformWarn}>{t('mcpe.platformWarn')}</Box>}

      <Box sx={styles.listContainer}>
        <PackageList />
      </Box>

      <Box sx={styles.detail}>
        {vm.activePackageId ? (
          <PackageDetail packageId={vm.activePackageId} />
        ) : (
          <Box sx={styles.emptyState}>
            {vm.packages.length === 0 ? (
              <>
                <Inventory2RoundedIcon sx={{ fontSize: 40, opacity: 0.4 }} />
                <Typography sx={{ fontSize: 12 }}>{t('mcpe.empty.noPackages')}</Typography>
                <Typography sx={{ fontSize: 11 }}>{t('mcpe.empty.hint')}</Typography>
              </>
            ) : (
              <Typography sx={{ fontSize: 12 }}>{t('mcpe.empty.selectOne')}</Typography>
            )}
          </Box>
        )}
      </Box>
    </Box>
  );
};

// 页面入口：包一层状态 Provider（页面 keepAlive=true，首次进入后状态/事件订阅常驻）。
export const McpePage = () => (
  <McpeProvider>
    <McpePageBody />
  </McpeProvider>
);
