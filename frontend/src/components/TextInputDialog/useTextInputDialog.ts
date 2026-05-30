import { useEffect, useState } from 'react';
import type { TextInputDialogProps } from './TextInputDialog';

// TextInputDialog 的 ViewModel：管输入框受控 state + 提交守门。
// open 切换到 true 时把输入重置为 initialValue（每次打开都从干净/初值开始）。
export interface TextInputDialogModel {
  value: string;
  setValue: (v: string) => void;
  canSubmit: boolean;
  submit: () => void;
}

export function useTextInputDialog(props: TextInputDialogProps): TextInputDialogModel {
  const { open, initialValue = '', allowEmpty = false, onSubmit, onClose } = props;
  const [value, setValue] = useState(initialValue);

  useEffect(() => {
    if (open) setValue(initialValue);
  }, [open, initialValue]);

  const trimmed = value.trim();
  const canSubmit = allowEmpty || trimmed.length > 0;

  const submit = (): void => {
    if (!canSubmit) return;
    onSubmit(trimmed);
    onClose();
  };

  return { value, setValue, canSubmit, submit };
}
