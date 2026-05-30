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
import AccountTreeRoundedIcon from '@mui/icons-material/AccountTreeRounded';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import CloudDownloadRoundedIcon from '@mui/icons-material/CloudDownloadRounded';
import SouthRoundedIcon from '@mui/icons-material/SouthRounded';
import NorthRoundedIcon from '@mui/icons-material/NorthRounded';
import Inventory2RoundedIcon from '@mui/icons-material/Inventory2Rounded';
import UnarchiveRoundedIcon from '@mui/icons-material/UnarchiveRounded';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import { useT } from '@/i18n';
import type { GitWorktree } from '@/services';
import { gitPageStyles } from './GitPage.styles';
import { useGitPage } from './useGitPage';

// GitPage：选一个已登记项目 → 只读展示其 Git 状态（分支 + 脏标 + 本地分支 + 最近提交）。
// 端到端验证 gitsvc 往返（GetStatus / Branches / Log）。变更操作（worktree/stash/网络）留后续单元。

export const GitPage = () => {
  const theme = useTheme();
  const styles = gitPageStyles(theme);
  const t = useT();
  const vm = useGitPage();

  const errorTitle = t('git.error.title');
  const toolbarDisabled = vm.busy || vm.loading;

  const onFetch = (): void =>
    void vm.fetch({ successTitle: t('git.ops.fetch.title'), successMessage: t('git.ops.fetch.done'), errorTitle });
  const onPull = (): void =>
    void vm.pull({ successTitle: t('git.ops.pull.title'), successMessage: t('git.ops.pull.done'), errorTitle });
  const onStash = (): void =>
    void vm.stash({ successTitle: t('git.ops.stash.title'), successMessage: t('git.ops.stash.done'), errorTitle });
  const onStashPop = (): void =>
    void vm.stashPop({
      successTitle: t('git.ops.stashPop.title'),
      successMessage: t('git.ops.stashPop.done'),
      errorTitle,
    });
  const onPush = (): void =>
    void vm.push({
      successTitle: t('git.ops.push.title'),
      successMessage: t('git.ops.push.done'),
      errorTitle,
      confirmTitle: t('git.pushConfirm.title'),
      confirmMessage: t('git.pushConfirm.message'),
      okLabel: t('git.pushConfirm.ok'),
      cancelLabel: t('git.pushConfirm.cancel'),
    });
  const onAddWorktree = (): void =>
    void vm.addWorktree({
      pickerTitle: t('git.worktree.pickerTitle'),
      successTitle: t('git.worktree.addedTitle'),
      successMessage: t('git.worktree.addedMessage'),
      errorTitle,
    });
  const onRemoveWorktree = (wt: GitWorktree): void =>
    void vm.removeWorktree(wt, {
      confirmTitle: t('git.worktree.removeConfirm.title'),
      confirmMessage: t('git.worktree.removeConfirm.message', { path: wt.path }),
      okLabel: t('git.worktree.removeConfirm.ok'),
      cancelLabel: t('git.worktree.removeConfirm.cancel'),
      successTitle: t('git.worktree.removedTitle'),
      successMessage: t('git.worktree.removedMessage'),
      errorTitle,
    });

  return (
    <Box sx={styles.root}>
      <Box sx={styles.header}>
        <Typography sx={styles.eyebrow}>{t('git.eyebrow')}</Typography>
        <Typography sx={styles.title}>{t('git.title')}</Typography>
      </Box>

      {vm.projects.length === 0 ? (
        <Box sx={styles.empty}>{t('git.noProjects')}</Box>
      ) : (
        <>
          <Box sx={styles.pickerRow}>
            <TextField
              select
              size="small"
              fullWidth
              label={t('git.project')}
              value={vm.selectedPath}
              onChange={(e) => void vm.select(e.target.value, errorTitle)}
            >
              {vm.projects.map((p) => (
                <MenuItem key={p.id} value={p.path}>
                  {p.alias ?? p.name}
                </MenuItem>
              ))}
            </TextField>
            <Tooltip title={t('git.refresh')}>
              <span>
                <IconButton
                  aria-label={t('git.refresh')}
                  onClick={() => void vm.refresh(errorTitle)}
                  disabled={!vm.selectedPath || vm.loading}
                >
                  <RefreshRoundedIcon />
                </IconButton>
              </span>
            </Tooltip>
          </Box>

          {vm.loading && <Box sx={styles.empty}>{t('git.loading')}</Box>}

          {!vm.loading && vm.selectedPath && vm.status && (
            <>
              {!vm.status.isRepo ? (
                <Box sx={styles.statusCard}>
                  <AccountTreeRoundedIcon fontSize="small" sx={{ color: theme.palette.foundation.text.muted }} />
                  <Typography sx={{ color: theme.palette.foundation.text.secondary }}>
                    {t('git.notARepo')}
                  </Typography>
                </Box>
              ) : (
                <>
                  <Box sx={styles.statusCard}>
                    <AccountTreeRoundedIcon fontSize="small" sx={{ color: theme.palette.foundation.accent }} />
                    <Typography sx={styles.branchName}>{vm.status.branch || t('git.detached')}</Typography>
                    <Chip
                      label={vm.status.isDirty ? t('git.dirty') : t('git.clean')}
                      size="small"
                      variant="outlined"
                      sx={{
                        borderRadius: 6,
                        color: vm.status.isDirty
                          ? theme.palette.foundation.status.warning
                          : theme.palette.foundation.status.success,
                        borderColor: vm.status.isDirty
                          ? theme.palette.foundation.status.warning
                          : theme.palette.foundation.status.success,
                      }}
                    />
                  </Box>

                  <Box sx={styles.toolbar}>
                    <Button
                      size="small"
                      variant="outlined"
                      startIcon={<CloudDownloadRoundedIcon />}
                      onClick={onFetch}
                      disabled={toolbarDisabled}
                      sx={{ textTransform: 'none' }}
                    >
                      {t('git.actions.fetch')}
                    </Button>
                    <Button
                      size="small"
                      variant="outlined"
                      startIcon={<SouthRoundedIcon />}
                      onClick={onPull}
                      disabled={toolbarDisabled}
                      sx={{ textTransform: 'none' }}
                    >
                      {t('git.actions.pull')}
                    </Button>
                    <Button
                      size="small"
                      variant="outlined"
                      startIcon={<NorthRoundedIcon />}
                      onClick={onPush}
                      disabled={toolbarDisabled}
                      sx={{ textTransform: 'none' }}
                    >
                      {t('git.actions.push')}
                    </Button>
                    <Button
                      size="small"
                      variant="outlined"
                      startIcon={<Inventory2RoundedIcon />}
                      onClick={onStash}
                      disabled={toolbarDisabled}
                      sx={{ textTransform: 'none' }}
                    >
                      {t('git.actions.stash')}
                    </Button>
                    <Button
                      size="small"
                      variant="outlined"
                      startIcon={<UnarchiveRoundedIcon />}
                      onClick={onStashPop}
                      disabled={toolbarDisabled}
                      sx={{ textTransform: 'none' }}
                    >
                      {t('git.actions.stashPop')}
                    </Button>
                  </Box>

                  <Box sx={styles.section}>
                    <Typography sx={styles.sectionTitle}>
                      {t('git.branches', { count: String(vm.branches.length) })}
                    </Typography>
                    <Box sx={styles.branchList}>
                      {vm.branches.map((b) => (
                        <Chip
                          key={b.name}
                          label={b.name}
                          size="small"
                          variant={b.isCurrent ? 'filled' : 'outlined'}
                          color={b.isCurrent ? 'primary' : 'default'}
                          sx={{ borderRadius: 6, fontFamily: 'monospace' }}
                        />
                      ))}
                    </Box>
                  </Box>

                  <Box sx={styles.section}>
                    <Typography sx={styles.sectionTitle}>{t('git.commits')}</Typography>
                    {vm.commits.length === 0 ? (
                      <Box sx={styles.empty}>{t('git.noCommits')}</Box>
                    ) : (
                      <Box sx={styles.commitList}>
                        {vm.commits.map((c) => (
                          <Box key={c.hash} sx={styles.commitRow}>
                            <Typography component="span" sx={styles.commitHash}>
                              {c.shortHash}
                            </Typography>
                            <Typography component="span" sx={styles.commitSubject}>
                              {c.subject}
                            </Typography>
                            <Typography component="span" sx={styles.commitMeta}>
                              {c.author}
                            </Typography>
                          </Box>
                        ))}
                      </Box>
                    )}
                  </Box>

                  <Box sx={styles.section}>
                    <Typography sx={styles.sectionTitle}>
                      {t('git.worktree.title', { count: String(vm.worktrees.length) })}
                    </Typography>
                    <Box sx={styles.worktreeAddRow}>
                      <TextField
                        size="small"
                        fullWidth
                        placeholder={t('git.worktree.namePlaceholder')}
                        value={vm.newWorktreeName}
                        onChange={(e) => vm.setNewWorktreeName(e.target.value)}
                        disabled={toolbarDisabled}
                      />
                      <Button
                        size="small"
                        variant="outlined"
                        startIcon={<AddRoundedIcon />}
                        onClick={onAddWorktree}
                        disabled={toolbarDisabled || !vm.newWorktreeName.trim()}
                        sx={{ textTransform: 'none', whiteSpace: 'nowrap' }}
                      >
                        {t('git.worktree.add')}
                      </Button>
                    </Box>
                    <Box sx={styles.worktreeList}>
                      {vm.worktrees.map((wt) => (
                        <Box key={wt.path} sx={styles.worktreeRow}>
                          <Typography component="span" sx={styles.worktreePath} title={wt.path}>
                            {wt.path}
                          </Typography>
                          {wt.branch && (
                            <Chip
                              label={wt.branch}
                              size="small"
                              variant="outlined"
                              sx={{ borderRadius: 6, fontFamily: 'monospace' }}
                            />
                          )}
                          <Typography component="span" sx={styles.commitHash}>
                            {wt.commit}
                          </Typography>
                          {wt.isMain ? (
                            <Chip
                              label={t('git.worktree.main')}
                              size="small"
                              color="primary"
                              sx={{ borderRadius: 6 }}
                            />
                          ) : (
                            <Tooltip title={t('git.worktree.remove')}>
                              <span>
                                <IconButton
                                  size="small"
                                  aria-label={t('git.worktree.remove')}
                                  onClick={() => onRemoveWorktree(wt)}
                                  disabled={toolbarDisabled}
                                >
                                  <DeleteOutlineRoundedIcon fontSize="small" />
                                </IconButton>
                              </span>
                            </Tooltip>
                          )}
                        </Box>
                      ))}
                    </Box>
                  </Box>
                </>
              )}
            </>
          )}
        </>
      )}
    </Box>
  );
};
