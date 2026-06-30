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

  return (
    <Box sx={shared.section}>
      <Box sx={shared.sectionHeader}>
        <Typography sx={shared.sectionTitle}>{t('settings.botbridge.title')}</Typography>
        <Typography sx={shared.sectionHint}>{t('settings.botbridge.hint')}</Typography>
      </Box>

      <Box sx={styles.card}>
        <Box sx={styles.row}>
          <Typography sx={styles.label}>{t('settings.botbridge.enabled')}</Typography>
          <Switch checked={vm.enabled} onChange={(e) => vm.setEnabled(e.target.checked)} />
        </Box>

        <Box sx={styles.row}>
          <Typography sx={styles.label}>{t('settings.botbridge.platform')}</Typography>
          <TextField
            select
            size="small"
            value={vm.platform}
            onChange={(e) => vm.setPlatform(e.target.value)}
            sx={styles.input}
          >
            <MenuItem value="feishu">{t('settings.botbridge.platformFeishu')}</MenuItem>
          </TextField>
        </Box>

        <Box sx={styles.row}>
          <Typography sx={styles.label}>{t('settings.botbridge.appId')}</Typography>
          <TextField size="small" value={vm.appId} onChange={(e) => vm.setAppId(e.target.value)} sx={styles.input} />
        </Box>

        <Box sx={styles.row}>
          <Typography sx={styles.label}>{t('settings.botbridge.appSecret')}</Typography>
          <TextField
            size="small"
            type="password"
            value={vm.appSecret}
            placeholder={t('settings.botbridge.secretPlaceholder')}
            onChange={(e) => vm.setAppSecret(e.target.value)}
            sx={styles.input}
          />
        </Box>

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

        <Box component="button" type="button" sx={styles.actionPrimary} disabled={saving} onClick={() => void vm.save()}>
          <SaveRoundedIcon fontSize="small" />
          {t('settings.botbridge.save')}
        </Box>
      </Box>
    </Box>
  );
};

