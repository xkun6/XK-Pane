import { Box, Button, IconButton, Switch, Tooltip, Typography, useTheme } from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import GetAppRoundedIcon from '@mui/icons-material/GetAppRounded';
import EditRoundedIcon from '@mui/icons-material/EditRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import { useT } from '@/i18n';
import type { AgentMcpServer } from '@/services';
import { mcpStyles } from './Mcp.styles';
import { McpFormDialog } from './McpFormDialog';
import { useMcp } from './useMcp';
import { summarizeMcpServer } from './mcpSummary';
import { settingsPageStyles } from '../SettingsPage.styles';

// MCP 管理子页面：MCP server 以 XK-Pane 为唯一事实源（SSOT），按每行的两个开关分别投影到
// Claude Code（~/.claude.json 的 mcpServers）与 Codex（~/.codex/config.toml 的 [mcp_servers]）。
//
// 文案 namespace：settings.mcp.*。
export const Mcp = () => {
  const theme = useTheme();
  const shared = settingsPageStyles(theme);
  const styles = mcpStyles(theme);
  const vm = useMcp();
  const t = useT();

  const errorTitle = t('settings.mcp.error.title');

  const handleRemove = (s: AgentMcpServer): void => {
    void vm.remove(s, {
      confirmTitle: t('settings.mcp.removeConfirm.title'),
      confirmMessage: t('settings.mcp.removeConfirm.message', { name: s.name }),
      okLabel: t('settings.mcp.removeConfirm.ok'),
      cancelLabel: t('settings.mcp.removeConfirm.cancel'),
      errorTitle,
    });
  };

  const handleToggle = (s: AgentMcpServer, key: 'enabledClaude' | 'enabledCodex', value: boolean): void => {
    void vm.toggleEnabled(s, key, value, { errorTitle });
  };

  const handleImport = (): void => {
    void vm.importFromLive({
      errorTitle,
      doneTitle: t('settings.mcp.importDone.title'),
      doneMessage: (count) => t('settings.mcp.importDone.message', { count }),
    });
  };

  const handleSubmit = (): void => {
    void vm.submit({ errorTitle });
  };

  return (
    <Box sx={shared.section}>
      <Box sx={shared.sectionHeader}>
        <Typography sx={shared.sectionTitle}>{t('settings.mcp.title')}</Typography>
        <Typography sx={shared.sectionHint}>{t('settings.mcp.hint')}</Typography>
      </Box>

      <Box sx={styles.root}>
        <Box sx={styles.toolbar}>
          <Button
            size="small"
            variant="outlined"
            startIcon={<GetAppRoundedIcon />}
            onClick={handleImport}
            sx={styles.toolbarBtn}
          >
            {t('settings.mcp.importCurrent')}
          </Button>
          <Button
            size="small"
            variant="outlined"
            startIcon={<AddRoundedIcon />}
            onClick={vm.openAdd}
            sx={styles.toolbarBtn}
          >
            {t('settings.mcp.add')}
          </Button>
        </Box>

        <Box sx={styles.list}>
          {!vm.loading && vm.servers.length === 0 && <Box sx={styles.empty}>{t('settings.mcp.empty')}</Box>}

          {vm.servers.map((s) => {
            const summary = summarizeMcpServer(s.serverJson);
            return (
              <Box key={s.id} sx={styles.row}>
                <Box sx={styles.rowMain}>
                  <Typography sx={styles.rowName}>{s.name}</Typography>
                  <Typography sx={styles.rowSummary}>{summary ?? t('settings.mcp.noSummary')}</Typography>
                </Box>

                <Box sx={styles.rowToggles}>
                  <Box sx={styles.toggleItem}>
                    <Typography sx={styles.toggleLabel}>{t('settings.mcp.toggles.claude')}</Typography>
                    <Switch
                      size="small"
                      checked={s.enabledClaude}
                      onChange={(e) => handleToggle(s, 'enabledClaude', e.target.checked)}
                    />
                  </Box>
                  <Box sx={styles.toggleItem}>
                    <Typography sx={styles.toggleLabel}>{t('settings.mcp.toggles.codex')}</Typography>
                    <Switch
                      size="small"
                      checked={s.enabledCodex}
                      onChange={(e) => handleToggle(s, 'enabledCodex', e.target.checked)}
                    />
                  </Box>
                </Box>

                <Box sx={styles.rowActions}>
                  <Tooltip title={t('settings.mcp.actions.edit')}>
                    <IconButton
                      size="small"
                      aria-label={t('settings.mcp.actions.edit')}
                      onClick={() => vm.openEdit(s)}
                    >
                      <EditRoundedIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title={t('settings.mcp.actions.remove')}>
                    <IconButton
                      size="small"
                      aria-label={t('settings.mcp.actions.remove')}
                      onClick={() => handleRemove(s)}
                    >
                      <DeleteOutlineRoundedIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                </Box>
              </Box>
            );
          })}
        </Box>
      </Box>

      <McpFormDialog
        open={vm.formOpen}
        editingId={vm.editingId}
        draft={vm.draft}
        nameValid={vm.nameValid}
        jsonError={vm.jsonError}
        onFieldChange={vm.setField}
        onFormat={vm.formatJson}
        onCancel={vm.cancel}
        onSubmit={handleSubmit}
      />
    </Box>
  );
};
