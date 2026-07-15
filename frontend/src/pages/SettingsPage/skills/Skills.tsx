import { Box, Button, IconButton, Switch, Tooltip, Typography, useTheme } from '@mui/material';
import FolderOpenRoundedIcon from '@mui/icons-material/FolderOpenRounded';
import GetAppRoundedIcon from '@mui/icons-material/GetAppRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import { useT } from '@/i18n';
import { NativeDialogs, type AgentSkill } from '@/services';
import { skillsStyles } from './Skills.styles';
import { useSkills } from './useSkills';
import { settingsPageStyles } from '../SettingsPage.styles';

// Skills 管理子页面：Skill 以 XK-Pane 为唯一事实源（SSOT），按每行的两个开关分别投影到
// Claude Code（~/.claude/skills）与 Codex（~/.codex/skills）。删除会把 SSOT 目录挪进回收
// 目录而非直接销毁。
//
// 文案 namespace：settings.skills.*。
export const Skills = () => {
  const theme = useTheme();
  const shared = settingsPageStyles(theme);
  const styles = skillsStyles(theme);
  const vm = useSkills();
  const t = useT();

  const errorTitle = t('settings.skills.error.title');

  const handleImportFolder = async (): Promise<void> => {
    const dir = await NativeDialogs.openDirectory({ title: t('settings.skills.importFolderDialogTitle') });
    if (!dir) return;
    await vm.importFromFolder(dir, { errorTitle });
  };

  const handleImportApps = (): void => {
    void vm.importFromApps({
      errorTitle,
      doneTitle: t('settings.skills.importDone.title'),
      doneMessage: (count) => t('settings.skills.importDone.message', { count }),
    });
  };

  const handleToggle = (s: AgentSkill, key: 'enabledClaude' | 'enabledCodex', value: boolean): void => {
    void vm.toggleEnabled(s, key, value, { errorTitle });
  };

  const handleRemove = (s: AgentSkill): void => {
    void vm.remove(s, {
      confirmTitle: t('settings.skills.removeConfirm.title'),
      confirmMessage: t('settings.skills.removeConfirm.message', { name: s.name }),
      okLabel: t('settings.skills.removeConfirm.ok'),
      cancelLabel: t('settings.skills.removeConfirm.cancel'),
      errorTitle,
    });
  };

  return (
    <Box sx={shared.section}>
      <Box sx={shared.sectionHeader}>
        <Typography sx={shared.sectionTitle}>{t('settings.skills.title')}</Typography>
        <Typography sx={shared.sectionHint}>{t('settings.skills.hint')}</Typography>
      </Box>

      <Box sx={styles.root}>
        <Box sx={styles.toolbar}>
          <Button
            size="small"
            variant="outlined"
            startIcon={<FolderOpenRoundedIcon />}
            onClick={() => void handleImportFolder()}
            sx={styles.toolbarBtn}
          >
            {t('settings.skills.importFolder')}
          </Button>
          <Button
            size="small"
            variant="outlined"
            startIcon={<GetAppRoundedIcon />}
            onClick={handleImportApps}
            sx={styles.toolbarBtn}
          >
            {t('settings.skills.importApps')}
          </Button>
        </Box>

        <Box sx={styles.list}>
          {!vm.loading && vm.skills.length === 0 && <Box sx={styles.empty}>{t('settings.skills.empty')}</Box>}

          {vm.skills.map((s) => (
            <Box key={s.id} sx={styles.row}>
              <Box sx={styles.rowMain}>
                <Box sx={styles.rowTitleLine}>
                  <Typography sx={styles.rowName}>{s.name}</Typography>
                  <Typography sx={styles.rowId}>{s.id}</Typography>
                </Box>
                {s.description && (
                  <Tooltip title={s.description}>
                    <Typography sx={styles.rowDescription}>{s.description}</Typography>
                  </Tooltip>
                )}
              </Box>

              <Box sx={styles.rowToggles}>
                <Box sx={styles.toggleItem}>
                  <Typography sx={styles.toggleLabel}>{t('settings.skills.toggles.claude')}</Typography>
                  <Switch
                    size="small"
                    checked={s.enabledClaude}
                    onChange={(e) => handleToggle(s, 'enabledClaude', e.target.checked)}
                  />
                </Box>
                <Box sx={styles.toggleItem}>
                  <Typography sx={styles.toggleLabel}>{t('settings.skills.toggles.codex')}</Typography>
                  <Switch
                    size="small"
                    checked={s.enabledCodex}
                    onChange={(e) => handleToggle(s, 'enabledCodex', e.target.checked)}
                  />
                </Box>
              </Box>

              <Box sx={styles.rowActions}>
                <Tooltip title={t('settings.skills.remove')}>
                  <IconButton
                    size="small"
                    aria-label={t('settings.skills.remove')}
                    onClick={() => handleRemove(s)}
                  >
                    <DeleteOutlineRoundedIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
              </Box>
            </Box>
          ))}
        </Box>
      </Box>
    </Box>
  );
};
