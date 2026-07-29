import { Box, Button, Chip, IconButton, Tooltip, Typography, useTheme } from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import GetAppRoundedIcon from '@mui/icons-material/GetAppRounded';
import EditRoundedIcon from '@mui/icons-material/EditRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import SwapHorizRoundedIcon from '@mui/icons-material/SwapHorizRounded';
import CheckCircleRoundedIcon from '@mui/icons-material/CheckCircleRounded';
import TuneRoundedIcon from '@mui/icons-material/TuneRounded';
import { useT } from '@/i18n';
import type { AgentAppType, AgentProvider } from '@/services';
import { extractBaseUrlHint } from './agentProviderDisplay';
import { globalProviderPanelStyles } from './GlobalProviderPanel.styles';
import { GlobalProviderFormDialog } from './GlobalProviderFormDialog';
import { SystemConfigDialog } from './SystemConfigDialog';
import { CliVersionBar } from './CliVersionBar';
import { useGlobalProviderPanel, type ClaudeTemplateId } from './useGlobalProviderPanel';

export interface GlobalProviderPanelProps {
  appType: AgentAppType;
}

// cc-switch 式全局配置 Provider 面板：claude / codex 各一份实例（appType 区分）。
// 列表 + 新增/编辑（对话框）+ 删除 + 切换 + 从当前 live 导入。
export const GlobalProviderPanel = ({ appType }: GlobalProviderPanelProps) => {
  const theme = useTheme();
  const styles = globalProviderPanelStyles(theme);
  const t = useT();
  const vm = useGlobalProviderPanel(appType);

  const errorTitle = t('settings.provider.error.title');

  const handleSwitch = (p: AgentProvider): void => {
    void vm.switchTo(p, { errorTitle });
  };
  const handleRemove = (p: AgentProvider): void => {
    void vm.remove(p, {
      confirmTitle: t('settings.provider.global.removeConfirm.title'),
      confirmMessage: t('settings.provider.global.removeConfirm.message', { name: p.name }),
      okLabel: t('settings.provider.global.removeConfirm.ok'),
      cancelLabel: t('settings.provider.global.removeConfirm.cancel'),
      errorTitle,
    });
  };
  const handleImportCurrent = (): void => {
    void vm.importCurrent({ errorTitle });
  };
  const handleSubmit = (): void => {
    void vm.submit({ errorTitle });
  };
  const handleApplyTemplate = (template: ClaudeTemplateId): void => {
    void vm.applyTemplate(template, {
      confirmTitle: t('settings.provider.global.form.templateOverwriteConfirm.title'),
      confirmMessage: t('settings.provider.global.form.templateOverwriteConfirm.message'),
      okLabel: t('settings.provider.global.form.templateOverwriteConfirm.ok'),
      cancelLabel: t('settings.provider.global.form.templateOverwriteConfirm.cancel'),
    });
  };

  return (
    <Box sx={styles.root}>
      <CliVersionBar appType={appType} />

      {vm.isTwoLayer && (
        <Box sx={styles.baseCard}>
          <Box sx={styles.baseCardMain}>
            <Typography sx={styles.baseCardTitle}>{t('settings.provider.global.baseConfig.title')}</Typography>
            <Typography sx={styles.baseCardDesc}>{t('settings.provider.global.baseConfig.desc')}</Typography>
          </Box>
          <Button
            size="small"
            variant="outlined"
            startIcon={<TuneRoundedIcon />}
            onClick={vm.openBaseEdit}
            sx={styles.toolbarBtn}
          >
            {t('settings.provider.global.baseConfig.edit')}
          </Button>
        </Box>
      )}

      <Box sx={styles.toolbar}>
        <Button
          size="small"
          variant="outlined"
          startIcon={<GetAppRoundedIcon />}
          onClick={handleImportCurrent}
          sx={styles.toolbarBtn}
        >
          {t('settings.provider.global.importCurrent')}
        </Button>
        <Button
          size="small"
          variant="outlined"
          startIcon={<AddRoundedIcon />}
          onClick={vm.openAdd}
          sx={styles.toolbarBtn}
        >
          {t('settings.provider.global.add')}
        </Button>
      </Box>

      <Box sx={styles.list}>
        {!vm.loading && vm.providers.length === 0 && (
          <Box sx={styles.empty}>{t('settings.provider.global.empty')}</Box>
        )}

        {vm.providers.map((p) => {
          const baseUrlHint = extractBaseUrlHint(appType, p.settingsJson);
          return (
            <Box key={p.id} sx={p.isCurrent ? styles.rowCurrent : styles.row}>
              <Box sx={styles.rowMain}>
                <Box sx={styles.rowTitleLine}>
                  {p.isCurrent && (
                    <CheckCircleRoundedIcon fontSize="small" sx={styles.currentIcon} aria-hidden />
                  )}
                  <Typography sx={styles.rowName}>{p.name}</Typography>
                  <Chip
                    label={t(`settings.provider.global.category.${p.category}`)}
                    size="small"
                    variant="outlined"
                    sx={styles.categoryChip}
                  />
                  {p.isCurrent && (
                    <Chip
                      label={t('settings.provider.global.currentBadge')}
                      size="small"
                      variant="outlined"
                      sx={styles.currentChip}
                    />
                  )}
                </Box>
                {baseUrlHint && <Typography sx={styles.rowSub}>{baseUrlHint}</Typography>}
              </Box>

              <Box sx={styles.rowActions}>
                <Tooltip title={t('settings.provider.global.actions.switch')}>
                  <span>
                    <IconButton
                      size="small"
                      aria-label={t('settings.provider.global.actions.switch')}
                      disabled={p.isCurrent}
                      onClick={() => handleSwitch(p)}
                    >
                      <SwapHorizRoundedIcon fontSize="small" />
                    </IconButton>
                  </span>
                </Tooltip>
                <Tooltip title={t('settings.provider.global.actions.edit')}>
                  <IconButton
                    size="small"
                    aria-label={t('settings.provider.global.actions.edit')}
                    onClick={() => vm.openEdit(p)}
                  >
                    <EditRoundedIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Tooltip title={t('settings.provider.global.actions.remove')}>
                  <span>
                    <IconButton
                      size="small"
                      aria-label={t('settings.provider.global.actions.remove')}
                      disabled={p.isCurrent}
                      onClick={() => handleRemove(p)}
                    >
                      <DeleteOutlineRoundedIcon fontSize="small" />
                    </IconButton>
                  </span>
                </Tooltip>
              </Box>
            </Box>
          );
        })}
      </Box>

      <Typography sx={styles.bottomHint}>{t('settings.provider.global.hint')}</Typography>

      <GlobalProviderFormDialog
        appType={appType}
        open={vm.formOpen}
        editingId={vm.editingId}
        editingIsCurrent={vm.editingIsCurrent}
        draft={vm.draft}
        claudeJsonError={vm.claudeJsonError}
        codexAuthJsonError={vm.codexAuthJsonError}
        onFieldChange={vm.setField}
        onApplyTemplate={handleApplyTemplate}
        onCancel={vm.cancel}
        onSubmit={handleSubmit}
      />

      <SystemConfigDialog
        appType={appType}
        open={vm.baseDialogOpen}
        value={vm.baseJson}
        error={vm.baseJsonError}
        onChange={vm.setBaseJson}
        onCancel={vm.cancelBaseEdit}
        onSave={() => void vm.saveBase({ errorTitle })}
      />
    </Box>
  );
};
