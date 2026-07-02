import { Box, Chip, MenuItem, Switch, TextField, Typography, useTheme } from '@mui/material';
import SaveRoundedIcon from '@mui/icons-material/SaveRounded';
import SmartToyRoundedIcon from '@mui/icons-material/SmartToyRounded';
import { useT } from '@/i18n';
import { settingsPageStyles } from '../SettingsPage.styles';
import { botbridgeStyles } from './Botbridge.styles';
import { useBotbridge } from './useBotbridge';

export const Botbridge = () => {
  const theme = useTheme();
  const shared = settingsPageStyles(theme);
  const styles = botbridgeStyles(theme);
  const vm = useBotbridge();
  const t = useT();

  const saving = vm.status.kind === 'saving' || vm.status.kind === 'loading';
  // 保存中锁整个表单：SetConfig 内含平台重连（可耗时数秒），完成后会用后端快照
  // 全量回写表单，期间的用户编辑会被静默吞掉。未成功加载（loaded=false）时禁止
  // 保存——空壳表单保存会清空后端已存凭据。
  const formDisabled = saving;
  const saveDisabled = saving || !vm.loaded;

  return (
    <Box sx={shared.section}>
      <Box sx={shared.sectionHeader}>
        <Typography sx={shared.sectionTitle}>{t('settings.botbridge.title')}</Typography>
        <Typography sx={shared.sectionHint}>{t('settings.botbridge.hint')}</Typography>
      </Box>

      <Box sx={styles.card}>
        <Box sx={styles.row}>
          <Typography sx={styles.label}>{t('settings.botbridge.enabled')}</Typography>
          <Switch checked={vm.enabled} disabled={formDisabled} onChange={(e) => vm.setEnabled(e.target.checked)} />
        </Box>

        <Box sx={styles.row}>
          <Typography sx={styles.label}>{t('settings.botbridge.platformLabel')}</Typography>
          <TextField
            select
            size="small"
            value={vm.platform}
            disabled={formDisabled}
            onChange={(e) => vm.setPlatform(e.target.value)}
            sx={styles.input}
          >
            {vm.platforms.map((id) => (
              <MenuItem key={id} value={id}>
                {t(`settings.botbridge.platform.${id}`)}
              </MenuItem>
            ))}
          </TextField>
        </Box>

        {vm.fields.map((field) => (
          <Box sx={styles.row} key={field.key}>
            <Typography sx={styles.label}>{t(`settings.botbridge.field.${field.key}`)}</Typography>
            <TextField
              size="small"
              type={field.secret ? 'password' : 'text'}
              value={field.value}
              disabled={formDisabled}
              placeholder={field.secret ? t('settings.botbridge.secretPlaceholder') : undefined}
              onChange={(e) => vm.setField(field.key, e.target.value)}
              sx={styles.input}
            />
          </Box>
        ))}

        <Box sx={styles.statusRow}>
          <Chip
            size="small"
            icon={<SmartToyRoundedIcon />}
            label={vm.connected ? t('settings.botbridge.connected') : t('settings.botbridge.disconnected')}
            color={vm.connected ? 'success' : 'default'}
            variant="outlined"
          />
          {vm.error && <Typography sx={styles.statusError}>{vm.error}</Typography>}
          {vm.status.kind === 'saved' && <Typography sx={styles.statusText}>{t('settings.botbridge.saved')}</Typography>}
          {vm.status.kind === 'failed' && <Typography sx={styles.statusError}>{vm.status.message}</Typography>}
        </Box>

        <Box component="button" type="button" sx={styles.actionPrimary} disabled={saveDisabled} onClick={() => void vm.save()}>
          <SaveRoundedIcon fontSize="small" />
          {t('settings.botbridge.save')}
        </Box>
      </Box>
    </Box>
  );
};
