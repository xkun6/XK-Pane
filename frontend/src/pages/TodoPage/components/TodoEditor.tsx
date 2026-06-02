import { useState, type ReactNode } from 'react';
import {
  Box,
  Button,
  Chip,
  IconButton,
  InputAdornment,
  LinearProgress,
  MenuItem,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material';
import SaveRoundedIcon from '@mui/icons-material/SaveRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import CloseRoundedIcon from '@mui/icons-material/CloseRounded';
import EditNoteRoundedIcon from '@mui/icons-material/EditNoteRounded';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import CheckBoxRoundedIcon from '@mui/icons-material/CheckBoxRounded';
import CheckBoxOutlineBlankRoundedIcon from '@mui/icons-material/CheckBoxOutlineBlankRounded';
import { useT } from '@/i18n';
import { BUILTIN_TODO_TYPES, type TodoEditForm } from '../useTodoPage';
import type { TodoSubtask } from '@/services';
import { todoPageStyles } from '../TodoPage.styles';

interface Props {
  form: TodoEditForm;
  isCreating: boolean;
  editingId: string | null;
  subtasks: TodoSubtask[];
  allTypes: string[];
  // scopeRef 下拉数据源（结构化挑选，替代手填）：项目用 path、工作空间用 name。
  projects: { path: string; name: string; alias: string | null }[];
  workspaces: { name: string; alias: string | null }[];
  onPatch: (patch: Partial<TodoEditForm>) => void;
  onSave: () => void;
  onClose: () => void;
  onDelete: () => void;
  onAddCustomType: (type: string) => void;
  onRemoveCustomType: (type: string) => void;
  onAddSubtask: (title: string) => void;
  onToggleSubtask: (subtaskId: string, completed: boolean) => void;
  onRemoveSubtask: (subtaskId: string) => void;
}

const RECURRENCE = ['', 'daily', 'weekly', 'monthly'] as const;

// 日期/时间归一：编辑器统一把 dueDate / reminderAt 存为 RFC3339（对齐 cc-pane new Date(v).toISOString()），
// 让后端 overdue 比较与 recurrence 的 reminder 偏移（RFC3339 解析）一致生效。
// dateInputToISO："YYYY-MM-DD"（date 输入，按 UTC 午夜解析）→ RFC3339；空串保持空。
const dateInputToISO = (v: string): string => (v ? new Date(v).toISOString() : '');
// localInputToISO："YYYY-MM-DDTHH:mm"（datetime-local，按本地时区解析）→ RFC3339。
const localInputToISO = (v: string): string => (v ? new Date(v).toISOString() : '');
// isoToLocalInput：RFC3339 → datetime-local 显示值（本地壁钟，避免直接 slice 显示成 UTC 偏移）。
const isoToLocalInput = (iso: string): string => {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const pad = (n: number): string => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

export const TodoEditor = (p: Props) => {
  const theme = useTheme();
  const styles = todoPageStyles(theme);
  const t = useT();
  const [newTag, setNewTag] = useState('');
  const [newType, setNewType] = useState('');
  const [newSub, setNewSub] = useState('');

  const { form } = p;
  const needsScopeRef = form.scope === 'workspace' || form.scope === 'project';
  // scope=workspace/project 时 scopeRef 必填（对齐后端校验），缺则禁用保存并提示。
  const scopeRefMissing = needsScopeRef && !form.scopeRef.trim();

  const seg = (
    value: string,
    options: { value: string; label: string }[],
    onChange: (v: string) => void,
  ): ReactNode => (
    <ToggleButtonGroup
      exclusive
      size="small"
      value={value}
      onChange={(_, v) => v && onChange(v)}
      sx={styles.segGroup}
      fullWidth
    >
      {options.map((o) => (
        <ToggleButton key={o.value} value={o.value}>
          {o.label}
        </ToggleButton>
      ))}
    </ToggleButtonGroup>
  );

  const propRow = (label: string, control: ReactNode): ReactNode => (
    <Box sx={styles.propRow}>
      <Typography sx={styles.propLabel}>{label}</Typography>
      {control}
    </Box>
  );

  const typeLabel = (v: string): string =>
    (BUILTIN_TODO_TYPES as readonly string[]).includes(v) ? t(`todo.type.${v}`) : v;

  const commitTag = (): void => {
    const tag = newTag.trim();
    if (tag && !form.tags.includes(tag)) p.onPatch({ tags: [...form.tags, tag] });
    setNewTag('');
  };

  const commitType = (): void => {
    const v = newType.trim().toLowerCase();
    if (v) p.onAddCustomType(v);
    setNewType('');
  };

  const commitSub = (): void => {
    if (newSub.trim()) {
      p.onAddSubtask(newSub);
      setNewSub('');
    }
  };

  const doneSubs = p.subtasks.filter((s) => s.completed).length;
  const subPct = p.subtasks.length > 0 ? (doneSubs / p.subtasks.length) * 100 : 0;

  return (
    <Box
      sx={styles.rightPane}
      onKeyDown={(e) => {
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
          e.preventDefault();
          p.onSave();
        }
      }}
    >
      <Box sx={styles.editorHeader}>
        <Box sx={styles.editorBadge}>
          <EditNoteRoundedIcon fontSize="small" />
          <span>{p.isCreating ? t('todo.editor.new') : t('todo.editor.detail')}</span>
        </Box>
        <Button
          size="small"
          variant="contained"
          startIcon={<SaveRoundedIcon fontSize="small" />}
          disabled={!form.title.trim() || scopeRefMissing}
          onClick={p.onSave}
          sx={{ borderRadius: 6, textTransform: 'none' }}
        >
          {p.isCreating ? t('todo.editor.create') : t('todo.editor.save')}
        </Button>
        {!p.isCreating && (
          <Tooltip title={t('todo.editor.delete')}>
            <IconButton size="small" onClick={p.onDelete} aria-label={t('todo.editor.delete')}>
              <DeleteOutlineRoundedIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        )}
        <Tooltip title={t('todo.editor.close')}>
          <IconButton size="small" onClick={p.onClose} aria-label={t('todo.editor.close')}>
            <CloseRoundedIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>

      <Box sx={styles.editorBody}>
        <TextField
          variant="standard"
          placeholder={t('todo.editor.titlePlaceholder')}
          value={form.title}
          onChange={(e) => p.onPatch({ title: e.target.value })}
          sx={styles.titleInput}
          fullWidth
        />

        {propRow(
          t('todo.editor.status'),
          seg(
            form.status,
            [
              { value: 'todo', label: t('todo.status.todo') },
              { value: 'in_progress', label: t('todo.status.in_progress') },
              { value: 'done', label: t('todo.status.done') },
            ],
            (v) => p.onPatch({ status: v }),
          ),
        )}

        {propRow(
          t('todo.editor.priority'),
          seg(
            form.priority,
            [
              { value: 'high', label: t('todo.priority.high') },
              { value: 'medium', label: t('todo.priority.medium') },
              { value: 'low', label: t('todo.priority.low') },
            ],
            (v) => p.onPatch({ priority: v }),
          ),
        )}

        {propRow(
          t('todo.editor.scope'),
          seg(
            form.scope,
            [
              { value: 'global', label: t('todo.scope.global') },
              { value: 'workspace', label: t('todo.scope.workspace') },
              { value: 'project', label: t('todo.scope.project') },
              { value: 'external', label: t('todo.scope.external') },
              { value: 'temp_script', label: t('todo.scope.temp_script') },
            ],
            // 切换作用域时清空 scopeRef（对齐 cc-pane）。
            (v) => p.onPatch({ scope: v, scopeRef: '' }),
          ),
        )}

        {needsScopeRef &&
          propRow(
            t('todo.editor.scopeRef'),
            <TextField
              select
              size="small"
              value={form.scopeRef}
              onChange={(e) => p.onPatch({ scopeRef: e.target.value })}
              fullWidth
              error={scopeRefMissing}
              helperText={scopeRefMissing ? t('todo.editor.scopeRefRequired') : undefined}
            >
              <MenuItem value="">
                {t('todo.editor.scopeRefNone')}
              </MenuItem>
              {form.scope === 'project'
                ? p.projects.map((pr) => (
                    <MenuItem key={pr.path} value={pr.path}>
                      {pr.alias || pr.name}
                    </MenuItem>
                  ))
                : p.workspaces.map((ws) => (
                    <MenuItem key={ws.name} value={ws.name}>
                      {ws.alias || ws.name}
                    </MenuItem>
                  ))}
            </TextField>,
          )}

        {propRow(
          t('todo.editor.type'),
          <Box sx={styles.tagWrap}>
            <Chip
              size="small"
              label={t('todo.editor.noType')}
              color={form.todoType === '' ? 'primary' : 'default'}
              variant={form.todoType === '' ? 'filled' : 'outlined'}
              onClick={() => p.onPatch({ todoType: '' })}
              sx={styles.pill}
            />
            {p.allTypes.map((ty) => {
              const isCustom = !(BUILTIN_TODO_TYPES as readonly string[]).includes(ty);
              return (
                <Chip
                  key={ty}
                  size="small"
                  label={typeLabel(ty)}
                  color={form.todoType === ty ? 'primary' : 'default'}
                  variant={form.todoType === ty ? 'filled' : 'outlined'}
                  onClick={() => p.onPatch({ todoType: form.todoType === ty ? '' : ty })}
                  onDelete={isCustom ? () => p.onRemoveCustomType(ty) : undefined}
                  sx={styles.pill}
                />
              );
            })}
            <TextField
              size="small"
              variant="standard"
              placeholder={t('todo.editor.addType')}
              value={newType}
              onChange={(e) => setNewType(e.target.value)}
              onBlur={commitType}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  commitType();
                }
              }}
              sx={{ width: 96 }}
            />
          </Box>,
        )}

        {propRow(
          t('todo.editor.dueDate'),
          <TextField
            size="small"
            type="date"
            value={form.dueDate ? form.dueDate.slice(0, 10) : ''}
            onChange={(e) => p.onPatch({ dueDate: dateInputToISO(e.target.value) })}
            fullWidth
          />,
        )}

        {propRow(
          t('todo.editor.reminder'),
          <TextField
            size="small"
            type="datetime-local"
            value={form.reminderAt ? isoToLocalInput(form.reminderAt) : ''}
            onChange={(e) => p.onPatch({ reminderAt: localInputToISO(e.target.value) })}
            fullWidth
          />,
        )}

        {propRow(
          t('todo.editor.recurrence'),
          <TextField
            size="small"
            select
            value={form.recurrence}
            onChange={(e) => p.onPatch({ recurrence: e.target.value })}
            fullWidth
          >
            {RECURRENCE.map((r) => (
              <MenuItem key={r || 'none'} value={r}>
                {t(
                  r === ''
                    ? 'todo.editor.recurrenceNone'
                    : r === 'daily'
                      ? 'todo.editor.recurrenceDaily'
                      : r === 'weekly'
                        ? 'todo.editor.recurrenceWeekly'
                        : 'todo.editor.recurrenceMonthly',
                )}
              </MenuItem>
            ))}
          </TextField>,
        )}

        {propRow(
          t('todo.editor.tags'),
          <Box sx={styles.tagWrap}>
            {form.tags.map((tag) => (
              <Chip
                key={tag}
                size="small"
                label={tag}
                onDelete={() => p.onPatch({ tags: form.tags.filter((x) => x !== tag) })}
                sx={styles.pill}
              />
            ))}
            <TextField
              size="small"
              variant="standard"
              placeholder={t('todo.editor.addTag')}
              value={newTag}
              onChange={(e) => setNewTag(e.target.value)}
              onBlur={commitTag}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  commitTag();
                }
              }}
              sx={{ width: 110 }}
            />
          </Box>,
        )}

        {propRow(
          t('todo.editor.description'),
          <TextField
            size="small"
            multiline
            minRows={3}
            placeholder={t('todo.editor.descriptionHint')}
            value={form.description}
            onChange={(e) => p.onPatch({ description: e.target.value })}
            fullWidth
          />,
        )}

        {!p.isCreating && p.editingId && (
          <Box sx={styles.subBlock}>
            <Typography sx={styles.propLabel}>{t('todo.editor.subtasks')}</Typography>
            {p.subtasks.length > 0 && (
              <Box sx={styles.subProgress}>
                <span>{`${doneSubs}/${p.subtasks.length}`}</span>
                <LinearProgress variant="determinate" value={subPct} sx={{ flex: 1, borderRadius: 4 }} />
              </Box>
            )}
            {p.subtasks.map((s) => (
              <Box key={s.id} sx={styles.subRow}>
                <IconButton size="small" onClick={() => p.onToggleSubtask(s.id, !s.completed)} aria-label={s.title}>
                  {s.completed ? (
                    <CheckBoxRoundedIcon fontSize="small" sx={{ color: theme.palette.primary.main }} />
                  ) : (
                    <CheckBoxOutlineBlankRoundedIcon fontSize="small" />
                  )}
                </IconButton>
                <Typography sx={s.completed ? styles.subTitleDone : styles.subTitle}>{s.title}</Typography>
                <IconButton size="small" onClick={() => p.onRemoveSubtask(s.id)} aria-label={t('todo.editor.delete')}>
                  <DeleteOutlineRoundedIcon fontSize="small" />
                </IconButton>
              </Box>
            ))}
            <TextField
              size="small"
              placeholder={t('todo.editor.addSubtask')}
              value={newSub}
              onChange={(e) => setNewSub(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  commitSub();
                }
              }}
              slotProps={{
                input: {
                  endAdornment: (
                    <InputAdornment position="end">
                      <IconButton size="small" onClick={commitSub} aria-label={t('todo.editor.addSubtask')}>
                        <AddRoundedIcon fontSize="small" />
                      </IconButton>
                    </InputAdornment>
                  ),
                },
              }}
              fullWidth
            />
          </Box>
        )}
      </Box>
    </Box>
  );
};
