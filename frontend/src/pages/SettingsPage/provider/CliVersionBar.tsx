import { Box, Button, Chip, CircularProgress, IconButton, Tooltip, Typography, useTheme } from '@mui/material';
import SystemUpdateAltRoundedIcon from '@mui/icons-material/SystemUpdateAltRounded';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import DownloadRoundedIcon from '@mui/icons-material/DownloadRounded';
import { useT } from '@/i18n';
import type { AgentAppType } from '@/services';
import { cliVersionBarStyles } from './CliVersionBar.styles';
import { useCliVersionBar } from './useCliVersionBar';

export interface CliVersionBarProps {
  appType: AgentAppType;
}

// Provider 设置页 Claude/Codex 标签顶部的 CLI 版本条：本地版本 + npm 最新版检测，
// 可发现更新时给出一键升级按钮（借鉴 cc-switch 的本地环境检查）。
export const CliVersionBar = ({ appType }: CliVersionBarProps) => {
  const theme = useTheme();
  const styles = cliVersionBarStyles(theme);
  const t = useT();
  const vm = useCliVersionBar(appType);
  const status = vm.status;

  const handleUpgrade = (): void => {
    void vm.upgrade({
      doneTitle: t('settings.provider.cliUpdate.doneTitle'),
      doneMessage: (version: string) => t('settings.provider.cliUpdate.doneMessage', { version }),
      warnTitle: t('settings.provider.cliUpdate.warnTitle'),
      failTitle: t('settings.provider.cliUpdate.failTitle'),
    });
  };

  const handleInstall = (): void => {
    void vm.install({
      doneTitle: t('settings.provider.cliUpdate.installDoneTitle'),
      doneMessage: (version: string) => t('settings.provider.cliUpdate.installDoneMessage', { version }),
      warnTitle: t('settings.provider.cliUpdate.installWarnTitle'),
      failTitle: t('settings.provider.cliUpdate.installFailTitle'),
    });
  };

  // 第二行小字：checking 且没有旧结果时显示"检测中"；有 status 时拼「当前 vX · 最新 vY」
  // （任一侧未知就填 versionUnknown，避免出现空白占位）；detail 非空再附加显示。
  let subLine = '';
  if (vm.checking && !status) {
    subLine = t('settings.provider.cliUpdate.checking');
  } else if (status) {
    const hasCurrent = status.installed && status.currentVersion !== '';
    const hasLatest = status.latestVersion !== '';
    if (hasCurrent || hasLatest) {
      const current = hasCurrent ? status.currentVersion : t('settings.provider.cliUpdate.versionUnknown');
      const latest = hasLatest ? status.latestVersion : t('settings.provider.cliUpdate.versionUnknown');
      subLine = t('settings.provider.cliUpdate.versionLine', { current, latest });
    }
    if (status.detail) {
      subLine = subLine ? `${subLine} · ${status.detail}` : status.detail;
    }
  }

  return (
    <Box sx={styles.root}>
      <SystemUpdateAltRoundedIcon fontSize="small" sx={styles.icon} aria-hidden />
      <Box sx={styles.info}>
        <Box sx={styles.titleLine}>
          <Typography sx={styles.name}>{t(`settings.provider.cliUpdate.name.${appType}`)}</Typography>
          {status?.updateAvailable && (
            <Chip
              label={t('settings.provider.cliUpdate.updateAvailable')}
              size="small"
              variant="outlined"
              sx={styles.chipUpdateAvailable}
            />
          )}
          {status && !status.updateAvailable && status.installed && status.latestVersion !== '' && (
            <Chip
              label={t('settings.provider.cliUpdate.upToDate')}
              size="small"
              variant="outlined"
              sx={styles.chipUpToDate}
            />
          )}
          {status && !status.installed && (
            <Chip
              label={t('settings.provider.cliUpdate.notInstalled')}
              size="small"
              variant="outlined"
              sx={styles.chipNotInstalled}
            />
          )}
        </Box>
        {subLine !== '' && <Typography sx={styles.subLine}>{subLine}</Typography>}
      </Box>

      <Box sx={styles.actions}>
        <Tooltip title={t('settings.provider.cliUpdate.refresh')}>
          <span>
            <IconButton
              size="small"
              aria-label={t('settings.provider.cliUpdate.refresh')}
              disabled={vm.checking || vm.upgrading || vm.installing}
              onClick={() => void vm.refresh()}
            >
              <RefreshRoundedIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
        {status?.updateAvailable && (
          <Button
            size="small"
            variant="contained"
            disabled={vm.upgrading}
            startIcon={vm.upgrading ? <CircularProgress size={14} color="inherit" /> : undefined}
            onClick={handleUpgrade}
            sx={styles.upgradeBtn}
          >
            {t('settings.provider.cliUpdate.upgrade')}
          </Button>
        )}
        {status && !status.installed && status.installable && (
          <Button
            size="small"
            variant="contained"
            disabled={vm.installing}
            startIcon={
              vm.installing ? <CircularProgress size={14} color="inherit" /> : <DownloadRoundedIcon fontSize="small" />
            }
            onClick={handleInstall}
            sx={styles.upgradeBtn}
          >
            {t('settings.provider.cliUpdate.install')}
          </Button>
        )}
      </Box>
    </Box>
  );
};
