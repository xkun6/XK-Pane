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
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import { useT } from '@/i18n';
import type { Memory } from '@/services';
import { memorysPageStyles } from './MemorysPage.styles';
import { useMemorysPage } from './useMemorysPage';

// MemorysPage：记忆子系统 UI（app 级，按 scope 分级）。搜索 + scope/category 过滤 + 统计条 +
// 新建（标题/内容/scope/类别/重要度/标签）+ 选中看-编辑 + 删除。端到端验证 memorysvc。

const SCOPES = ['global', 'workspace', 'project', 'session'] as const;
const CATEGORIES = ['decision', 'lesson', 'preference', 'pattern', 'fact', 'plan'] as const;
const IMPORTANCES = [1, 2, 3, 4, 5] as const;

export const MemorysPage = () => {
  const theme = useTheme();
  const styles = memorysPageStyles(theme);
  const t = useT();
  const vm = useMemorysPage();

  const errorTitle = t('memory.error.title');
  const disabled = vm.busy || vm.loading;

  const scopeLabel = (s: string): string => {
    const k = `memory.scope.${s}`;
    return t(k) === k ? s : t(k);
  };
  const categoryLabel = (c: string): string => {
    const k = `memory.category.${c}`;
    return t(k) === k ? c : t(k);
  };

  const onCreate = (): void =>
    void vm.create({ errorTitle, scopeRefRequired: t('memory.create.scopeRefRequired') });

  const onSave = (): void =>
    void vm.saveEdit({
      savedTitle: t('memory.saved.title'),
      savedMessage: t('memory.saved.message'),
      errorTitle,
    });

  const onRemove = (m: Memory): void =>
    void vm.remove(m, {
      confirmTitle: t('memory.removeConfirm.title'),
      confirmMessage: t('memory.removeConfirm.message', { title: m.title }),
      okLabel: t('memory.removeConfirm.ok'),
      cancelLabel: t('memory.removeConfirm.cancel'),
      errorTitle,
    });

  const scopeRefLabel = (scope: string): string => {
    if (scope === 'workspace') return t('memory.field.workspaceName');
    if (scope === 'project') return t('memory.field.projectPath');
    if (scope === 'session') return t('memory.field.sessionId');
    return t('memory.field.scopeRef');
  };

  return (
    <Box sx={styles.root}>
      <Box sx={styles.header}>
        <Typography sx={styles.eyebrow}>{t('memory.eyebrow')}</Typography>
        <Typography sx={styles.title}>{t('memory.title')}</Typography>
      </Box>

      {vm.stats && (
        <Box sx={styles.statsRow}>
          <Chip
            size="small"
            variant="outlined"
            sx={{ borderRadius: 6 }}
            label={t('memory.stats.total', { count: String(vm.stats.total) })}
          />
          {CATEGORIES.map((c) =>
            (vm.stats?.byCategory[c] ?? 0) > 0 ? (
              <Chip
                key={c}
                size="small"
                variant="outlined"
                sx={{ borderRadius: 6 }}
                label={`${categoryLabel(c)} ${vm.stats?.byCategory[c] ?? 0}`}
              />
            ) : null,
          )}
        </Box>
      )}

      <Box sx={styles.filterRow}>
        <TextField
          size="small"
          sx={{ flex: 1, minWidth: 180 }}
          placeholder={t('memory.searchPlaceholder')}
          value={vm.search}
          onChange={(e) => vm.setSearch(e.target.value)}
        />
        <TextField
          select
          size="small"
          label={t('memory.filter.scope')}
          value={vm.scopeFilter}
          onChange={(e) => vm.setScopeFilter(e.target.value)}
          sx={{ minWidth: 130 }}
        >
          <MenuItem value="">{t('memory.filter.all')}</MenuItem>
          {SCOPES.map((s) => (
            <MenuItem key={s} value={s}>
              {scopeLabel(s)}
            </MenuItem>
          ))}
        </TextField>
        <TextField
          select
          size="small"
          label={t('memory.filter.category')}
          value={vm.categoryFilter}
          onChange={(e) => vm.setCategoryFilter(e.target.value)}
          sx={{ minWidth: 130 }}
        >
          <MenuItem value="">{t('memory.filter.all')}</MenuItem>
          {CATEGORIES.map((c) => (
            <MenuItem key={c} value={c}>
              {categoryLabel(c)}
            </MenuItem>
          ))}
        </TextField>
        <Tooltip title={t('memory.refresh')}>
          <span>
            <IconButton aria-label={t('memory.refresh')} onClick={() => void vm.refresh(errorTitle)} disabled={disabled}>
              <RefreshRoundedIcon />
            </IconButton>
          </span>
        </Tooltip>
      </Box>

      <Box sx={styles.createSection}>
        <Typography sx={styles.sectionTitle}>{t('memory.create.title')}</Typography>
        <TextField
          size="small"
          fullWidth
          placeholder={t('memory.field.titlePlaceholder')}
          value={vm.draft.title}
          onChange={(e) => vm.patchDraft({ title: e.target.value })}
          disabled={vm.busy}
        />
        <TextField
          size="small"
          fullWidth
          multiline
          minRows={2}
          placeholder={t('memory.field.contentPlaceholder')}
          value={vm.draft.content}
          onChange={(e) => vm.patchDraft({ content: e.target.value })}
          disabled={vm.busy}
        />
        <Box sx={styles.formGrid}>
          <TextField
            select
            size="small"
            label={t('memory.field.scope')}
            value={vm.draft.scope}
            onChange={(e) => vm.patchDraft({ scope: e.target.value })}
            disabled={vm.busy}
          >
            {SCOPES.map((s) => (
              <MenuItem key={s} value={s}>
                {scopeLabel(s)}
              </MenuItem>
            ))}
          </TextField>
          {vm.draft.scope !== 'global' && (
            <TextField
              size="small"
              label={scopeRefLabel(vm.draft.scope)}
              value={vm.draft.scopeRef}
              onChange={(e) => vm.patchDraft({ scopeRef: e.target.value })}
              disabled={vm.busy}
            />
          )}
          <TextField
            select
            size="small"
            label={t('memory.field.category')}
            value={vm.draft.category}
            onChange={(e) => vm.patchDraft({ category: e.target.value })}
            disabled={vm.busy}
          >
            {CATEGORIES.map((c) => (
              <MenuItem key={c} value={c}>
                {categoryLabel(c)}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            select
            size="small"
            label={t('memory.field.importance')}
            value={vm.draft.importance}
            onChange={(e) => vm.patchDraft({ importance: Number(e.target.value) })}
            disabled={vm.busy}
          >
            {IMPORTANCES.map((n) => (
              <MenuItem key={n} value={n}>
                {n}
              </MenuItem>
            ))}
          </TextField>
        </Box>
        <TextField
          size="small"
          fullWidth
          label={t('memory.field.tags')}
          placeholder={t('memory.field.tagsPlaceholder')}
          value={vm.draft.tags}
          onChange={(e) => vm.patchDraft({ tags: e.target.value })}
          disabled={vm.busy}
        />
        <Box>
          <Button
            size="small"
            variant="contained"
            startIcon={<AddRoundedIcon />}
            onClick={onCreate}
            disabled={vm.busy || !vm.draft.title.trim() || !vm.draft.content.trim()}
            sx={{ textTransform: 'none' }}
          >
            {t('memory.create.action')}
          </Button>
        </Box>
      </Box>

      <Typography sx={styles.sectionTitle}>{t('memory.list', { count: String(vm.total) })}</Typography>
      {!vm.loading && vm.items.length === 0 ? (
        <Box sx={styles.empty}>{t('memory.empty')}</Box>
      ) : (
        <Box sx={styles.list}>
          {vm.items.map((m) => (
            <Box key={m.id} sx={m.id === vm.selectedId ? { ...styles.row, ...styles.rowSelected } : styles.row}>
              <Box
                sx={styles.rowMain}
                role="button"
                tabIndex={0}
                onClick={() => vm.openMemory(m)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') vm.openMemory(m);
                }}
              >
                <Typography sx={styles.rowTitle} title={m.title}>
                  {m.title}
                </Typography>
                <Typography sx={styles.rowMeta}>{m.content}</Typography>
              </Box>
              <Chip size="small" variant="outlined" sx={{ borderRadius: 6 }} label={`P${m.importance}`} />
              <Chip size="small" variant="outlined" sx={{ borderRadius: 6 }} label={categoryLabel(m.category)} />
              <Chip size="small" variant="outlined" sx={{ borderRadius: 6 }} label={scopeLabel(m.scope)} />
              <Tooltip title={t('memory.actions.remove')}>
                <span>
                  <IconButton
                    size="small"
                    aria-label={t('memory.actions.remove')}
                    onClick={() => onRemove(m)}
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
              {t('memory.editTitle')}
              {vm.editDirty ? ' *' : ''}
            </Typography>
            <Button
              size="small"
              variant="contained"
              startIcon={<SaveRoundedIcon />}
              onClick={onSave}
              disabled={vm.busy || !vm.editDirty}
              sx={{ textTransform: 'none' }}
            >
              {t('memory.save')}
            </Button>
          </Box>
          <TextField
            size="small"
            fullWidth
            label={t('memory.field.titleLabel')}
            value={vm.edit.title}
            onChange={(e) => vm.patchEdit({ title: e.target.value })}
            disabled={vm.busy}
          />
          <TextField
            size="small"
            fullWidth
            multiline
            minRows={4}
            label={t('memory.field.contentLabel')}
            value={vm.edit.content}
            onChange={(e) => vm.patchEdit({ content: e.target.value })}
            disabled={vm.busy}
          />
          <Box sx={styles.formGrid}>
            <TextField
              select
              size="small"
              label={t('memory.field.category')}
              value={vm.edit.category}
              onChange={(e) => vm.patchEdit({ category: e.target.value })}
              disabled={vm.busy}
            >
              {CATEGORIES.map((c) => (
                <MenuItem key={c} value={c}>
                  {categoryLabel(c)}
                </MenuItem>
              ))}
            </TextField>
            <TextField
              select
              size="small"
              label={t('memory.field.importance')}
              value={vm.edit.importance}
              onChange={(e) => vm.patchEdit({ importance: Number(e.target.value) })}
              disabled={vm.busy}
            >
              {IMPORTANCES.map((n) => (
                <MenuItem key={n} value={n}>
                  {n}
                </MenuItem>
              ))}
            </TextField>
          </Box>
          <TextField
            size="small"
            fullWidth
            label={t('memory.field.tags')}
            value={vm.edit.tags}
            onChange={(e) => vm.patchEdit({ tags: e.target.value })}
            disabled={vm.busy}
          />
        </Box>
      )}
    </Box>
  );
};
