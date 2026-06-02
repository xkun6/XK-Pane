import { Box, Typography, useTheme } from '@mui/material';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import AccountTreeRoundedIcon from '@mui/icons-material/AccountTreeRounded';
import SettingsRoundedIcon from '@mui/icons-material/SettingsRounded';
import SmartToyRoundedIcon from '@mui/icons-material/SmartToyRounded';
import type { ReactNode } from 'react';
import { useT } from '@/i18n';
import type { LaunchRecord } from '@/services';
import type { RightPanelId } from '@/rightpanel';
import { useHomePage } from './useHomePage';
import { homePageStyles } from './HomePage.styles';

// Home 仪表盘：时段问候 + 快捷操作 + 最近启动 + CLI 环境检测。
// 所有人类可见文本走 t()；颜色取 theme.palette.foundation；图标用 @mui/icons-material *Rounded。
export const HomePage = () => {
  const theme = useTheme();
  const styles = homePageStyles(theme);
  const t = useT();
  const vm = useHomePage();

  // panel 字段：该卡片打开右侧抽屉（git 已是面板而非页面），其余走页面导航。
  const actions: Array<{ id: string; label: string; icon: ReactNode; panel?: RightPanelId }> = [
    { id: 'terminal', label: t('home.actions.terminal'), icon: <TerminalRoundedIcon /> },
    { id: 'git', label: t('home.actions.git'), icon: <AccountTreeRoundedIcon />, panel: 'git' },
    { id: 'settings', label: t('home.actions.settings'), icon: <SettingsRoundedIcon /> },
  ];

  const activate = (a: { id: string; panel?: RightPanelId }): void =>
    a.panel ? vm.openPanel(a.panel) : vm.navigate(a.id);

  // 最近启动行的 CLI 标签：'none' → 终端；否则按 id 在 tools 里找显示名，找不到回落到 id。
  const cliLabel = (cliTool: string): string => {
    if (cliTool === 'none' || cliTool === '') return t('home.recent.shell');
    return vm.tools.find((tt) => tt.id === cliTool)?.displayName ?? cliTool;
  };

  const formatTime = (iso: string): string => {
    if (!iso) return '';
    const d = new Date(iso);
    return Number.isNaN(d.getTime()) ? '' : d.toLocaleString();
  };

  const renderLaunch = (r: LaunchRecord) => (
    <Box
      key={r.id}
      sx={styles.recentRow}
      role="button"
      tabIndex={0}
      title={t('home.recent.relaunchHint', { path: r.projectPath })}
      onClick={() => vm.relaunch(r)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          vm.relaunch(r);
        }
      }}
    >
      <Box component="span" sx={styles.recentIcon}>
        <TerminalRoundedIcon fontSize="small" />
      </Box>
      <Typography component="span" sx={styles.recentName}>
        {r.projectName} · {cliLabel(r.cliTool)}
      </Typography>
      <Typography component="span" sx={styles.recentTime}>
        {formatTime(r.launchedAt)}
      </Typography>
    </Box>
  );

  return (
    <Box sx={styles.root}>
      <Box sx={styles.body}>
        {/* 问候头 */}
        <Box sx={styles.section}>
          <Box sx={styles.greetingRow}>
            <Typography component="h1" sx={styles.greeting}>
              {t(`home.greeting.${vm.greetingKey}`)}
            </Typography>
            <Box component="span" sx={styles.statChip}>
              {t('home.projects.stat', { count: String(vm.projectCount) })}
            </Box>
          </Box>
          <Typography sx={styles.tagline}>{t('home.tagline')}</Typography>
        </Box>

        {/* 快捷操作 */}
        <Box sx={styles.section}>
          <Typography sx={styles.sectionTitle}>{t('home.actions.title')}</Typography>
          <Box sx={styles.actionsGrid}>
            {actions.map((a) => (
              <Box
                key={a.id}
                sx={styles.actionCard}
                role="button"
                tabIndex={0}
                onClick={() => activate(a)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    activate(a);
                  }
                }}
              >
                <Box component="span" sx={styles.actionIcon}>
                  {a.icon}
                </Box>
                <Typography component="span" sx={styles.actionLabel}>
                  {a.label}
                </Typography>
              </Box>
            ))}
          </Box>
        </Box>

        {/* 最近启动 */}
        <Box sx={styles.section}>
          <Typography sx={styles.sectionTitle}>{t('home.recent.title')}</Typography>
          {vm.launchesLoading ? (
            <Box sx={styles.emptyHint}>{t('common.loading')}</Box>
          ) : vm.recentLaunches.length === 0 ? (
            <Box sx={styles.emptyHint}>{t('home.recent.empty')}</Box>
          ) : (
            <Box sx={styles.recentList}>{vm.recentLaunches.map(renderLaunch)}</Box>
          )}
        </Box>

        {/* 环境检测 */}
        <Box sx={styles.section}>
          <Typography sx={styles.sectionTitle}>{t('home.env.title')}</Typography>
          {vm.toolsLoading ? (
            <Box sx={styles.emptyHint}>{t('home.env.detecting')}</Box>
          ) : (
            <Box sx={styles.envList}>
              {vm.tools.map((tool) => (
                <Box key={tool.id} sx={styles.envRow}>
                  <Box
                    component="span"
                    sx={{
                      ...styles.envDot,
                      backgroundColor: tool.installed
                        ? theme.palette.foundation.status.success
                        : theme.palette.foundation.text.muted,
                    }}
                  />
                  <SmartToyRoundedIcon fontSize="small" sx={{ color: theme.palette.foundation.text.secondary }} />
                  <Typography component="span" sx={styles.envName}>
                    {tool.displayName}
                  </Typography>
                  <Typography component="span" sx={styles.envVersion}>
                    {tool.installed ? tool.version || t('home.env.installed') : t('home.env.notInstalled')}
                  </Typography>
                </Box>
              ))}
            </Box>
          )}
        </Box>
      </Box>
    </Box>
  );
};
