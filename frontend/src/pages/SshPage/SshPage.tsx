import { useState, type MouseEvent } from 'react';
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  ListItemText,
  Menu,
  MenuItem,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import DnsRoundedIcon from '@mui/icons-material/DnsRounded';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import SmartToyRoundedIcon from '@mui/icons-material/SmartToyRounded';
import NetworkCheckRoundedIcon from '@mui/icons-material/NetworkCheckRounded';
import EditRoundedIcon from '@mui/icons-material/EditRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import ContentCopyRoundedIcon from '@mui/icons-material/ContentCopyRounded';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import { useT } from '@/i18n';
import type { SshMachine } from '@/services';
import { sshPageStyles } from './SshPage.styles';
import { useSshPage } from './useSshPage';
import { SshMachineFormDialog } from './SshMachineFormDialog';
import type { TestState } from './useSshPage';

// SSH 连接中心页（左侧导航「SSH」）：机器列表 + 一键连接（裸终端 / CLI 菜单）+ 连通状态点 +
// 增删改查。对标 cc-pane SshMachinesView，落地为 Foundation MVVM + 主题取色。
export const SshPage = () => {
  const theme = useTheme();
  const styles = sshPageStyles(theme);
  const t = useT();
  const vm = useSshPage();

  // 「以 CLI 启动」菜单：锚点 + 目标机器（null = 关闭）。
  const [launchMenu, setLaunchMenu] = useState<{ anchorEl: HTMLElement; machine: SshMachine } | null>(
    null,
  );

  const openLaunchMenu = (e: MouseEvent<HTMLElement>, machine: SshMachine): void => {
    setLaunchMenu({ anchorEl: e.currentTarget, machine });
  };
  const closeLaunchMenu = (): void => setLaunchMenu(null);

  const handleRemove = (m: SshMachine): void => {
    void vm.remove(m, {
      confirmTitle: t('ssh.removeConfirm.title'),
      confirmMessage: t('ssh.removeConfirm.message', { name: m.name }),
      okLabel: t('ssh.removeConfirm.ok'),
      cancelLabel: t('ssh.removeConfirm.cancel'),
      errorTitle: t('ssh.error.title'),
    });
  };
  const handleSubmit = (): void => {
    void vm.submit({ errorTitle: t('ssh.error.title') });
  };

  const target = (m: SshMachine): string => {
    const host = m.user ? `${m.user}@${m.host}` : m.host;
    return m.port && m.port !== 22 ? `${host}:${m.port}` : host;
  };

  // 连通状态点配色 + 提示文案。
  const dotColor = (test: TestState | undefined): string => {
    if (!test || (!test.testing && test.result === null)) return theme.palette.foundation.divider;
    if (test.testing) return theme.palette.foundation.status.warning;
    return test.result?.reachable
      ? theme.palette.foundation.status.success
      : theme.palette.foundation.status.danger;
  };
  const dotTitle = (test: TestState | undefined): string => {
    if (!test || (!test.testing && test.result === null)) return t('ssh.status.unknown');
    if (test.testing) return t('ssh.status.checking');
    if (test.result?.reachable) return test.result.message || t('ssh.status.online');
    return test.result?.message || t('ssh.status.offline');
  };

  return (
    <Box sx={styles.root}>
      <Box sx={styles.body}>
        <Box sx={styles.header}>
          <Typography component="h1" sx={styles.title}>
            {t('ssh.title')}
          </Typography>
          <Typography sx={styles.hint}>{t('ssh.hint')}</Typography>
        </Box>

        <Box sx={styles.toolbar}>
          <Button
            size="small"
            variant="text"
            startIcon={
              vm.checkingAll ? <CircularProgress size={14} /> : <RefreshRoundedIcon />
            }
            onClick={() => void vm.testAll()}
            disabled={vm.checkingAll || vm.machines.length === 0}
            sx={styles.toolbarBtn}
          >
            {t('ssh.testAll')}
          </Button>
          <Button
            size="small"
            variant="outlined"
            startIcon={<AddRoundedIcon />}
            onClick={vm.openAdd}
            sx={styles.toolbarBtn}
          >
            {t('ssh.add')}
          </Button>
        </Box>

        <Box sx={styles.list}>
          {!vm.loading && vm.machines.length === 0 && (
            <Box sx={styles.empty}>
              <DnsRoundedIcon sx={styles.emptyIcon} />
              <Typography sx={styles.emptyText}>{t('ssh.empty')}</Typography>
              <Button
                size="small"
                variant="outlined"
                startIcon={<AddRoundedIcon />}
                onClick={vm.openAdd}
                sx={styles.emptyAction}
              >
                {t('ssh.emptyAction')}
              </Button>
            </Box>
          )}

          {vm.machines.map((m) => {
            const test = vm.tests[m.id];
            return (
              <Box key={m.id} sx={styles.row}>
                <Box sx={styles.rowIconWrap}>
                  <DnsRoundedIcon sx={styles.rowIcon} />
                  <Tooltip title={dotTitle(test)}>
                    <Box sx={{ ...styles.statusDot, backgroundColor: dotColor(test) }} />
                  </Tooltip>
                </Box>

                <Box sx={styles.rowMain}>
                  <Box sx={styles.rowTitleLine}>
                    <Typography sx={styles.rowName}>{m.name}</Typography>
                    <Chip
                      label={t(`ssh.auth.${m.authMethod}`)}
                      size="small"
                      variant="outlined"
                      sx={styles.typeChip}
                    />
                    {m.authMethod === 'password' && m.hasStoredPassword && (
                      <Chip
                        label={t('ssh.pwStored')}
                        size="small"
                        variant="outlined"
                        sx={styles.pwChip}
                      />
                    )}
                    {m.tags.map((tag) => (
                      <Chip
                        key={tag}
                        label={tag}
                        size="small"
                        variant="outlined"
                        sx={styles.tagChip}
                      />
                    ))}
                  </Box>
                  <Typography sx={styles.rowSub}>
                    {target(m)}
                    {m.defaultPath ? `  ·  ${m.defaultPath}` : ''}
                  </Typography>
                  {m.description && (
                    <Typography sx={styles.rowDesc}>{m.description}</Typography>
                  )}
                </Box>

                <Box sx={styles.rowActions}>
                  <Tooltip title={t('ssh.actions.connect')}>
                    <IconButton
                      size="small"
                      aria-label={t('ssh.actions.connect')}
                      onClick={() => vm.connect(m, 'none')}
                    >
                      <TerminalRoundedIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title={t('ssh.actions.launchWith')}>
                    <IconButton
                      size="small"
                      aria-label={t('ssh.actions.launchWith')}
                      onClick={(e) => openLaunchMenu(e, m)}
                    >
                      <SmartToyRoundedIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title={t('ssh.actions.test')}>
                    <IconButton
                      size="small"
                      aria-label={t('ssh.actions.test')}
                      onClick={() => void vm.testConnectivity(m)}
                      disabled={vm.checkingAll || test?.testing}
                    >
                      <NetworkCheckRoundedIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title={t('ssh.actions.edit')}>
                    <IconButton
                      size="small"
                      aria-label={t('ssh.actions.edit')}
                      onClick={() => vm.openEdit(m)}
                    >
                      <EditRoundedIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title={t('ssh.actions.copy')}>
                    <IconButton
                      size="small"
                      aria-label={t('ssh.actions.copy')}
                      onClick={() => vm.copyConnection(m)}
                    >
                      <ContentCopyRoundedIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title={t('ssh.actions.remove')}>
                    <IconButton
                      size="small"
                      aria-label={t('ssh.actions.remove')}
                      onClick={() => handleRemove(m)}
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

      {/* 以 CLI 启动菜单 */}
      <Menu
        open={launchMenu !== null}
        anchorEl={launchMenu?.anchorEl ?? null}
        onClose={closeLaunchMenu}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        {vm.toolsLoading && <MenuItem disabled>{t('common.loading')}</MenuItem>}
        {!vm.toolsLoading &&
          vm.tools.map((tool) => (
            <MenuItem
              key={tool.id}
              disabled={!tool.installed}
              onClick={() => {
                if (launchMenu) vm.connect(launchMenu.machine, tool.id);
                closeLaunchMenu();
              }}
            >
              <ListItemText
                primary={tool.displayName}
                secondary={tool.installed ? tool.version || undefined : t('ssh.launchMenu.notInstalled')}
              />
            </MenuItem>
          ))}
      </Menu>

      <SshMachineFormDialog
        open={vm.formOpen}
        editingId={vm.editingId}
        draft={vm.draft}
        setField={vm.setField}
        onSubmit={handleSubmit}
        onCancel={vm.cancel}
      />
    </Box>
  );
};
