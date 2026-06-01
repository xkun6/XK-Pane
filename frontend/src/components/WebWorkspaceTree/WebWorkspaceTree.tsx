import { type KeyboardEvent } from 'react';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  IconButton,
  InputLabel,
  ListItemText,
  Menu,
  MenuItem,
  Select,
  Snackbar,
  TextField,
  Typography,
  useTheme,
} from '@mui/material';
import MoreVertRoundedIcon from '@mui/icons-material/MoreVertRounded';
import ChevronRightRoundedIcon from '@mui/icons-material/ChevronRightRounded';
import FolderRoundedIcon from '@mui/icons-material/FolderRounded';
import FolderOpenRoundedIcon from '@mui/icons-material/FolderOpenRounded';
import SmartToyRoundedIcon from '@mui/icons-material/SmartToyRounded';
import AutoAwesomeRoundedIcon from '@mui/icons-material/AutoAwesomeRounded';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import RocketLaunchRoundedIcon from '@mui/icons-material/RocketLaunchRounded';
import { useT } from '@/i18n';
import type { SessionInfo, Workspace, WorkspaceProject } from '@/services';
import { webWorkspaceTreeStyles } from './WebWorkspaceTree.styles';
import { useWebWorkspaceTree, type NoticeKey } from './useWebWorkspaceTree';

type T = (k: string, p?: Record<string, string>) => string;

function basename(path: string): string {
  return path.split(/[/\\]/).filter(Boolean).pop() || path;
}

function wsLabel(w: Workspace): string {
  return w.alias || w.name;
}

function projectLabel(p: WorkspaceProject): string {
  return p.alias || basename(p.path);
}

function relTime(modifiedAt: number, t: T): string {
  const sec = Math.max(0, Math.floor(Date.now() / 1000 - modifiedAt));
  if (sec < 60) return t('webWorkspaceTree.sessions.time.justNow');
  const min = Math.floor(sec / 60);
  if (min < 60) return t('webWorkspaceTree.sessions.time.minutes', { n: String(min) });
  const hr = Math.floor(min / 60);
  if (hr < 24) return t('webWorkspaceTree.sessions.time.hours', { n: String(hr) });
  return t('webWorkspaceTree.sessions.time.days', { n: String(Math.floor(hr / 24)) });
}

const noticeSeverity: Record<NoticeKey, 'success' | 'warning' | 'error'> = {
  copied: 'success',
  copyFailed: 'warning',
  resumeFailed: 'error',
};

export interface WebWorkspaceTreeProps {
  // 选中会话（attach/恢复/新建）后回调 —— 手机端用来自动收起承载本栏的抽屉。
  onSelect?: () => void;
}

