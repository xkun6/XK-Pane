import {
  Box,
  Chip,
  IconButton,
  MenuItem,
  Tab,
  Tabs,
  TextField,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material';
import type { KeyboardEvent } from 'react';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import DescriptionRoundedIcon from '@mui/icons-material/DescriptionRounded';
import ArrowBackRoundedIcon from '@mui/icons-material/ArrowBackRounded';
import SellRoundedIcon from '@mui/icons-material/SellRounded';
import RestoreRoundedIcon from '@mui/icons-material/RestoreRounded';
import { useT } from '@/i18n';
import { DiffView } from '@/components/DiffView';
import type { FileVersion, HistoryLabel } from '@/services';
import { historyPageStyles } from './HistoryPage.styles';
import { useHistoryPage, type HistoryViewMode } from './useHistoryPage';
import { formatSize, formatTime } from './historyFormat';

const KNOWN_SOURCES = new Set(['git_commit', 'claude_session', 'user', 'build', 'restore', 'branch_switch', 'auto', 'manual']);

export const HistoryPage = () => {
  const theme = useTheme();
  const styles = historyPageStyles(theme);
  const t = useT();
  const vm = useHistoryPage();

  const errorTitle = t('history.error.title');

  // 标签来源 → 语义色（取 theme.palette.foundation.*，禁硬编码十六进制）。
  const sourceColor = (source: string): string => {
    const f = theme.palette.foundation;
    switch (source) {
      case 'git_commit':
        return f.status.warning;
      case 'build':
        return f.status.success;
      case 'restore':
        return f.status.danger;
      case 'claude_session':
      case 'user':
        return f.accent;
      default:
        return f.text.secondary;
    }
  };
  const sourceLabel = (source: string): string =>
    KNOWN_SOURCES.has(source) ? t(`history.projectRestore.source.${source}`) : source;

  const diffLabels = {
    binary: t('history.diff.binary'),
    truncated: t('history.diff.truncated'),
    identical: t('history.diff.identical'),
    added: t('history.diff.added'),
    removed: t('history.diff.removed'),
  };

  const showBranchFilter =
    (vm.viewMode === 'diff' || vm.viewMode === 'content') && vm.fileBranches.filter(Boolean).length > 1;

  const onTimelineKeyDown = (e: KeyboardEvent<HTMLDivElement>): void => {
    const list = vm.filteredVersions;
    if (list.length === 0) return;
    const idx = vm.selectedVersion ? list.findIndex((v) => v.id === vm.selectedVersion!.id) : -1;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      const next = list[Math.min(idx + 1, list.length - 1)];
      if (next) void vm.selectVersion(next, errorTitle);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      const prev = list[Math.max(idx - 1, 0)];
      if (prev) void vm.selectVersion(prev, errorTitle);
    }
  };

  const renderVersionRow = (v: FileVersion) => {
    const selected = vm.selectedVersion?.id === v.id;
    const vLabels = vm.versionLabels[v.id] ?? [];
    return (
      <Box
        key={v.id}
        sx={{ ...styles.versionRow, ...(selected ? styles.versionRowSelected : {}) }}
        onClick={() => void vm.selectVersion(v, errorTitle)}
      >
        <Typography sx={styles.versionTime}>{formatTime(v.createdAt)}</Typography>
        <Box sx={styles.versionMeta}>
          <Box component="span">{formatSize(v.size)}</Box>
          <Chip
            label={v.branch || t('history.branchFilter.unknown')}
            size="small"
            variant="outlined"
            sx={{ borderRadius: 6, fontFamily: 'monospace', height: 18 }}
          />
        </Box>
        {vLabels.length > 0 && (
          <Box sx={styles.versionBadges}>
            {vLabels.map((l) => (
              <Chip
                key={l.id}
                icon={<SellRoundedIcon />}
                label={l.name}
                size="small"
                variant="outlined"
                sx={{ borderRadius: 6, height: 18, color: sourceColor(l.source), borderColor: sourceColor(l.source) }}
              />
            ))}
          </Box>
        )}
      </Box>
    );
  };

  const renderDeletedTab = () => {
    if (vm.deletedLoading) return <Box sx={styles.empty}>{t('history.fileList.loading')}</Box>;
    if (vm.deletedFiles.length === 0) return <Box sx={styles.empty}>{t('history.deleted.empty')}</Box>;
    return (
      <Box sx={styles.list}>
        {vm.deletedFiles.map((f) => (
          <Box key={`${f.filePath}-${f.id}`} sx={styles.listRow}>
            <Box sx={styles.listMain}>
              <Typography sx={styles.listTitle} title={f.filePath}>
                {f.filePath}
              </Typography>
              <Box sx={styles.listMeta}>
                <Box component="span">{formatTime(f.createdAt)}</Box>
                {f.branch && <Box component="span">{f.branch}</Box>}
              </Box>
            </Box>
            <Tooltip title={t('history.deleted.restore')}>
              <IconButton
                size="small"
                aria-label={t('history.deleted.restore')}
                onClick={() =>
                  void vm.restoreDeleted(f, {
                    confirmTitle: t('history.deleted.restoreConfirm.title'),
                    confirmMessage: t('history.deleted.restoreConfirm.message', { path: f.filePath }),
                    ok: t('history.deleted.restoreConfirm.ok'),
                    cancel: t('history.deleted.restoreConfirm.cancel'),
                    successTitle: t('history.deleted.restored.title'),
                    successMessage: t('history.deleted.restored.message'),
                    errorTitle,
                  })
                }
              >
                <RestoreRoundedIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Box>
        ))}
      </Box>
    );
  };

  const renderProjectRestoreTab = () => {
    if (vm.labels.length === 0) return <Box sx={styles.empty}>{t('history.projectRestore.empty')}</Box>;
    return (
      <Box sx={styles.list}>
        {vm.labels.map((l: HistoryLabel) => (
          <Box key={l.id} sx={styles.listRow}>
            <Box sx={styles.listMain}>
              <Typography sx={styles.listTitle} title={l.name}>
                {l.name}
              </Typography>
              <Box sx={styles.listMeta}>
                <Chip
                  label={sourceLabel(l.source)}
                  size="small"
                  variant="outlined"
                  sx={{ borderRadius: 6, height: 18, color: sourceColor(l.source), borderColor: sourceColor(l.source) }}
                />
                <Box component="span">{t('history.projectRestore.fileCount', { count: String(l.fileSnapshots.length) })}</Box>
                <Box component="span">{formatTime(l.timestamp)}</Box>
              </Box>
            </Box>
            <Tooltip title={t('history.projectRestore.restoreTo')}>
              <IconButton
                size="small"
                aria-label={t('history.projectRestore.restoreTo')}
                onClick={() =>
                  void vm.restoreToLabel(l, {
                    confirmTitle: t('history.projectRestore.confirm.title'),
                    confirmMessage: t('history.projectRestore.confirm.message', {
                      name: l.name,
                      count: String(l.fileSnapshots.length),
                    }),
                    ok: t('history.projectRestore.confirm.ok'),
                    cancel: t('history.projectRestore.confirm.cancel'),
                    successTitle: t('history.projectRestore.restored.title'),
                    successMessage: t('history.projectRestore.restored.message'),
                    errorTitle,
                  })
                }
              >
                <RestoreRoundedIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Box>
        ))}
      </Box>
    );
  };

  const renderPreviewBody = () => {
    switch (vm.viewMode) {
      case 'diff':
        if (vm.diffLoading) return <Box sx={styles.empty}>{t('history.diff.loading')}</Box>;
        if (!vm.selectedVersion) return <Box sx={styles.empty}>{t('history.timeline.selectVersion')}</Box>;
        return (
          <>
            {vm.diffCaption && <Typography sx={styles.previewCaption}>{t(`history.diff.${vm.diffCaption}`)}</Typography>}
            {vm.diff && (
              <Box sx={styles.previewBody}>
                <DiffView diff={vm.diff} labels={diffLabels} />
              </Box>
            )}
          </>
        );
      case 'content':
        if (vm.contentLoading) return <Box sx={styles.empty}>{t('history.content.loading')}</Box>;
        if (!vm.selectedVersion) return <Box sx={styles.empty}>{t('history.timeline.selectVersion')}</Box>;
        return (
          <Box sx={styles.previewBody}>
            {vm.content === '' ? (
              <Typography sx={styles.hint}>{t('history.content.empty')}</Typography>
            ) : (
              <Box component="pre" sx={styles.contentPre}>
                {vm.content}
              </Box>
            )}
          </Box>
        );
      case 'deleted':
        return <Box sx={styles.previewBody}>{renderDeletedTab()}</Box>;
      case 'project-restore':
        return <Box sx={styles.previewBody}>{renderProjectRestoreTab()}</Box>;
      default:
        return null;
    }
  };

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
                    <Box key={f.filePath} sx={styles.fileRow} onClick={() => void vm.selectFile(f.filePath, errorTitle)}>
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
                          <Chip label={f.branch} size="small" variant="outlined" sx={{ borderRadius: 6, fontFamily: 'monospace' }} />
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

          {/* 阶段 2：版本时间线 + 4-tab 预览 */}
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

              <Box sx={styles.stage2}>
                {/* 左：版本时间线（倒序） */}
                <Box sx={styles.timeline} tabIndex={0} onKeyDown={onTimelineKeyDown}>
                  {vm.filteredVersions.length === 0 ? (
                    <Box sx={styles.empty}>{t('history.timeline.empty')}</Box>
                  ) : (
                    vm.filteredVersions.map(renderVersionRow)
                  )}
                </Box>

                {/* 右：4-tab 预览 */}
                <Box sx={styles.preview}>
                  <Box sx={styles.previewToolbar}>
                    <Tabs
                      value={vm.viewMode}
                      onChange={(_, val) => vm.setViewMode(val as HistoryViewMode, errorTitle)}
                      variant="scrollable"
                      scrollButtons={false}
                      sx={{ minHeight: 36, flex: 1 }}
                    >
                      <Tab value="diff" label={t('history.tabs.diff')} sx={{ minHeight: 36, textTransform: 'none' }} />
                      <Tab value="content" label={t('history.tabs.content')} sx={{ minHeight: 36, textTransform: 'none' }} />
                      <Tab value="deleted" label={t('history.tabs.deleted')} sx={{ minHeight: 36, textTransform: 'none' }} />
                      <Tab
                        value="project-restore"
                        label={t('history.tabs.projectRestore')}
                        sx={{ minHeight: 36, textTransform: 'none' }}
                      />
                    </Tabs>
                    {showBranchFilter && (
                      <TextField
                        select
                        size="small"
                        label={t('history.branchFilter.label')}
                        value={vm.branchFilter}
                        onChange={(e) => vm.setBranchFilter(e.target.value, errorTitle)}
                        sx={{ minWidth: 140 }}
                      >
                        <MenuItem value="">{t('history.branchFilter.all')}</MenuItem>
                        {vm.fileBranches.filter(Boolean).map((b) => (
                          <MenuItem key={b} value={b}>
                            {b}
                          </MenuItem>
                        ))}
                      </TextField>
                    )}
                  </Box>
                  {renderPreviewBody()}
                </Box>
              </Box>
            </>
          )}
        </>
      )}
    </Box>
  );
};
