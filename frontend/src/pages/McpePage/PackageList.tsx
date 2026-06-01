import { Box, IconButton, Tooltip, useTheme } from '@mui/material';
import Inventory2RoundedIcon from '@mui/icons-material/Inventory2Rounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import { useT } from '@/i18n';
import { NativeDialogs } from '@/services';
import { mcpeStyles } from './McpePage.styles';
import { useMcpe, isPackageRunning } from './McpeContext';
import { messageOf } from './useMcpePage';

// 包列表：行显示图标/名称/项目路径/RUN 徽标 + hover 显隐的删除按钮。
export const PackageList = () => {
  const theme = useTheme();
  const styles = mcpeStyles(theme);
  const t = useT();
  const { packages, activePackageId, runs, setActivePackage, deletePackage } = useMcpe();

  const handleDelete = async (id: string, name: string) => {
    const ok = await NativeDialogs.confirm({
      title: t('mcpe.delete.title'),
      message: t('mcpe.delete.message', { name }),
      okLabel: t('mcpe.delete.ok'),
      cancelLabel: t('mcpe.delete.cancel'),
    });
    if (!ok) return;
    try {
      await deletePackage(id);
    } catch (e) {
      await NativeDialogs.error(t('mcpe.error.title'), messageOf(e));
    }
  };

  if (packages.length === 0) {
    return <Box sx={styles.listEmpty}>{t('mcpe.list.empty')}</Box>;
  }

  return (
    <Box>
      {packages.map((pkg) => {
        const active = pkg.id === activePackageId;
        const running = isPackageRunning(runs, pkg.id);
        return (
          <Box
            key={pkg.id}
            sx={{ ...styles.row, ...(active ? styles.rowActive : {}) }}
            onClick={() => setActivePackage(pkg.id)}
          >
            <Inventory2RoundedIcon sx={{ fontSize: 14, mr: 1, flexShrink: 0 }} />
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Box sx={styles.rowName}>{pkg.name}</Box>
              <Box sx={styles.rowPath}>{pkg.projectPath}</Box>
            </Box>
            {running && (
              <Box component="span" sx={styles.runBadge}>
                ● {t('mcpe.list.running')}
              </Box>
            )}
            <Tooltip title={t('mcpe.action.delete')}>
              <IconButton
                className="mcpe-row-del"
                size="small"
                sx={styles.rowDel}
                aria-label={t('mcpe.action.delete')}
                onClick={(e) => {
                  e.stopPropagation();
                  void handleDelete(pkg.id, pkg.name);
                }}
              >
                <DeleteOutlineRoundedIcon sx={{ fontSize: 14 }} />
              </IconButton>
            </Tooltip>
          </Box>
        );
      })}
    </Box>
  );
};
