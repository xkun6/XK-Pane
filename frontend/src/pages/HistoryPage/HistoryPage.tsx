import {
  Box,
  Button,
  Chip,
  FormControlLabel,
  IconButton,
  MenuItem,
  Switch,
  Tab,
  Tabs,
  TextField,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material';
import { useEffect, useState, type KeyboardEvent } from 'react';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import DescriptionRoundedIcon from '@mui/icons-material/DescriptionRounded';
import ArrowBackRoundedIcon from '@mui/icons-material/ArrowBackRounded';
import SellRoundedIcon from '@mui/icons-material/SellRounded';
import RestoreRoundedIcon from '@mui/icons-material/RestoreRounded';
import PhotoCameraRoundedIcon from '@mui/icons-material/PhotoCameraRounded';
import SaveRoundedIcon from '@mui/icons-material/SaveRounded';
import CleaningServicesRoundedIcon from '@mui/icons-material/CleaningServicesRounded';
import { useT } from '@/i18n';
import { DiffView } from '@/components/DiffView';
import { TextInputDialog } from '@/components/TextInputDialog';
import type { FileVersion, HistoryConfig, HistoryLabel } from '@/services';
import { useLocalHistoryChangeEvent } from '@/shared/hooks/useLocalHistoryChangeEvent';
import { historyPageStyles } from './HistoryPage.styles';
import { useHistoryPage, type HistoryViewMode } from './useHistoryPage';
import { formatSize, formatTime } from './historyFormat';

const KNOWN_SOURCES = new Set(['git_commit', 'claude_session', 'user', 'build', 'restore', 'branch_switch', 'auto', 'manual']);
const MB = 1024 * 1024;

// 两种用法见 GitPage：整页模式（项目选择器）/ 绑定模式（boundPath，右侧抽屉跟随当前目录）。
export interface HistoryPageProps {
  boundPath?: string | null;
  // 绑定模式下「当前打开的文件」（项目内正斜杠相对路径）。非空时自动跟随该文件直达时间线
  // （VS Code Timeline 行为）；为空 / undefined 时回落到文件列表挑选。仅本地历史抽屉传入。
  boundFile?: string | null;
}

export const HistoryPage = ({ boundPath, boundFile }: HistoryPageProps) => {
  const theme = useTheme();
  const styles = historyPageStyles(theme);
  const t = useT();
  const vm = useHistoryPage();

  const [tagVersion, setTagVersion] = useState<FileVersion | null>(null);
  const [snapshotPath, setSnapshotPath] = useState('');

  const errorTitle = t('history.error.title');
  const bound = boundPath !== undefined;

  // watcher 自动捕获 → live 刷新当前项目的列表 / 时间线。
  useLocalHistoryChangeEvent(vm.selectedPath, vm.reloadSilent);

  // 绑定模式：boundPath / boundFile 变化即重选目标。有 boundFile 则跟随该文件直达时间线
  // （VS Code Timeline）；否则回落到文件列表。只盯绑定输入，语言切换不重载（保留视图态）。
  useEffect(() => {
    if (!bound) return;
    if (boundPath && boundFile) void vm.followFile(boundPath, boundFile, errorTitle);
    else void vm.select(boundPath ?? '', errorTitle);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bound, boundPath, boundFile]);

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
  const showVersionActions =
    (vm.viewMode === 'diff' || vm.viewMode === 'content') && vm.selectedVersion !== null;

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

  const onRestoreVersion = (v: FileVersion): void =>
    void vm.restoreVersion(v, {
      confirmTitle: t('history.restoreConfirm.title'),
      confirmMessage: t('history.restoreConfirm.message', { path: vm.selectedFile ?? '' }),
      ok: t('history.restoreConfirm.ok'),
      cancel: t('history.restoreConfirm.cancel'),
      successTitle: t('history.restored.title'),
      successMessage: t('history.restored.message'),
      errorTitle,
    });

  const onSnapshot = (): void => {
    const rel = snapshotPath.trim();
    if (!rel) return;
    void vm.snapshot(rel, {
      createdTitle: t('history.snapshot.title'),
      createdMessage: t('history.snapshot.created'),
      noChangeMessage: t('history.snapshot.noChange'),
      errorTitle,
    });
    setSnapshotPath('');
  };

  const onSaveConfig = (): void =>
    void vm.saveConfig({
      successTitle: t('history.config.saved.title'),
      successMessage: t('history.config.saved.message'),
      errorTitle,
    });

  const onCleanup = (): void =>
    void vm.cleanup({
      confirmTitle: t('history.cleanup.confirm.title'),
      confirmMessage: t('history.cleanup.confirm.message'),
      ok: t('history.cleanup.confirm.ok'),
      cancel: t('history.cleanup.confirm.cancel'),
      doneTitle: t('history.cleanup.done.title'),
      doneMessage: t('history.cleanup.done.message'),
      errorTitle,
    });

  // 配置编辑：基于当前 config 派生新对象交给 VM（VM 不持翻译串，纯数据）。
  const patchConfig = (patch: Partial<HistoryConfig>): void => {
    if (vm.config) vm.setConfig({ ...vm.config, ...patch });
  };
  const numField = (v: string): number => {
    const n = Number(v);
    return Number.isFinite(n) && n >= 0 ? Math.floor(n) : 0;
  };

  const renderVersionRow = (v: FileVersion) => {
    const selected = vm.selectedVersion?.id === v.id;
    const vLabels = vm.versionLabels[v.id] ?? [];
    return (
      <Box
        key={v.id}
        sx={{ ...styles.versionRow, ...(selected ? styles.versionRowSelected : {}) }}
        onClick={() => void vm.selectVersion(v, errorTitle)}
        onContextMenu={(e) => {
          e.preventDefault();
          setTagVersion(v);
        }}
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
      {!bound && (
        <Box sx={styles.header}>
          <Typography sx={styles.eyebrow}>{t('history.eyebrow')}</Typography>
          <Typography sx={styles.title}>{t('history.title')}</Typography>
        </Box>
      )}

      {(bound ? !boundPath : vm.projects.length === 0) ? (
        <Box sx={styles.empty}>{bound ? t('history.panel.noDir') : t('history.noProjects')}</Box>
      ) : (
        <>
          <Box sx={bound ? { ...styles.pickerRow, justifyContent: 'flex-end' } : styles.pickerRow}>
            {!bound && (
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
            )}
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
                <Box sx={styles.timeline} tabIndex={0} onKeyDown={onTimelineKeyDown}>
                  {vm.filteredVersions.length === 0 ? (
                    <Box sx={styles.empty}>{t('history.timeline.empty')}</Box>
                  ) : (
                    vm.filteredVersions.map(renderVersionRow)
                  )}
                </Box>

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
                      <Tab value="project-restore" label={t('history.tabs.projectRestore')} sx={{ minHeight: 36, textTransform: 'none' }} />
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

                  {showVersionActions && vm.selectedVersion && (
                    <Box sx={styles.previewToolbar}>
                      <Button
                        size="small"
                        variant="outlined"
                        startIcon={<SellRoundedIcon />}
                        onClick={() => setTagVersion(vm.selectedVersion)}
                        sx={{ textTransform: 'none' }}
                      >
                        {t('history.tag.add')}
                      </Button>
                      <Button
                        size="small"
                        variant="outlined"
                        startIcon={<RestoreRoundedIcon />}
                        onClick={() => vm.selectedVersion && onRestoreVersion(vm.selectedVersion)}
                        sx={{ textTransform: 'none' }}
                      >
                        {t('history.restore')}
                      </Button>
                      <Typography sx={styles.hint}>{t('history.timeline.rightClickHint')}</Typography>
                    </Box>
                  )}

                  {renderPreviewBody()}
                </Box>
              </Box>
            </>
          )}

          {/* 操作区：手动快照 + 配置 + 统计 + 清理（项目级，仅阶段1=文件列表时显示，
              进入某文件历史时让位给时间线+diff，保持页面干净） */}
          {vm.selectedPath && vm.config && !vm.selectedFile && (
            <>
              <Box sx={styles.section}>
                <Typography sx={styles.sectionTitle}>{t('history.snapshot.title')}</Typography>
                <Box sx={styles.snapshotRow}>
                  <TextField
                    size="small"
                    fullWidth
                    placeholder={t('history.snapshotPlaceholder')}
                    value={snapshotPath}
                    onChange={(e) => setSnapshotPath(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault();
                        onSnapshot();
                      }
                    }}
                  />
                  <Button
                    size="small"
                    variant="outlined"
                    startIcon={<PhotoCameraRoundedIcon />}
                    onClick={onSnapshot}
                    disabled={!snapshotPath.trim()}
                    sx={{ textTransform: 'none', whiteSpace: 'nowrap' }}
                  >
                    {t('history.snapshotAction')}
                  </Button>
                </Box>
              </Box>

              <Box sx={styles.section}>
                <Typography sx={styles.sectionTitle}>{t('history.config.title')}</Typography>
                <FormControlLabel
                  control={
                    <Switch
                      checked={vm.config.enabled}
                      onChange={(e) => patchConfig({ enabled: e.target.checked })}
                    />
                  }
                  label={t('history.config.enabled')}
                />
                <Box sx={styles.configGrid}>
                  <TextField
                    size="small"
                    type="number"
                    label={t('history.config.maxVersionsPerFile')}
                    value={vm.config.maxVersionsPerFile}
                    onChange={(e) => patchConfig({ maxVersionsPerFile: numField(e.target.value) })}
                  />
                  <TextField
                    size="small"
                    type="number"
                    label={t('history.config.maxAgeDays')}
                    value={vm.config.maxAgeDays}
                    onChange={(e) => patchConfig({ maxAgeDays: numField(e.target.value) })}
                  />
                  <TextField
                    size="small"
                    type="number"
                    label={t('history.config.maxFileSizeMB')}
                    value={Math.round(vm.config.maxFileSize / MB)}
                    onChange={(e) => patchConfig({ maxFileSize: numField(e.target.value) * MB })}
                  />
                  <TextField
                    size="small"
                    type="number"
                    label={t('history.config.maxTotalSizeMB')}
                    value={Math.round(vm.config.maxTotalSize / MB)}
                    onChange={(e) => patchConfig({ maxTotalSize: numField(e.target.value) * MB })}
                  />
                  <TextField
                    size="small"
                    type="number"
                    label={t('history.config.minSaveIntervalSecs')}
                    value={vm.config.minSaveIntervalSecs}
                    onChange={(e) => patchConfig({ minSaveIntervalSecs: numField(e.target.value) })}
                  />
                </Box>
                <TextField
                  size="small"
                  fullWidth
                  multiline
                  minRows={3}
                  maxRows={8}
                  label={t('history.config.ignorePatterns')}
                  value={vm.config.ignorePatterns.join('\n')}
                  onChange={(e) => patchConfig({ ignorePatterns: e.target.value.split('\n') })}
                />
                <Box sx={styles.configActions}>
                  <Button
                    size="small"
                    variant="contained"
                    startIcon={<SaveRoundedIcon />}
                    onClick={onSaveConfig}
                    sx={{ textTransform: 'none' }}
                  >
                    {t('history.config.save')}
                  </Button>
                </Box>
              </Box>

              <Box sx={styles.section}>
                <Typography sx={styles.sectionTitle}>{t('history.stats.title')}</Typography>
                {vm.stats && (
                  <Typography sx={styles.statsSummary}>
                    {t('history.stats.summary', {
                      versions: String(vm.stats.versionCount),
                      files: String(vm.stats.fileCount),
                      size: formatSize(vm.stats.totalBlobSize),
                    })}
                  </Typography>
                )}
                <Box sx={styles.configActions}>
                  <Button
                    size="small"
                    variant="outlined"
                    startIcon={<CleaningServicesRoundedIcon />}
                    onClick={onCleanup}
                    sx={{ textTransform: 'none' }}
                  >
                    {t('history.cleanup.action')}
                  </Button>
                </Box>
              </Box>
            </>
          )}
        </>
      )}

      <TextInputDialog
        open={tagVersion !== null}
        title={t('history.tag.title')}
        label={t('history.tag.label')}
        okLabel={t('history.tag.ok')}
        cancelLabel={t('common.cancel')}
        initialValue=""
        onSubmit={(name) => {
          const v = tagVersion;
          setTagVersion(null);
          if (v) {
            void vm.addTag(v, name, {
              successTitle: t('history.tag.added.title'),
              successMessage: t('history.tag.added.message'),
              errorTitle: t('history.tag.failed'),
            });
          }
        }}
        onClose={() => setTagVersion(null)}
      />
    </Box>
  );
};
