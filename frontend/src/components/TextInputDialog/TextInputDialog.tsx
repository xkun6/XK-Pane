import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, TextField, useTheme } from '@mui/material';
import type { FormEvent } from 'react';
import { textInputDialogStyles } from './TextInputDialog.styles';
import { useTextInputDialog } from './useTextInputDialog';

// TextInputDialog：应用内「输入一个文本值」对话框（NativeDialogs 不含文本录入，
// 此为其补充）。受控：父组件管 open + 提供 onSubmit / onClose；文案全部由父传入
// （已经 t() 过，遵 MVVM「View 不持翻译串」—— 这里 props 即 View 的输入）。
// 用途：新建工作空间 / 重命名 / 设别名 等共用。Enter 提交，Esc / 点遮罩关闭。
export interface TextInputDialogProps {
  open: boolean;
  title: string;
  label: string;
  okLabel: string;
  cancelLabel: string;
  initialValue?: string;
  allowEmpty?: boolean; // 默认 false：trim 后为空时禁用提交
  onSubmit: (value: string) => void;
  onClose: () => void;
}

export const TextInputDialog = (props: TextInputDialogProps) => {
  const theme = useTheme();
  const styles = textInputDialogStyles(theme);
  const { value, setValue, canSubmit, submit } = useTextInputDialog(props);

  const handleSubmit = (e: FormEvent): void => {
    e.preventDefault();
    submit();
  };

  return (
    <Dialog open={props.open} onClose={props.onClose} fullWidth maxWidth="xs">
      <Box component="form" onSubmit={handleSubmit}>
        <DialogTitle sx={styles.title}>{props.title}</DialogTitle>
        <DialogContent sx={styles.content}>
          <TextField
            autoFocus
            fullWidth
            size="small"
            label={props.label}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            sx={styles.field}
          />
        </DialogContent>
        <DialogActions sx={styles.actions}>
          <Button onClick={props.onClose} color="inherit">
            {props.cancelLabel}
          </Button>
          <Button type="submit" variant="contained" disabled={!canSubmit}>
            {props.okLabel}
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
};
