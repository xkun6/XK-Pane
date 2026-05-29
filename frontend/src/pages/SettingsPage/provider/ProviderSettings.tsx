import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  IconButton,
  MenuItem,
  Switch,
  TextField,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import EditRoundedIcon from '@mui/icons-material/EditRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import StarRoundedIcon from '@mui/icons-material/StarRounded';
import StarBorderRoundedIcon from '@mui/icons-material/StarBorderRounded';
import { useT } from '@/i18n';
import type { Provider } from '@/services';
import { providerSettingsStyles } from './ProviderSettings.styles';
import { useProviderSettings } from './useProviderSettings';

// provider 设置子页（走已验证的设置框架）：列出 Provider + 新增/编辑（MUI Dialog 表单）+
// 删除（原生确认）+ 设默认。MVP 表单字段 = name / type / apiKey / baseUrl / (新增时) isDefault；
// 云类型的 region/projectId/awsProfile/configDir 条件字段留 3b。
const PROVIDER_TYPES = [
  'anthropic',
  'bedrock',
  'vertex',
  'proxy',
  'config_profile',
  'open_ai',
  'gemini',
  'kimi',
  'glm',
  'cursor',
  'opencode',
] as const;

// 每种 provider 类型在表单里显示哪些字段（与 provider.EnvVars 的 to_env_vars 映射对齐）：
// key 类只需 apiKey(+baseUrl)；云类用 region/awsProfile/projectId；config_profile 用 configDir。
type ProviderField = 'apiKey' | 'baseUrl' | 'region' | 'awsProfile' | 'projectId' | 'configDir';
const DEFAULT_FIELDS: ProviderField[] = ['apiKey', 'baseUrl'];
const FIELDS_BY_TYPE: Record<string, ProviderField[]> = {
  anthropic: ['apiKey', 'baseUrl'],
  proxy: ['apiKey', 'baseUrl'],
  open_ai: ['apiKey', 'baseUrl'],
  gemini: ['apiKey', 'baseUrl'],
  kimi: ['apiKey', 'baseUrl'],
  glm: ['apiKey', 'baseUrl'],
  opencode: ['apiKey', 'baseUrl'],
  cursor: ['apiKey'],
  bedrock: ['region', 'awsProfile'],
  vertex: ['region', 'projectId'],
  config_profile: ['configDir'],
};

