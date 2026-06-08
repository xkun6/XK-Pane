import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface SshMachineFormSx {
  formField: S;
  formRow: S;
  hostField: S;
  portField: S;
}

export const sshMachineFormStyles = (_theme: Theme): SshMachineFormSx => ({
  formField: { mt: 2 },
  formRow: { display: 'flex', gap: 1.5, mt: 2 },
  hostField: { flex: 1 },
  portField: { width: 120 },
});
