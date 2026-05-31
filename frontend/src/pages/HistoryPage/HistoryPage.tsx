import { Box, Chip, IconButton, MenuItem, TextField, Tooltip, Typography, useTheme } from '@mui/material';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import DescriptionRoundedIcon from '@mui/icons-material/DescriptionRounded';
import ArrowBackRoundedIcon from '@mui/icons-material/ArrowBackRounded';
import SellRoundedIcon from '@mui/icons-material/SellRounded';
import { useT } from '@/i18n';
import { historyPageStyles } from './HistoryPage.styles';
import { useHistoryPage } from './useHistoryPage';
import { formatSize, formatTime } from './historyFormat';

// HistoryPage：选一个已登记项目 → 两阶段浏览本地历史。
//   阶段 1（无选中文件）：最近变更文件列表（自动捕获 + 手动快照产生的版本聚合）。
//   阶段 2（选中文件后）：版本时间线 + 4-tab 预览（diff/content/deleted/项目快照）—— P4c-2/3。

export const HistoryPage = () => {
  const theme = useTheme();
  const styles = historyPageStyles(theme);
  const t = useT();
  const vm = useHistoryPage();

  const errorTitle = t('history.error.title');

  return (
    <Box sx={styles.root}>
      <Box sx={styles.header}>
        <Typography sx={styles.eyebrow}>{t('history.eyebrow')}</Typography>
        <Typography sx={styles.title}>{t('history.title')}</Typography>
      </Box>

      {vm.projects.length === 0 ? (
        <Box sx={styles.empty}>{t('history.noProjects')}</Box>
      ) : (
        <>
          <Box sx={styles.pickerRow}>
            <TextField
              select
              size="small"
              fullWidth
              label={t('history.project')}
              value={vm.selectedPath}
              onChange={(e) => void vm.select(e.target.value, errorTitle)}
            >
              {vm.projects.map((p) => (
                <MenuItem key={p.id} value={p.path}>
                  {p.alias ?? p.name}
                </MenuItem>
              ))}
            </TextField>
            <Tooltip title={t('history.refresh')}>
              <span>
                <IconButton
                  aria-label={t('history.refresh')}
                  onClick={() => void vm.refresh(errorTitle)}
                  disabled={!vm.selectedPath || vm.loading}
                >
                  <RefreshRoundedIcon />
                </IconButton>
              </span>
            </Tooltip>
          </Box>

          {vm.loading && <Box sx={styles.empty}>{t('history.fileList.loading')}</Box>}

          {/* 阶段 1：文件列表 */}
          {!vm.loading && vm.selectedPath && !vm.selectedFile && (
            <>
              <Typography sx={styles.hint}>{t('history.fileList.autoTrack')}</Typography>
              {vm.files.length === 0 ? (
                <Box sx={styles.empty}>{t('history.fileList.empty')}</Box>
              ) : (
                <Box sx={styles.fileList}>
                  {vm.files.map((f) => (
                    <Box key={f.filePath} sx={styles.fileRow} onClick={() => vm.selectFile(f.filePath)}>
                      <DescriptionRoundedIcon fontSize="small" sx={styles.fileIcon} />
                      <Box sx={styles.fileMain}>
                        <Typography sx={styles.fileName} title={f.filePath}>
                          {f.fileName}
                        </Typography>
                        {f.dir && <Typography sx={styles.fileDir}>{f.dir}</Typography>}
                      </Box>
                      <Box sx={styles.fileMeta}>
                        {f.labelName && (
                          <Chip
                            icon={<SellRoundedIcon />}
                            label={f.labelName}
                            size="small"
                            variant="outlined"
                            sx={{ borderRadius: 6, color: theme.palette.foundation.accent, borderColor: theme.palette.foundation.accent }}
                          />
                        )}
                        {f.branch && (
                          <Chip
                            label={f.branch}
                            size="small"
                            variant="outlined"
                            sx={{ borderRadius: 6, fontFamily: 'monospace' }}
                          />
                        )}
                        <Box component="span">{formatSize(f.size)}</Box>
                        <Box component="span">{formatTime(f.timestamp)}</Box>
                      </Box>
                    </Box>
                  ))}
                </Box>
              )}
            </>
          )}

          {/* 阶段 2：版本时间线 + 4-tab 预览（P4c-2/3 填充） */}
          {!vm.loading && vm.selectedFile && (
            <>
              <Box sx={styles.stageHeader}>
                <Tooltip title={t('history.backToList')}>
                  <IconButton size="small" aria-label={t('history.backToList')} onClick={vm.backToList}>
                    <ArrowBackRoundedIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Typography sx={styles.stagePath} title={vm.selectedFile}>
                  {vm.selectedFile}
                </Typography>
              </Box>
              <Box sx={styles.empty}>{t('history.timeline.selectVersion')}</Box>
            </>
          )}
        </>
      )}
    </Box>
  );
};
