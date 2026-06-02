import { useEffect, useState } from 'react';
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
  TextField,
  Typography,
} from '@mui/material';
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded';
import { useT } from '@/i18n';
import type { Project, Todo } from '@/services';

interface Props {
  open: boolean;
  todo: Todo | null;
  projects: Project[];
  onClose: () => void;
  onConfirm: (project: Project, cliTool: string) => void;
}

// 可分派的 CLI（对齐后端保留的 3 适配器）。
const CLI_TOOLS = ['claude', 'codex', 'reclaude'];

// 分派对话框（XK 新增，cc-pane 无此 UI 入口）：选目标项目 + CLI → 起会话执行该 Todo。
export const TodoDispatchDialog = ({ open, todo, projects, onClose, onConfirm }: Props) => {
  const t = useT();
  const [projectPath, setProjectPath] = useState('');
  const [cliTool, setCliTool] = useState('claude');

  // 预填：scope=project 时用 scopeRef 作目标项目路径；否则首个已登记项目。
  useEffect(() => {
    if (!open || !todo) return;
    const preset =
      todo.scope === 'project' && todo.scopeRef ? todo.scopeRef : (projects[0]?.path ?? '');
    setProjectPath(preset);
    setCliTool('claude');
  }, [open, todo, projects]);

  const handleConfirm = (): void => {
    const project = projects.find((p) => p.path === projectPath);
    if (project) onConfirm(project, cliTool);
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>{t('todo.dispatch.title')}</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
        <Typography variant="body2" color="text.secondary" noWrap>
          {todo?.title}
        </Typography>
        <TextField
          select
          size="small"
          label={t('todo.dispatch.project')}
          value={projectPath}
          onChange={(e) => setProjectPath(e.target.value)}
          fullWidth
        >
          {projects.length === 0 ? (
            <MenuItem value="" disabled>
              {t('todo.dispatch.noProject')}
            </MenuItem>
          ) : (
            projects.map((p) => (
              <MenuItem key={p.path} value={p.path}>
                {p.alias || p.name}
              </MenuItem>
            ))
          )}
        </TextField>
        <TextField
          select
          size="small"
          label={t('todo.dispatch.cliTool')}
          value={cliTool}
          onChange={(e) => setCliTool(e.target.value)}
          fullWidth
        >
          {CLI_TOOLS.map((c) => (
            <MenuItem key={c} value={c}>
              {c}
            </MenuItem>
          ))}
        </TextField>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} sx={{ textTransform: 'none' }}>
          {t('todo.dispatch.cancel')}
        </Button>
        <Button
          variant="contained"
          startIcon={<PlayArrowRoundedIcon fontSize="small" />}
          disabled={!projectPath}
          onClick={handleConfirm}
          sx={{ borderRadius: 6, textTransform: 'none' }}
        >
          {t('todo.dispatch.confirm')}
        </Button>
      </DialogActions>
    </Dialog>
  );
};
