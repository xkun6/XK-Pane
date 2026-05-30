import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface TextInputDialogSx {
  title: S;
  content: S;
  actions: S;
}

export const textInputDialogStyles = (theme: Theme): TextInputDialogSx => {
  const fp = theme.palette.foundation;
  return {
    title: { fontSize: 15, fontWeight: 600, color: fp.text.primary },
    content: { pt: 1 },
    actions: { px: 3, pb: 2, pt: 1 },
  };
};
