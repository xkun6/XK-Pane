import { Box, Button, CircularProgress, Tab, Tabs, Typography, useTheme } from '@mui/material';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import SaveRoundedIcon from '@mui/icons-material/SaveRounded';
import { useT } from '@/i18n';
import { CodeEditor } from '@/components/CodeEditor';
import { instructionsStyles } from './Instructions.styles';
import { useInstructions, type InstructionApp } from './useInstructions';
import { settingsPageStyles } from '../SettingsPage.styles';

// 全局指令子页面：快捷编辑 Claude Code 的 ~/.claude/CLAUDE.md 与 Codex 的
// ~/.codex/AGENTS.md，Ctrl+S 或点「保存」即写盘（首次写入前自动 .xkpane-bak 备份）。
//
// 文案 namespace：settings.instructions.*。
const FILE_NAME: Record<InstructionApp, string> = { claude: 'CLAUDE.md', codex: 'AGENTS.md' };

export const Instructions = () => {
  const theme = useTheme();
  const shared = settingsPageStyles(theme);
  const styles = instructionsStyles(theme);
  const vm = useInstructions();
  const t = useT();

  const errorTitle = t('settings.instructions.error.title');

  const handleSave = (): void => {
    void vm.save({ errorTitle });
  };

  const handleReload = (): void => {
    void vm.reload({
      confirmTitle: t('settings.instructions.reloadConfirm.title'),
      confirmMessage: t('settings.instructions.reloadConfirm.message'),
      okLabel: t('settings.instructions.reloadConfirm.ok'),
      cancelLabel: t('settings.instructions.reloadConfirm.cancel'),
      errorTitle,
    });
  };

  return (
    <Box sx={shared.section}>
      <Box sx={shared.sectionHeader}>
        <Typography sx={shared.sectionTitle}>{t('settings.instructions.title')}</Typography>
        <Typography sx={shared.sectionHint}>{t('settings.instructions.hint')}</Typography>
      </Box>

      <Box sx={styles.root}>
        <Tabs value={vm.tab} onChange={(_, v: InstructionApp) => vm.setTab(v)} sx={styles.tabs}>
          <Tab value="claude" label={t('settings.instructions.tabs.claude')} sx={styles.tab} />
          <Tab value="codex" label={t('settings.instructions.tabs.codex')} sx={styles.tab} />
        </Tabs>

        <Box sx={styles.toolbar}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <Typography sx={styles.pathLabel}>{t(`settings.instructions.pathLabel.${vm.tab}`)}</Typography>
            {vm.dirty && <Typography sx={styles.unsavedBadge}>{t('settings.instructions.unsaved')}</Typography>}
          </Box>
          <Box sx={styles.toolbarActions}>
            <Button
              size="small"
              variant="outlined"
              startIcon={<RefreshRoundedIcon />}
              onClick={handleReload}
              disabled={vm.loading || vm.saving}
            >
              {t('settings.instructions.reload')}
            </Button>
            <Button
              size="small"
              variant="contained"
              startIcon={vm.saving ? <CircularProgress size={14} color="inherit" /> : <SaveRoundedIcon />}
              onClick={handleSave}
              disabled={!vm.dirty || vm.saving}
            >
              {t('settings.instructions.save')}
            </Button>
          </Box>
        </Box>

        {vm.loadError && <Typography sx={styles.loadErrorText}>{t('settings.instructions.loadError')}</Typography>}

        <Box sx={styles.editorContainer}>
          <CodeEditor path={FILE_NAME[vm.tab]} value={vm.content} onChange={vm.setContent} onSave={handleSave} />
        </Box>
      </Box>
    </Box>
  );
};
