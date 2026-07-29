import { Box, Tab, Tabs, useTheme } from '@mui/material';
import { useT } from '@/i18n';
import { providerSettingsStyles } from './ProviderSettings.styles';
import { useProviderSettings, type ProviderTabId } from './useProviderSettings';
import { GlobalProviderPanel } from './GlobalProviderPanel';
import { OtherCliProviders } from './OtherCliProviders';

// Provider 设置子页（走已验证的设置框架）：四标签——
//   Claude Code / Codex / Grok Build：cc-switch 式全局配置 Provider（整份 CLI 配置文件切换），
//     由 GlobalProviderPanel 按 appType 参数化复用；Claude 启动时自动合并「系统公共配置 +
//     供应商凭据补丁」写入 ~/.claude/settings.json；
//   其它 CLI：按会话注入类型的 typed 表单 CRUD（gemini/kimi/glm/cursor/opencode）。
export const ProviderSettings = () => {
  const theme = useTheme();
  const styles = providerSettingsStyles(theme);
  const t = useT();
  const vm = useProviderSettings();

  return (
    <Box sx={styles.root}>
      <Tabs value={vm.tab} onChange={(_, v: ProviderTabId) => vm.setTab(v)} sx={styles.tabs}>
        <Tab value="claude" label={t('settings.provider.tabs.claude')} sx={styles.tab} />
        <Tab value="codex" label={t('settings.provider.tabs.codex')} sx={styles.tab} />
        <Tab value="grok" label={t('settings.provider.tabs.grok')} sx={styles.tab} />
        <Tab value="other" label={t('settings.provider.tabs.other')} sx={styles.tab} />
      </Tabs>
      <Box sx={styles.body}>
        {vm.tab === 'claude' && <GlobalProviderPanel appType="claude" />}
        {vm.tab === 'codex' && <GlobalProviderPanel appType="codex" />}
        {vm.tab === 'grok' && <GlobalProviderPanel appType="grok" />}
        {vm.tab === 'other' && <OtherCliProviders />}
      </Box>
    </Box>
  );
};
