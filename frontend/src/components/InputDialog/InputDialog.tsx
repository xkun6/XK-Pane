import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  useTheme,
} from '@mui/material';
import { useEffect, useState, type FocusEvent, type KeyboardEvent } from 'react';
import { inputDialogStyles } from './InputDialog.styles';

// 受控的「单行文本输入」对话框：用于新建文件 / 文件夹、重命名等需要取名的场景。
// NativeDialogs 不提供 prompt（铁律禁用浏览器 prompt），故自造此组件。
// 文案全部由调用方经 props 传入（已 t() 处理），本组件不含硬编码可见字符串。
export interface InputDialogProps {
  open: boolean;
  title: string;
  label: string;
  confirmText: string;
  cancelText: string;
  initialValue?: string;
  onConfirm: (value: string) => void;
  onClose: () => void;
}

export const InputDialog = ({
  open,
  title,
  label,
  confirmText,
  cancelText,
  initialValue = '',
  onConfirm,
  onClose,
}: InputDialogProps) => {
  const theme = useTheme();
  const styles = inputDialogStyles(theme);
  const [value, setValue] = useState(initialValue);

  // 每次打开用 initialValue 重置输入。
  useEffect(() => {
    if (open) setValue(initialValue);
  }, [open, initialValue]);

  const trimmed = value.trim();
  const submit = (): void => {
    if (trimmed) onConfirm(trimmed);
  };
  const onKeyDown = (e: KeyboardEvent): void => {
    if (e.key === 'Enter') {
      e.preventDefault();
      submit();
    }
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth slotProps={{ paper: { sx: styles.paper } }}>
      <DialogTitle sx={styles.title}>{title}</DialogTitle>
      <DialogContent sx={styles.content}>
        <TextField
          autoFocus
          fullWidth
          size="small"
          variant="outlined"
          label={label}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={onKeyDown}
          sx={styles.field}
          slotProps={{
            htmlInput: { onFocus: (e: FocusEvent<HTMLInputElement>) => e.currentTarget.select() },
          }}
        />
      </DialogContent>
      <DialogActions sx={styles.actions}>
        <Button onClick={onClose} sx={styles.cancelBtn}>
          {cancelText}
        </Button>
        <Button onClick={submit} disabled={!trimmed} variant="contained" sx={styles.confirmBtn}>
          {confirmText}
        </Button>
      </DialogActions>
    </Dialog>
  );
};
