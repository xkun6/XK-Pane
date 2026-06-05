import { useEffect, useState, type FormEvent } from 'react';
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
import type { SshMachine } from '@/services';

// 提交草稿：仅 machineId + remotePath；其余连接字段（host/port/user/...）由父从机器 DTO 拷入合成 cfg。
export interface SshProjectDraft {
  machineId: string;
  remotePath: string;
}

// 文案由父（持 t()）传入，遵 MVVM「View 不持翻译串」。
export interface AddSshProjectDialogStrings {
  title: string;
  machineLabel: string;
  pathLabel: string;
  pathPlaceholder: string;
  add: string;
  cancel: string;
  noMachines: string;
  noMachinesHint: string;
}

export interface AddSshProjectDialogProps {
  open: boolean;
  machines: SshMachine[];
  strings: AddSshProjectDialogStrings;
  onSubmit: (draft: SshProjectDraft) => void;
  onClose: () => void;
}

// 「添加 SSH 项目」对话框：选机器（已在设置里登记的 ssh_machines）+ 填远端路径。
// 受控：父管 open / 提供 onSubmit / onClose。无机器时给空态提示（引导去设置里添加）。
// remotePath 留空 = 用机器的 defaultPath（父合成 cfg 时兜底）；本地表单态，不持业务逻辑。
export const AddSshProjectDialog = ({
  open,
  machines,
  strings,
  onSubmit,
  onClose,
}: AddSshProjectDialogProps) => {
  const [machineId, setMachineId] = useState('');
  const [remotePath, setRemotePath] = useState('');

  // 每次打开：预选第一台机器并把远端路径填为其 defaultPath（用户可改）。
  useEffect(() => {
    if (!open) return;
    const first = machines[0];
    setMachineId(first?.id ?? '');
    setRemotePath(first?.defaultPath ?? '');
  }, [open, machines]);

  const onPickMachine = (id: string): void => {
    setMachineId(id);
    const m = machines.find((x) => x.id === id);
    setRemotePath(m?.defaultPath ?? '');
  };

  const hasMachines = machines.length > 0;
  const canSubmit = hasMachines && machineId !== '';

  const handleSubmit = (e: FormEvent): void => {
    e.preventDefault();
    if (!canSubmit) return;
    onSubmit({ machineId, remotePath: remotePath.trim() });
  };

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <Box component="form" onSubmit={handleSubmit}>
        <DialogTitle sx={{ pb: 1 }}>{strings.title}</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
          {!hasMachines ? (
            <Box>
              <Box sx={{ fontSize: 13 }}>{strings.noMachines}</Box>
              <Box sx={{ fontSize: 11, mt: 0.5, opacity: 0.7, lineHeight: 1.5 }}>
                {strings.noMachinesHint}
              </Box>
            </Box>
          ) : (
            <>
              <TextField
                select
                fullWidth
                size="small"
                label={strings.machineLabel}
                value={machineId}
                onChange={(e) => onPickMachine(e.target.value)}
              >
                {machines.map((m) => (
                  <MenuItem key={m.id} value={m.id}>
                    {m.name} ({m.user ? `${m.user}@${m.host}` : m.host})
                  </MenuItem>
                ))}
              </TextField>
              <TextField
                fullWidth
                size="small"
                label={strings.pathLabel}
                placeholder={strings.pathPlaceholder}
                value={remotePath}
                onChange={(e) => setRemotePath(e.target.value)}
              />
            </>
          )}
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button onClick={onClose} color="inherit">
            {strings.cancel}
          </Button>
          <Button type="submit" variant="contained" disabled={!canSubmit}>
            {strings.add}
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
};
