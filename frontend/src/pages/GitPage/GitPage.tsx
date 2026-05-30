import {
  Box,
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
import { useT } from '@/i18n';
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
                </>
              )}
            </>
          )}
        </>
      )}
    </Box>
  );
};