// WebWorkspaceTree：仅 Web（viewer）渲染的左栏，复刻桌面 WorkspaceTree 的「按工作区浏览 +
// 恢复历史会话 + 新建会话」。移动端无右键 → 行尾 ⋮ 按钮锚定 MUI Menu（桌面/手机一套）。
// 写类操作（改名/删除/拖拽）不在 viewer 暴露（fail-closed）；恢复/新建走 /api/launch。
export const WebWorkspaceTree = ({ onSelect }: WebWorkspaceTreeProps = {}) => {
  const theme = useTheme();
  const styles = webWorkspaceTreeStyles(theme);
  const t = useT();
  const vm = useWebWorkspaceTree({ onSelect });

  const onRowKey = (fn: () => void) => (e: KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      fn();
    }
  };

  const renderSessions = (project: WorkspaceProject) => {
    const state = vm.sessionStateFor(project.id);
    if (!state || state.status === 'loading') {
      return <Box sx={styles.subHint}>{t('webWorkspaceTree.sessions.loading')}</Box>;
    }
    if (state.status === 'error') {
      return <Box sx={styles.subHint}>{t('webWorkspaceTree.sessions.loadError')}</Box>;
    }
    if (state.sessions.length === 0) {
      return <Box sx={styles.subHint}>{t('webWorkspaceTree.sessions.empty')}</Box>;
    }
    return (
      <>
        {state.sessions.map((s) => (
          <Box
            key={s.filePath}
            sx={{ ...styles.row, ...styles.sessionRow }}
            role="button"
            tabIndex={0}
            title={s.description || s.id}
            onClick={() => vm.resume(s, project.path)}
            onKeyDown={onRowKey(() => vm.resume(s, project.path))}
          >
            {s.source === 'claude' ? (
              <SmartToyRoundedIcon sx={styles.sessionGlyph} />
            ) : (
              <AutoAwesomeRoundedIcon sx={styles.sessionGlyph} />
            )}
            <Box sx={styles.sessionDesc}>
              {s.description || t('webWorkspaceTree.sessions.noPrompt')}
            </Box>
            <Box component="span" sx={styles.sessionTime}>
              {relTime(s.modifiedAt, t)}
            </Box>
            <IconButton
              size="small"
              className="rowMore"
              sx={styles.moreBtn}
              aria-label={t('webWorkspaceTree.a11y.moreActions')}
              onClick={(e) => {
                e.stopPropagation();
                vm.openMenu({
                  kind: 'session',
                  anchorEl: e.currentTarget,
                  session: s,
                  projectPath: project.path,
                });
              }}
            >
              <MoreVertRoundedIcon fontSize="small" />
            </IconButton>
          </Box>
        ))}
      </>
    );
  };

  const renderProjects = (ws: Workspace) =>
    ws.projects.map((p) => {
      const open = vm.expandedProjectIds.has(p.id);
      return (
        <Box key={p.id}>
          <Box
            sx={{ ...styles.row, ...styles.projectRow }}
            role="button"
            tabIndex={0}
            title={p.path}
            onClick={() => vm.toggleProject(p)}
            onKeyDown={onRowKey(() => vm.toggleProject(p))}
          >
            <ChevronRightRoundedIcon
              sx={{ ...styles.chevron, ...(open ? styles.chevronOpen : {}) }}
            />
            <FolderOpenRoundedIcon sx={styles.rowGlyph} />
            <Box sx={styles.rowLabel}>{projectLabel(p)}</Box>
            <IconButton
              size="small"
              className="rowMore"
              sx={styles.moreBtn}
              aria-label={t('webWorkspaceTree.a11y.moreActions')}
              onClick={(e) => {
                e.stopPropagation();
                vm.openMenu({
                  kind: 'project',
                  anchorEl: e.currentTarget,
                  project: p,
                  label: projectLabel(p),
                });
              }}
            >
              <MoreVertRoundedIcon fontSize="small" />
            </IconButton>
          </Box>
          {open && renderSessions(p)}
        </Box>
      );
    });

  const sessionGlyph = (cliTool: string) => {
    if (cliTool === 'claude' || cliTool === 'reclaude')
      return <SmartToyRoundedIcon sx={styles.rowGlyph} />;
    if (cliTool === 'codex') return <AutoAwesomeRoundedIcon sx={styles.rowGlyph} />;
    return <TerminalRoundedIcon sx={styles.rowGlyph} />;
  };

  const renderActive = () => {
    if (vm.activeLoading) {
      return (
        <Box sx={styles.centerBox}>
          <CircularProgress size={20} />
        </Box>
      );
    }
    if (vm.activeFailed) {
      return <Box sx={styles.subHintFlush}>{t('webWorkspaceTree.active.loadError')}</Box>;
    }
    if (vm.activeSessions.length === 0) {
      return <Box sx={styles.subHintFlush}>{t('webWorkspaceTree.active.empty')}</Box>;
    }
    return vm.activeSessions.map((s: SessionInfo) => (
      <Box
        key={s.id}
        sx={{ ...styles.row, ...styles.wsRow }}
        role="button"
        tabIndex={0}
        title={s.cwd}
        onClick={() => vm.attach(s)}
        onKeyDown={onRowKey(() => vm.attach(s))}
      >
        {sessionGlyph(s.cliTool)}
        <Box sx={styles.rowLabel}>{basename(s.cwd)}</Box>
        {s.cliTool && s.cliTool !== 'none' && (
          <Box component="span" sx={styles.sessionTime}>
            {s.cliTool}
          </Box>
        )}
      </Box>
    ));
  };

  const renderTree = () => {
    if (vm.loading) {
      return (
        <Box sx={styles.centerBox}>
          <CircularProgress size={24} />
        </Box>
      );
    }
    if (vm.loadFailed) {
      return <Typography sx={styles.errorText}>{t('webWorkspaceTree.loadError')}</Typography>;
    }
    if (vm.workspaces.length === 0) {
      return <Typography sx={styles.emptyText}>{t('webWorkspaceTree.empty')}</Typography>;
    }
    return vm.workspaces.map((ws) => {
      const open = vm.expandedWorkspaceId === ws.id;
      return (
        <Box key={ws.id}>
          <Box
            sx={{ ...styles.row, ...styles.wsRow }}
            role="button"
            tabIndex={0}
            onClick={() => vm.toggleWorkspace(ws.id)}
            onKeyDown={onRowKey(() => vm.toggleWorkspace(ws.id))}
          >
            <ChevronRightRoundedIcon
              sx={{ ...styles.chevron, ...(open ? styles.chevronOpen : {}) }}
            />
            <FolderRoundedIcon sx={styles.rowGlyph} />
            <Box sx={{ ...styles.rowLabel, ...styles.wsLabel }}>{wsLabel(ws)}</Box>
            <Box component="span" sx={styles.countPill}>
              {ws.projects.length}
            </Box>
          </Box>
          {open && renderProjects(ws)}
        </Box>
      );
    });
  };

  return (
    <Box sx={styles.rail}>
      <Box sx={styles.header}>
        <Typography sx={styles.hint}>{t('webWorkspaceTree.rail.hint')}</Typography>
      </Box>
      <Box sx={styles.list}>
        <Box sx={styles.sectionHeader}>
          <Typography sx={styles.railTitle}>{t('webWorkspaceTree.active.title')}</Typography>
          <IconButton
            size="small"
            sx={styles.moreBtn}
            aria-label={t('webWorkspaceTree.active.refresh')}
            onClick={vm.reloadActive}
          >
            <RefreshRoundedIcon fontSize="small" />
          </IconButton>
        </Box>
        {renderActive()}

        <Divider sx={styles.sectionDivider} />

        <Typography sx={{ ...styles.railTitle, ...styles.workspacesHeader }}>
          {t('webWorkspaceTree.rail.title')}
        </Typography>
        {renderTree()}
      </Box>

      {/* 单实例锚定菜单：内容随 menu.kind 切换（project / session）。 */}
      <Menu
        anchorEl={vm.menu?.anchorEl ?? null}
        open={vm.menu !== null}
        onClose={vm.closeMenu}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        {vm.menu?.kind === 'project' && [
          <MenuItem
            key="new"
            onClick={() => {
              if (vm.menu?.kind === 'project') {
                vm.openNewSession(vm.menu.project.path, vm.menu.label);
              }
            }}
          >
            <ListItemText>{t('webWorkspaceTree.menu.newSession')}</ListItemText>
          </MenuItem>,
          <MenuItem
            key="copyPath"
            onClick={() => {
              if (vm.menu?.kind === 'project') vm.copyPath(vm.menu.project);
            }}
          >
            <ListItemText>{t('webWorkspaceTree.menu.copyPath')}</ListItemText>
          </MenuItem>,
        ]}
        {vm.menu?.kind === 'session' && [
          <MenuItem
            key="resume"
            onClick={() => {
              if (vm.menu?.kind === 'session') vm.resume(vm.menu.session, vm.menu.projectPath);
            }}
          >
            <ListItemText>{t('webWorkspaceTree.menu.resume')}</ListItemText>
          </MenuItem>,
          ...(vm.menu.session.source === 'claude'
            ? [
                <MenuItem
                  key="reclaude"
                  onClick={() => {
                    if (vm.menu?.kind === 'session') {
                      vm.resume(vm.menu.session, vm.menu.projectPath, 'reclaude');
                    }
                  }}
                >
                  <ListItemText>{t('webWorkspaceTree.menu.resumeWithReclaude')}</ListItemText>
                </MenuItem>,
              ]
            : []),
          <Divider key="div" />,
          <MenuItem
            key="copyId"
            onClick={() => {
              if (vm.menu?.kind === 'session') vm.copyId(vm.menu.session);
            }}
          >
            <ListItemText>{t('webWorkspaceTree.menu.copyId')}</ListItemText>
          </MenuItem>,
        ]}
      </Menu>

      {/* 新建会话对话框：项目固定（菜单目标），选 CLI + 输入 prompt → POST /api/launch。 */}
      <Dialog open={vm.dialog.open} onClose={vm.closeDialog} fullWidth maxWidth="xs">
        <DialogTitle>{t('webWorkspaceTree.dialog.title')}</DialogTitle>
        <DialogContent>
          <Typography sx={styles.dialogTarget}>
            {t('webWorkspaceTree.dialog.target')}: {vm.dialog.projectLabel}
          </Typography>

          <FormControl fullWidth sx={styles.dialogField} size="small">
            <InputLabel id="web-ws-cli">{t('webWorkspaceTree.dialog.cliTool')}</InputLabel>
            <Select
              labelId="web-ws-cli"
              label={t('webWorkspaceTree.dialog.cliTool')}
              value={vm.dialog.cliTool}
              onChange={(e) => vm.setCliTool(String(e.target.value))}
            >
              {vm.cliToolOptions.map((tool) => (
                <MenuItem key={tool} value={tool}>
                  {tool}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          <TextField
            fullWidth
            multiline
            minRows={3}
            size="small"
            sx={styles.dialogField}
            label={t('webWorkspaceTree.dialog.prompt')}
            placeholder={t('webWorkspaceTree.dialog.promptPlaceholder')}
            value={vm.dialog.prompt}
            onChange={(e) => vm.setPrompt(e.target.value)}
          />

          {vm.dialog.error && (
            <Typography sx={styles.dialogError}>
              {t('webWorkspaceTree.dialog.launchError')}: {vm.dialog.error}
            </Typography>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={vm.closeDialog}>{t('webWorkspaceTree.dialog.cancel')}</Button>
          <Button
            variant="contained"
            startIcon={<RocketLaunchRoundedIcon />}
            disabled={!vm.canLaunch}
            onClick={vm.launch}
          >
            {t('webWorkspaceTree.dialog.launch')}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar
        open={vm.notice !== null}
        autoHideDuration={2400}
        onClose={vm.clearNotice}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        {vm.notice ? (
          <Alert severity={noticeSeverity[vm.notice]} variant="filled" onClose={vm.clearNotice}>
            {t(`webWorkspaceTree.notice.${vm.notice}`)}
          </Alert>
        ) : (
          <span />
        )}
      </Snackbar>
    </Box>
  );
};
