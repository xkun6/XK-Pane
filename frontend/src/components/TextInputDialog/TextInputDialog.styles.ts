import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface TextInputDialogSx {
  title: S;
  content: S;
  field: S;
  actions: S;
}

export const textInputDialogStyles = (theme: Theme): TextInputDialogSx => {
  const fp = theme.palette.foundation;
  return {
    title: { fontSize: 15, fontWeight: 600, color: fp.text.primary },
    content: { pt: 1 },
    // TextField 自身 marginTop 给 outlined 浮动 label 留空间：MUI 的
    // `DialogTitle + DialogContent { paddingTop:0 }` 相邻选择器特异性高于 sx，
    // 会裁掉浮动 label，故把空间加在 TextField 上（不受该选择器影响）。
    field: { mt: 1.5 },
    actions: { px: 3, pb: 2, pt: 1 },
  };
};
