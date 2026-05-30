import {
  Box,
  Button,
  Chip,
  IconButton,
  MenuItem,
  TextField,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import CameraAltRoundedIcon from '@mui/icons-material/CameraAltRounded';
import RestoreRoundedIcon from '@mui/icons-material/RestoreRounded';
import LabelRoundedIcon from '@mui/icons-material/LabelRounded';
import { useT } from '@/i18n';
import type { RecentChange } from '@/services';
import { historyPageStyles } from './HistoryPage.styles';
import { useHistoryPage } from './useHistoryPage';

// HistoryPage：选已登记项目 → 浏览 Local History 最近变更 + 手动快照 + 回滚。
// 端到端验证 localhistorysvc（SnapshotFile / GetRecentChanges / RestoreVersion）。
// diff 查看为后续单元。

// 把 RFC3339 时间戳格式化为本地可读串（数据展示，非 UI 文案，不走 t()）。
function formatTime(ts: string): string {
  if (!ts) return '';
  const d = new Date(ts);
  return Number.isNaN(d.getTime()) ? ts : d.toLocaleString();
}

export const HistoryPage = () => {
  const theme = useTheme();
  const styles = historyPageStyles(theme);
  const t = useT();
  const vm = useHistoryPage();

  const errorTitle = t('history.error.title');
  const disabled = vm.busy || vm.loading;

  const onSnapshot = (): void =>
    void vm.snapshot({
      successTitle: t('history.snapshot.title'),
      createdMessage: t('history.snapshot.created'),
      noChangeMessage: t('history.snapshot.noChange'),
      errorTitle,
    });

  const onRestore = (c: RecentChange): void =>
    void vm.restore(c, {
      confirmTitle: t('history.restoreConfirm.title'),
      confirmMessage: t('history.restoreConfirm.message', { path: c.filePath }),
      okLabel: t('history.restoreConfirm.ok'),
      cancelLabel: t('history.restoreConfirm.cancel'),
      successTitle: t('history.restored.title'),
      successMessage: t('history.restored.message'),
      errorTitle,
    });

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
                  disabled={!vm.selectedPath || disabled}
                >
                  <RefreshRoundedIcon />
                </IconButton>
              </span>
            </Tooltip>
          </Box>

          {vm.selectedPath && (
            <>
              <Box sx={styles.snapshotRow}>
                <TextField
                  size="small"
                  fullWidth
                  placeholder={t('history.snapshotPlaceholder')}
                  value={vm.newFilePath}
                  onChange={(e) => vm.setNewFilePath(e.target.value)}
                  disabled={disabled}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') onSnapshot();
                  }}
                />
                <Button
                  size="small"
                  variant="outlined"
                  startIcon={<CameraAltRoundedIcon />}
                  onClick={onSnapshot}
                  disabled={disabled || !vm.newFilePath.trim()}
                  sx={{ textTransform: 'none', whiteSpace: 'nowrap' }}
                >
                  {t('history.snapshotAction')}
                </Button>
              </Box>

              <Typography sx={styles.sectionTitle}>
                {t('history.recent', { count: String(vm.changes.length) })}
              </Typography>
              {!vm.loading && vm.changes.length === 0 ? (
                <Box sx={styles.empty}>{t('history.empty')}</Box>
              ) : (
                <Box sx={styles.list}>
                  {vm.changes.map((c) => (
                    <Box key={`${c.filePath}:${c.versionId}`} sx={styles.row}>
                      <Box sx={styles.rowMain}>
                        <Typography sx={styles.filePath} title={c.filePath}>
                          {c.filePath}
                        </Typography>
                        <Typography sx={styles.timestamp}>{formatTime(c.timestamp)}</Typography>
                      </Box>
                      {c.labelName && (
                        <Chip
                          icon={<LabelRoundedIcon />}
                          label={c.labelName}
                          size="small"
                          variant="outlined"
                          sx={{ borderRadius: 6 }}
                        />
                      )}
                      {c.branch && (
                        <Chip
                          label={c.branch}
                          size="small"
                          variant="outlined"
                          sx={{ borderRadius: 6, fontFamily: 'monospace' }}
                        />
                      )}
                      <Tooltip title={t('history.restore')}>
                        <span>
                          <IconButton
                            size="small"
                            aria-label={t('history.restore')}
                            onClick={() => onRestore(c)}
                            disabled={disabled}
                          >
                            <RestoreRoundedIcon fontSize="small" />
                          </IconButton>
                        </span>
                      </Tooltip>
                    </Box>
                  ))}
                </Box>
              )}
            </>
          )}
        </>
      )}
    </Box>
  );
};
