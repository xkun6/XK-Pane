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
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import SaveRoundedIcon from '@mui/icons-material/SaveRounded';
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded';
import ArchiveRoundedIcon from '@mui/icons-material/ArchiveRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import { useT } from '@/i18n';
import type { Spec } from '@/services';
import { specsPageStyles } from './SpecsPage.styles';
import { useSpecsPage } from './useSpecsPage';

// SpecsPage：选已登记项目 → 列出 / 新建需求规格 → 选中看-编辑 .md 内容 → 激活 / 归档 / 删除。
// 端到端验证 specsvc（Create/List/GetContent/SaveContent/Update/Delete）。内容编辑用 MVP textarea。

const STATUS_COLOR: Record<string, 'success' | 'secondary' | 'warning'> = {
  active: 'success',
  draft: 'secondary',
  archived: 'warning',
};

export const SpecsPage = () => {
  const theme = useTheme();
  const styles = specsPageStyles(theme);
  const t = useT();
  const vm = useSpecsPage();

  const errorTitle = t('specs.error.title');
  const disabled = vm.busy || vm.loading;

  const onCreate = (): void => void vm.create({ errorTitle });

  const onSave = (): void =>
    void vm.saveContent({
      savedTitle: t('specs.saved.title'),
      savedMessage: t('specs.saved.message'),
      errorTitle,
    });

  const onArchive = (spec: Spec): void =>
    void vm.archive(spec, {
      confirmTitle: t('specs.archiveConfirm.title'),
      confirmMessage: t('specs.archiveConfirm.message', { title: spec.title }),
      okLabel: t('specs.archiveConfirm.ok'),
      cancelLabel: t('specs.archiveConfirm.cancel'),
      errorTitle,
    });

  const onRemove = (spec: Spec): void =>
    void vm.remove(spec, {
      confirmTitle: t('specs.removeConfirm.title'),
      confirmMessage: t('specs.removeConfirm.message', { title: spec.title }),
      okLabel: t('specs.removeConfirm.ok'),
      cancelLabel: t('specs.removeConfirm.cancel'),
      errorTitle,
    });

  const statusLabel = (status: string): string =>
    t(`specs.status.${status}`) === `specs.status.${status}` ? status : t(`specs.status.${status}`);

  const selectedSpec = vm.specs.find((s) => s.id === vm.selectedId) ?? null;

  return (
    <Box sx={styles.root}>
      <Box sx={styles.header}>
        <Typography sx={styles.eyebrow}>{t('specs.eyebrow')}</Typography>
        <Typography sx={styles.title}>{t('specs.title')}</Typography>
      </Box>

      {vm.projects.length === 0 ? (
        <Box sx={styles.empty}>{t('specs.noProjects')}</Box>
      ) : (
        <>
          <Box sx={styles.pickerRow}>
            <TextField
              select
              size="small"
              fullWidth
              label={t('specs.project')}
              value={vm.selectedPath}
              onChange={(e) => void vm.select(e.target.value, errorTitle)}
            >
              {vm.projects.map((p) => (
                <MenuItem key={p.id} value={p.path}>
                  {p.alias ?? p.name}
                </MenuItem>
              ))}
            </TextField>
            <Tooltip title={t('specs.refresh')}>
              <span>
                <IconButton
                  aria-label={t('specs.refresh')}
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
              <Box sx={styles.createRow}>
                <TextField
                  size="small"
                  fullWidth
                  placeholder={t('specs.createPlaceholder')}
                  value={vm.newTitle}
                  onChange={(e) => vm.setNewTitle(e.target.value)}
                  disabled={disabled}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') onCreate();
                  }}
                />
                <Button
                  size="small"
                  variant="contained"
                  startIcon={<AddRoundedIcon />}
                  onClick={onCreate}
                  disabled={disabled || !vm.newTitle.trim()}
                  sx={{ textTransform: 'none', whiteSpace: 'nowrap' }}
                >
                  {t('specs.create')}
                </Button>
              </Box>

              <Typography sx={styles.sectionTitle}>
                {t('specs.list', { count: String(vm.specs.length) })}
              </Typography>
              {!vm.loading && vm.specs.length === 0 ? (
                <Box sx={styles.empty}>{t('specs.empty')}</Box>
              ) : (
                <Box sx={styles.list}>
                  {vm.specs.map((spec) => (
                    <Box
                      key={spec.id}
                      sx={spec.id === vm.selectedId ? { ...styles.row, ...styles.rowSelected } : styles.row}
                    >
                      <Box
                        sx={styles.rowMain}
                        role="button"
                        tabIndex={0}
                        onClick={() => void vm.openSpec(spec, errorTitle)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') void vm.openSpec(spec, errorTitle);
                        }}
                      >
                        <Typography sx={styles.specTitle} title={spec.title}>
                          {spec.title}
                        </Typography>
                        <Typography sx={styles.fileName}>{spec.fileName}</Typography>
                      </Box>
                      <Chip
                        label={statusLabel(spec.status)}
                        size="small"
                        color={STATUS_COLOR[spec.status] ?? 'secondary'}
                        variant="outlined"
                        sx={{ borderRadius: 6 }}
                      />
                      {spec.status !== 'active' && spec.status !== 'archived' && (
                        <Tooltip title={t('specs.actions.activate')}>
                          <span>
                            <IconButton
                              size="small"
                              aria-label={t('specs.actions.activate')}
                              onClick={() => void vm.activate(spec, errorTitle)}
                              disabled={disabled}
                            >
                              <PlayArrowRoundedIcon fontSize="small" />
                            </IconButton>
                          </span>
                        </Tooltip>
                      )}
                      {spec.status !== 'archived' && (
                        <Tooltip title={t('specs.actions.archive')}>
                          <span>
                            <IconButton
                              size="small"
                              aria-label={t('specs.actions.archive')}
                              onClick={() => onArchive(spec)}
                              disabled={disabled}
                            >
                              <ArchiveRoundedIcon fontSize="small" />
                            </IconButton>
                          </span>
                        </Tooltip>
                      )}
                      <Tooltip title={t('specs.actions.remove')}>
                        <span>
                          <IconButton
                            size="small"
                            aria-label={t('specs.actions.remove')}
                            onClick={() => onRemove(spec)}
                            disabled={disabled}
                          >
                            <DeleteOutlineRoundedIcon fontSize="small" />
                          </IconButton>
                        </span>
                      </Tooltip>
                    </Box>
                  ))}
                </Box>
              )}

              {vm.selectedId && (
                <Box sx={styles.editorSection}>
                  <Box sx={styles.editorHeader}>
                    <Typography sx={styles.editorTitle}>
                      {selectedSpec?.title ?? ''}
                      {vm.contentDirty ? ' *' : ''}
                    </Typography>
                    <Button
                      size="small"
                      variant="contained"
                      startIcon={<SaveRoundedIcon />}
                      onClick={onSave}
                      disabled={disabled || !vm.contentDirty}
                      sx={{ textTransform: 'none' }}
                    >
                      {t('specs.save')}
                    </Button>
                  </Box>
                  {vm.contentLoading ? (
                    <Box sx={styles.empty}>{t('specs.contentLoading')}</Box>
                  ) : (
                    <TextField
                      multiline
                      minRows={12}
                      fullWidth
                      value={vm.content}
                      onChange={(e) => vm.setContent(e.target.value)}
                      disabled={vm.busy}
                      sx={styles.editor}
                    />
                  )}
                </Box>
              )}
            </>
          )}
        </>
      )}
    </Box>
  );
};
