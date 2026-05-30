import {
  Box,
  Button,
  Chip,
  FormControlLabel,
  IconButton,
  MenuItem,
  Switch,
  TextField,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import CameraAltRoundedIcon from '@mui/icons-material/CameraAltRounded';
import RestoreRoundedIcon from '@mui/icons-material/RestoreRounded';
import LabelRoundedIcon from '@mui/icons-material/LabelRounded';
import SaveRoundedIcon from '@mui/icons-material/SaveRounded';
import CleaningServicesRoundedIcon from '@mui/icons-material/CleaningServicesRounded';
import { useT } from '@/i18n';
import type { RecentChange } from '@/services';
import { DiffView } from '@/components/DiffView';
import { historyPageStyles } from './HistoryPage.styles';
import { useHistoryPage } from './useHistoryPage';

// HistoryPage：选已登记项目 → 浏览 Local History 最近变更 + 手动快照 + 回滚 + diff 查看，
// 以及每项目历史配置编辑 + 存储统计 + 立即清理。端到端验证 localhistorysvc 全部暴露面。

const MB = 1024 * 1024;

// 把 RFC3339 时间戳格式化为本地可读串（数据展示，非 UI 文案，不走 t()）。
function formatTime(ts: string): string {
  if (!ts) return '';
  const d = new Date(ts);
  return Number.isNaN(d.getTime()) ? ts : d.toLocaleString();
}

// 字节数 → 人类可读（B/KB/MB/GB 为国际单位符号，非翻译文案，同数据存储页风格）。
function formatBytes(n: number): string {
  if (n <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1);
  const v = n / Math.pow(1024, i);
  return `${i === 0 ? v : v.toFixed(1)} ${units[i] ?? 'B'}`;
}

// 受控数字输入解析：非法 / 负数归一为 0。
function toInt(v: string): number {
  const n = parseInt(v, 10);
  return Number.isNaN(n) ? 0 : Math.max(0, n);
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

  const diffLabels = {
    binary: t('history.diff.binary'),
    truncated: t('history.diff.truncated'),
    identical: t('history.diff.identical'),
    added: t('history.diff.added'),
    removed: t('history.diff.removed'),
  };
  const selectedChange = vm.changes.find((c) => c.versionId === vm.selectedVersionId) ?? null;

  const onSaveConfig = (): void =>
    void vm.saveConfig({
      savedTitle: t('history.config.saved.title'),
      savedMessage: t('history.config.saved.message'),
      errorTitle,
    });

  const onCleanup = (): void =>
    void vm.runCleanup({
      confirmTitle: t('history.cleanup.confirm.title'),
      confirmMessage: t('history.cleanup.confirm.message'),
      okLabel: t('history.cleanup.confirm.ok'),
      cancelLabel: t('history.cleanup.confirm.cancel'),
      doneTitle: t('history.cleanup.done.title'),
      doneMessage: t('history.cleanup.done.message'),
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
                    <Box
                      key={`${c.filePath}:${c.versionId}`}
                      sx={c.versionId === vm.selectedVersionId ? { ...styles.row, ...styles.rowSelected } : styles.row}
                    >
                      <Box
                        sx={styles.rowMain}
                        role="button"
                        tabIndex={0}
                        onClick={() => void vm.viewDiff(c, errorTitle)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') void vm.viewDiff(c, errorTitle);
                        }}
                      >
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

              {vm.selectedVersionId && (
                <Box sx={styles.diffSection}>
                  <Typography sx={styles.diffTitle}>
                    {t('history.diff.title', { path: selectedChange?.filePath ?? '' })}
                  </Typography>
                  {vm.diffLoading ? (
                    <Box sx={styles.empty}>{t('history.diff.loading')}</Box>
                  ) : (
                    vm.diff && <DiffView diff={vm.diff} labels={diffLabels} />
                  )}
                </Box>
              )}

              {vm.config && (
                <Box sx={styles.configSection}>
                  <Typography sx={styles.sectionTitle}>{t('history.config.title')}</Typography>

                  {vm.stats && (
                    <Box sx={styles.statsRow}>
                      <Chip
                        size="small"
                        variant="outlined"
                        sx={{ borderRadius: 6 }}
                        label={t('history.stats.summary', {
                          versions: String(vm.stats.versionCount),
                          files: String(vm.stats.fileCount),
                          size: formatBytes(vm.stats.totalBlobSize),
                        })}
                      />
                      <Chip
                        size="small"
                        variant="outlined"
                        sx={{ borderRadius: 6 }}
                        label={t('history.stats.limit', { limit: formatBytes(vm.config.maxTotalSize) })}
                      />
                    </Box>
                  )}

                  <FormControlLabel
                    control={
                      <Switch
                        checked={vm.config.enabled}
                        onChange={(e) => vm.patchConfig({ enabled: e.target.checked })}
                        disabled={vm.configBusy}
                      />
                    }
                    label={t('history.config.enabled')}
                  />

                  <Box sx={styles.configGrid}>
                    <TextField
                      type="number"
                      size="small"
                      label={t('history.config.maxVersionsPerFile')}
                      value={vm.config.maxVersionsPerFile}
                      onChange={(e) => vm.patchConfig({ maxVersionsPerFile: toInt(e.target.value) })}
                      disabled={vm.configBusy}
                    />
                    <TextField
                      type="number"
                      size="small"
                      label={t('history.config.maxAgeDays')}
                      value={vm.config.maxAgeDays}
                      onChange={(e) => vm.patchConfig({ maxAgeDays: toInt(e.target.value) })}
                      disabled={vm.configBusy}
                    />
                    <TextField
                      type="number"
                      size="small"
                      label={t('history.config.maxFileSizeMB')}
                      value={Math.round(vm.config.maxFileSize / MB)}
                      onChange={(e) => vm.patchConfig({ maxFileSize: toInt(e.target.value) * MB })}
                      disabled={vm.configBusy}
                    />
                    <TextField
                      type="number"
                      size="small"
                      label={t('history.config.maxTotalSizeMB')}
                      value={Math.round(vm.config.maxTotalSize / MB)}
                      onChange={(e) => vm.patchConfig({ maxTotalSize: toInt(e.target.value) * MB })}
                      disabled={vm.configBusy}
                    />
                    <TextField
                      type="number"
                      size="small"
                      label={t('history.config.minSaveIntervalSecs')}
                      value={vm.config.minSaveIntervalSecs}
                      onChange={(e) => vm.patchConfig({ minSaveIntervalSecs: toInt(e.target.value) })}
                      disabled={vm.configBusy}
                    />
                  </Box>

                  <TextField
                    multiline
                    minRows={3}
                    size="small"
                    fullWidth
                    label={t('history.config.ignorePatterns')}
                    value={vm.config.ignorePatterns.join('\n')}
                    onChange={(e) => vm.patchConfig({ ignorePatterns: e.target.value.split('\n') })}
                    disabled={vm.configBusy}
                    sx={{ '& textarea': { fontFamily: 'monospace', fontSize: 12 } }}
                  />

                  <Box sx={styles.configActions}>
                    <Button
                      size="small"
                      variant="contained"
                      startIcon={<SaveRoundedIcon />}
                      onClick={onSaveConfig}
                      disabled={vm.configBusy}
                      sx={{ textTransform: 'none' }}
                    >
                      {t('history.config.save')}
                    </Button>
                    <Button
                      size="small"
                      variant="outlined"
                      color="warning"
                      startIcon={<CleaningServicesRoundedIcon />}
                      onClick={onCleanup}
                      disabled={vm.configBusy || disabled}
                      sx={{ textTransform: 'none' }}
                    >
                      {t('history.cleanup.action')}
                    </Button>
                  </Box>
                </Box>
              )}
            </>
          )}
        </>
      )}
    </Box>
  );
};
