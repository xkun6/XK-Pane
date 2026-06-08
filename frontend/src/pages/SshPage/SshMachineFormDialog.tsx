import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
  TextField,
} from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { useT } from '@/i18n';
import type { SshMachineInput } from '@/services';
import { sshMachineFormStyles } from './SshMachineFormDialog.styles';

const AUTH_METHODS = ['key', 'agent', 'password'] as const;

export interface SshMachineFormDialogProps {
  open: boolean;
  editingId: string | null; // null = 新增；string = 编辑
  draft: SshMachineInput;
  setField: <K extends keyof SshMachineInput>(key: K, value: SshMachineInput[K]) => void;
  onSubmit: () => void;
  onCancel: () => void;
}

// SSH 机器新增 / 编辑表单（受控）：认证方式 key/agent/password；password 经 cryptox 加密，
// 编辑时密码框留空 = 保留原密码。本身是 View，直接 useT()；草稿态与提交逻辑由父 VM 持有。
export const SshMachineFormDialog = ({
  open,
  editingId,
  draft,
  setField,
  onSubmit,
  onCancel,
}: SshMachineFormDialogProps) => {
  const theme = useTheme();
  const styles = sshMachineFormStyles(theme);
  const t = useT();

  return (
    <Dialog open={open} onClose={onCancel} maxWidth="sm" fullWidth>
      <DialogTitle>
        {editingId ? t('ssh.form.editTitle') : t('ssh.form.addTitle')}
      </DialogTitle>
      <DialogContent>
        <TextField
          label={t('ssh.form.name')}
          placeholder={t('ssh.form.namePlaceholder')}
          value={draft.name}
          onChange={(e) => setField('name', e.target.value)}
          fullWidth
          size="small"
          sx={styles.formField}
        />
        <Box sx={styles.formRow}>
          <TextField
            label={t('ssh.form.host')}
            placeholder={t('ssh.form.hostPlaceholder')}
            value={draft.host}
            onChange={(e) => setField('host', e.target.value)}
            size="small"
            sx={styles.hostField}
          />
          <TextField
            label={t('ssh.form.port')}
            value={draft.port === 0 ? '' : String(draft.port)}
            onChange={(e) => {
              const n = parseInt(e.target.value, 10);
              setField('port', Number.isNaN(n) ? 0 : n);
            }}
            type="number"
            size="small"
            sx={styles.portField}
          />
        </Box>
        <TextField
          label={t('ssh.form.user')}
          placeholder={t('ssh.form.userPlaceholder')}
          value={draft.user}
          onChange={(e) => setField('user', e.target.value)}
          fullWidth
          size="small"
          sx={styles.formField}
        />
        <TextField
          label={t('ssh.form.authMethod')}
          value={draft.authMethod}
          onChange={(e) => setField('authMethod', e.target.value)}
          select
          fullWidth
          size="small"
          sx={styles.formField}
        >
          {AUTH_METHODS.map((method) => (
            <MenuItem key={method} value={method}>
              {t(`ssh.auth.${method}`)}
            </MenuItem>
          ))}
        </TextField>
        {draft.authMethod === 'key' && (
          <TextField
            label={t('ssh.form.identityFile')}
            placeholder={t('ssh.form.identityFilePlaceholder')}
            value={draft.identityFile}
            onChange={(e) => setField('identityFile', e.target.value)}
            fullWidth
            size="small"
            sx={styles.formField}
          />
        )}
        {draft.authMethod === 'password' && (
          <TextField
            label={t('ssh.form.password')}
            placeholder={
              editingId
                ? t('ssh.form.passwordPlaceholderEdit')
                : t('ssh.form.passwordPlaceholderNew')
            }
            value={draft.password}
            onChange={(e) => setField('password', e.target.value)}
            type="password"
            fullWidth
            size="small"
            sx={styles.formField}
          />
        )}
        <TextField
          label={t('ssh.form.defaultPath')}
          placeholder={t('ssh.form.defaultPathPlaceholder')}
          value={draft.defaultPath}
          onChange={(e) => setField('defaultPath', e.target.value)}
          fullWidth
          size="small"
          sx={styles.formField}
        />
        <TextField
          label={t('ssh.form.tags')}
          placeholder={t('ssh.form.tagsPlaceholder')}
          value={draft.tags.join(', ')}
          onChange={(e) =>
            setField(
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
          label={t('ssh.form.description')}
          value={draft.description}
          onChange={(e) => setField('description', e.target.value)}
          fullWidth
          size="small"
          sx={styles.formField}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel} sx={{ textTransform: 'none' }}>
          {t('ssh.form.cancel')}
        </Button>
        <Button onClick={onSubmit} variant="contained" sx={{ textTransform: 'none' }}>
          {t('ssh.form.save')}
        </Button>
      </DialogActions>
    </Dialog>
  );
};
