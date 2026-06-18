import type { SxProps, Theme } from '@mui/material';

export const inputDialogStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  return {
    paper: {
      borderRadius: 8,
      backgroundColor: fp.bg.elevated,
      border: `1px solid ${fp.divider}`,
      minWidth: 360,
    },
    title: {
      fontSize: 15,
      fontWeight: 600,
      color: fp.text.primary,
      pb: 1,
    },
    content: {
      pt: 1,
    },
    field: {
      '& .MuiOutlinedInput-root': {
        borderRadius: 6,
      },
    },
    actions: {
      px: 3,
      pb: 2,
      gap: 1,
    },
    cancelBtn: {
      borderRadius: 6,
      textTransform: 'none',
      color: fp.text.secondary,
      '&:hover': { backgroundColor: fp.bg.hover },
    },
    confirmBtn: {
      borderRadius: 6,
      textTransform: 'none',
      boxShadow: 'none',
      backgroundColor: fp.accent,
      color: '#fff',
      '&:hover': { backgroundColor: fp.accentHover, boxShadow: 'none' },
      '&.Mui-disabled': { backgroundColor: fp.bg.active, color: fp.text.muted },
    },
  };
};