export const ProviderSettings = () => {
  const theme = useTheme();
  const styles = providerSettingsStyles(theme);
  const t = useT();
  const vm = useProviderSettings();

  const handleRemove = (p: Provider): void => {
    void vm.remove(p, {
      confirmTitle: t('settings.provider.removeConfirm.title'),
      confirmMessage: t('settings.provider.removeConfirm.message', { name: p.name }),
      okLabel: t('settings.provider.removeConfirm.ok'),
      cancelLabel: t('settings.provider.removeConfirm.cancel'),
      errorTitle: t('settings.provider.error.title'),
    });
  };
  const handleSetDefault = (p: Provider): void => {
    void vm.setDefault(p, { errorTitle: t('settings.provider.error.title') });
  };
  const handleSubmit = (): void => {
    void vm.submit({ errorTitle: t('settings.provider.error.title') });
  };

  return (
    <Box sx={styles.root}>
      <Typography sx={styles.hint}>{t('settings.provider.hint')}</Typography>

      <Box sx={styles.toolbar}>
        <Button
          size="small"
          variant="outlined"
          startIcon={<AddRoundedIcon />}
          onClick={vm.openAdd}
          sx={styles.addBtn}
        >
          {t('settings.provider.add')}
        </Button>
      </Box>

      <Box sx={styles.list}>
        {!vm.loading && vm.providers.length === 0 && (
          <Box sx={styles.empty}>{t('settings.provider.empty')}</Box>
        )}

        {vm.providers.map((p) => (
          <Box key={p.id} sx={styles.row}>
            <Box sx={styles.rowMain}>
              <Box sx={styles.rowTitleLine}>
                <Typography sx={styles.rowName}>{p.name}</Typography>
                <Chip
                  label={t(`settings.provider.types.${p.providerType}`)}
                  size="small"
                  variant="outlined"
                  sx={styles.typeChip}
                />
                {p.isDefault && (
                  <Chip
                    label={t('settings.provider.defaultBadge')}
                    size="small"
                    variant="outlined"
                    sx={styles.defaultChip}
                  />
                )}
              </Box>
              <Typography sx={styles.rowSub}>
                {p.baseUrl || (p.hasApiKey ? t('settings.provider.keySet') : '')}
              </Typography>
            </Box>

            <Box sx={styles.rowActions}>
              {p.isDefault ? (
                <StarRoundedIcon fontSize="small" sx={{ color: theme.palette.foundation.accent }} />
              ) : (
                <Tooltip title={t('settings.provider.actions.setDefault')}>
                  <IconButton
                    size="small"
                    aria-label={t('settings.provider.actions.setDefault')}
                    onClick={() => handleSetDefault(p)}
                  >
                    <StarBorderRoundedIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
              )}
              <Tooltip title={t('settings.provider.actions.edit')}>
                <IconButton
                  size="small"
                  aria-label={t('settings.provider.actions.edit')}
                  onClick={() => vm.openEdit(p)}
                >
                  <EditRoundedIcon fontSize="small" />
                </IconButton>
              </Tooltip>
              <Tooltip title={t('settings.provider.actions.remove')}>
                <IconButton
                  size="small"
                  aria-label={t('settings.provider.actions.remove')}
                  onClick={() => handleRemove(p)}
                >
                  <DeleteOutlineRoundedIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            </Box>
          </Box>
        ))}
      </Box>

      <Dialog open={vm.formOpen} onClose={vm.cancel} maxWidth="sm" fullWidth>
        <DialogTitle>
          {vm.editingId
            ? t('settings.provider.form.editTitle')
            : t('settings.provider.form.addTitle')}
        </DialogTitle>
        <DialogContent>
          <TextField
            label={t('settings.provider.form.name')}
            placeholder={t('settings.provider.form.namePlaceholder')}
            value={vm.draft.name}
            onChange={(e) => vm.setField('name', e.target.value)}
            fullWidth
            size="small"
            sx={styles.formField}
          />
          <TextField
            label={t('settings.provider.form.type')}
            value={vm.draft.providerType}
            onChange={(e) => vm.setField('providerType', e.target.value)}
            select
            fullWidth
            size="small"
            sx={styles.formField}
          >
            {PROVIDER_TYPES.map((type) => (
              <MenuItem key={type} value={type}>
                {t(`settings.provider.types.${type}`)}
              </MenuItem>
            ))}
          </TextField>
          {(FIELDS_BY_TYPE[vm.draft.providerType] ?? DEFAULT_FIELDS).map((field) =>
            field === 'apiKey' ? (
              <TextField
                key="apiKey"
                label={t('settings.provider.form.apiKey')}
                placeholder={
                  vm.editingId
                    ? t('settings.provider.form.apiKeyPlaceholderEdit')
                    : t('settings.provider.form.apiKeyPlaceholderNew')
                }
                value={vm.draft.apiKey}
                onChange={(e) => vm.setField('apiKey', e.target.value)}
                type="password"
                fullWidth
                size="small"
                sx={styles.formField}
              />
            ) : (
              <TextField
                key={field}
                label={t(`settings.provider.form.${field}`)}
                value={vm.draft[field]}
                onChange={(e) => vm.setField(field, e.target.value)}
                fullWidth
                size="small"
                sx={styles.formField}
              />
            ),
          )}
          {vm.editingId === null && (
            <FormControlLabel
              sx={styles.formField}
              control={
                <Switch
                  checked={vm.draft.isDefault}
                  onChange={(e) => vm.setField('isDefault', e.target.checked)}
                />
              }
              label={t('settings.provider.form.isDefault')}
            />
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={vm.cancel} sx={{ textTransform: 'none' }}>
            {t('settings.provider.form.cancel')}
          </Button>
          <Button onClick={handleSubmit} variant="contained" sx={{ textTransform: 'none' }}>
            {t('settings.provider.form.save')}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};
