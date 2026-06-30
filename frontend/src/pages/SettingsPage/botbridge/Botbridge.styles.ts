import type { SxProps, Theme } from '@mui/material';

export const botbridgeStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  return {
    card: {
      display: 'flex',
      flexDirection: 'column',
      gap: 2,
      p: 2.5,
      borderRadius: 1.5,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
    },
    row: {
      display: 'grid',
      gridTemplateColumns: '140px minmax(0, 1fr)',
      gap: 2,
      alignItems: 'center',
    },
    label: {
      fontSize: 12,
      color: fp.text.muted,
      letterSpacing: 0.4,
      textTransform: 'uppercase',
      fontWeight: 600,
    },
    input: {
      '& .MuiInputBase-root': {
        fontSize: 13,
        backgroundColor: fp.bg.elevated,
      },
      '& .MuiInputBase-input': {
        py: 1,
      },
    },
    statusRow: {
      display: 'flex',
      alignItems: 'center',
      gap: 1,
      flexWrap: 'wrap',
    },
    statusText: {
      fontSize: 12,
      color: fp.text.muted,
    },
    statusError: {
      fontSize: 12,
      color: fp.status.danger,
    },
    actionPrimary: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 0.75,
      alignSelf: 'flex-start',
      fontSize: 13,
      fontWeight: 600,
      px: 2,
      py: 1,
      borderRadius: 1,
      border: `1px solid ${fp.accent}`,
      backgroundColor: fp.accent,
      color: '#fff',
      cursor: 'pointer',
      '&:hover': { backgroundColor: fp.accentHover },
      '&:disabled': { opacity: 0.6, cursor: 'not-allowed' },
    },
  };
};

