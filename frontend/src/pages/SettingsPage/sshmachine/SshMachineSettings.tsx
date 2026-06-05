import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  MenuItem,
  TextField,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import EditRoundedIcon from '@mui/icons-material/EditRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import NetworkCheckRoundedIcon from '@mui/icons-material/NetworkCheckRounded';
import CheckCircleRoundedIcon from '@mui/icons-material/CheckCircleRounded';
import ErrorOutlineRoundedIcon from '@mui/icons-material/ErrorOutlineRounded';
import { useT } from '@/i18n';
import type { SshMachine } from '@/services';
import { sshMachineSettingsStyles } from './SshMachineSettings.styles';
import { useSshMachineSettings } from './useSshMachineSettings';

// sshmachine 设置子页（走已验证的设置框架）：列出 SSH 机器 + 新增/编辑（MUI Dialog 表单）+
// 删除（原生确认）+ 逐机连通性测试。认证方式 key/agent/password；password 经 cryptox 加密，
// 编辑时密码框留空 = 保留原密码。所有文案走 t()，配色取 theme.palette.foundation.*。
const AUTH_METHODS = ['key', 'agent', 'password'] as const;

export const SshMachineSettings = () => {
  const theme = useTheme();
  const styles = sshMachineSettingsStyles(theme);
  const t = useT();
  const vm = useSshMachineSettings();

  const handleRemove = (m: SshMachine): void => {
    void vm.remove(m, {
      confirmTitle: t('settings.sshmachine.removeConfirm.title'),
      confirmMessage: t('settings.sshmachine.removeConfirm.message', { name: m.name }),
      okLabel: t('settings.sshmachine.removeConfirm.ok'),
      cancelLabel: t('settings.sshmachine.removeConfirm.cancel'),
      errorTitle: t('settings.sshmachine.error.title'),
    });
  };
  const handleSubmit = (): void => {
    void vm.submit({ errorTitle: t('settings.sshmachine.error.title') });
  };

  const target = (m: SshMachine): string => {
    const host = m.user ? `${m.user}@${m.host}` : m.host;
    return m.port && m.port !== 22 ? `${host}:${m.port}` : host;
  };

  return (
    <Box sx={styles.root}>
      <Typography sx={styles.hint}>{t('settings.sshmachine.hint')}</Typography>

      <Box sx={styles.toolbar}>
        <Button
          size="small"
          variant="outlined"
          startIcon={<AddRoundedIcon />}
          onClick={vm.openAdd}
          sx={styles.addBtn}
        >
          {t('settings.sshmachine.add')}
        </Button>
      </Box>

      <Box sx={styles.list}>
        {!vm.loading && vm.machines.length === 0 && (
          <Box sx={styles.empty}>{t('settings.sshmachine.empty')}</Box>
        )}

        {vm.machines.map((m) => {
          const test = vm.tests[m.id];
          return (
            <Box key={m.id} sx={styles.row}>
              <Box sx={styles.rowMain}>
                <Box sx={styles.rowTitleLine}>
                  <Typography sx={styles.rowName}>{m.name}</Typography>
                  <Chip
                    label={t(`settings.sshmachine.auth.${m.authMethod}`)}
                    size="small"
                    variant="outlined"
                    sx={styles.typeChip}
                  />
                  {m.authMethod === 'password' && m.hasStoredPassword && (
                    <Chip
                      label={t('settings.sshmachine.pwStored')}
                      size="small"
                      variant="outlined"
                      sx={styles.pwChip}
                    />
                  )}
                </Box>
                <Typography sx={styles.rowSub}>
                  {target(m)}
                  {m.defaultPath ? `  ·  ${m.defaultPath}` : ''}
                </Typography>
              </Box>

              <Box sx={styles.rowActions}>
                {test?.testing ? (
                  <CircularProgress size={16} sx={styles.spinner} />
                ) : test?.result ? (
                  <Tooltip title={test.result.message}>
                    {test.result.reachable ? (
                      <CheckCircleRoundedIcon fontSize="small" sx={styles.okIcon} />
                    ) : (
                      <ErrorOutlineRoundedIcon fontSize="small" sx={styles.failIcon} />
                    )}
                  </Tooltip>
                ) : null}
                <Tooltip title={t('settings.sshmachine.actions.test')}>
                  <IconButton
                    size="small"
                    aria-label={t('settings.sshmachine.actions.test')}
                    onClick={() => void vm.testConnectivity(m)}
                  >
                    <NetworkCheckRoundedIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Tooltip title={t('settings.sshmachine.actions.edit')}>
                  <IconButton
                    size="small"
                    aria-label={t('settings.sshmachine.actions.edit')}
                    onClick={() => vm.openEdit(m)}
                  >
                    <EditRoundedIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Tooltip title={t('settings.sshmachine.actions.remove')}>
                  <IconButton
                    size="small"
                    aria-label={t('settings.sshmachine.actions.remove')}
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

      <Dialog open={vm.formOpen} onClose={vm.cancel} maxWidth="sm" fullWidth>
        <DialogTitle>
          {vm.editingId
            ? t('settings.sshmachine.form.editTitle')
            : t('settings.sshmachine.form.addTitle')}
        </DialogTitle>
        <DialogContent>
          <TextField
            label={t('settings.sshmachine.form.name')}
            placeholder={t('settings.sshmachine.form.namePlaceholder')}
            value={vm.draft.name}
            onChange={(e) => vm.setField('name', e.target.value)}
            fullWidth
            size="small"
            sx={styles.formField}
          />
          <Box sx={styles.formRow}>
            <TextField
              label={t('settings.sshmachine.form.host')}
              placeholder={t('settings.sshmachine.form.hostPlaceholder')}
              value={vm.draft.host}
              onChange={(e) => vm.setField('host', e.target.value)}
              size="small"
              sx={styles.hostField}
            />
            <TextField
              label={t('settings.sshmachine.form.port')}
              value={vm.draft.port === 0 ? '' : String(vm.draft.port)}
              onChange={(e) => {
                const n = parseInt(e.target.value, 10);
                vm.setField('port', Number.isNaN(n) ? 0 : n);
              }}
              type="number"
              size="small"
              sx={styles.portField}
            />
          </Box>
          <TextField
            label={t('settings.sshmachine.form.user')}
            placeholder={t('settings.sshmachine.form.userPlaceholder')}
            value={vm.draft.user}
            onChange={(e) => vm.setField('user', e.target.value)}
            fullWidth
            size="small"
            sx={styles.formField}
          />
          <TextField
            label={t('settings.sshmachine.form.authMethod')}
            value={vm.draft.authMethod}
            onChange={(e) => vm.setField('authMethod', e.target.value)}
            select
            fullWidth
            size="small"
            sx={styles.formField}
          >
            {AUTH_METHODS.map((method) => (
              <MenuItem key={method} value={method}>
                {t(`settings.sshmachine.auth.${method}`)}
              </MenuItem>
            ))}
          </TextField>
          {vm.draft.authMethod === 'key' && (
            <TextField
              label={t('settings.sshmachine.form.identityFile')}
              placeholder={t('settings.sshmachine.form.identityFilePlaceholder')}
              value={vm.draft.identityFile}
              onChange={(e) => vm.setField('identityFile', e.target.value)}
              fullWidth
              size="small"
              sx={styles.formField}
            />
          )}
          {vm.draft.authMethod === 'password' && (
            <TextField
              label={t('settings.sshmachine.form.password')}
              placeholder={
                vm.editingId
                  ? t('settings.sshmachine.form.passwordPlaceholderEdit')
                  : t('settings.sshmachine.form.passwordPlaceholderNew')
              }
              value={vm.draft.password}
              onChange={(e) => vm.setField('password', e.target.value)}
              type="password"
              fullWidth
              size="small"
              sx={styles.formField}
            />
          )}
          <TextField
            label={t('settings.sshmachine.form.defaultPath')}
            placeholder={t('settings.sshmachine.form.defaultPathPlaceholder')}
            value={vm.draft.defaultPath}
            onChange={(e) => vm.setField('defaultPath', e.target.value)}
            fullWidth
            size="small"
            sx={styles.formField}
          />
          <TextField
            label={t('settings.sshmachine.form.tags')}
            placeholder={t('settings.sshmachine.form.tagsPlaceholder')}
            value={vm.draft.tags.join(', ')}
            onChange={(e) =>
              vm.setField(
                'tags',
                e.target.value
                  .split(',')
                  .map((s) => s.trim())
                  .filter(Boolean),
              )
            }
            fullWidth
            size="small"
            sx={styles.formField}
          />
          <TextField
            label={t('settings.sshmachine.form.description')}
            value={vm.draft.description}
            onChange={(e) => vm.setField('description', e.target.value)}
            fullWidth
            size="small"
            sx={styles.formField}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={vm.cancel} sx={{ textTransform: 'none' }}>
            {t('settings.sshmachine.form.cancel')}
          </Button>
          <Button onClick={handleSubmit} variant="contained" sx={{ textTransform: 'none' }}>
            {t('settings.sshmachine.form.save')}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};
