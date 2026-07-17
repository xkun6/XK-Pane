import { createTheme, type Theme, type ThemeOptions } from '@mui/material/styles';
import type { FoundationPalette, FoundationThemePreset } from './types';

// 把 foundation 字段挂到 MUI 类型系统上：
// 这样在任何组件里都能 theme.palette.foundation.bg.surface 取色。
declare module '@mui/material/styles' {
  interface Palette {
    foundation: FoundationPalette;
  }
  interface PaletteOptions {
    foundation?: FoundationPalette;
  }
}

const FONT_STACK = '"Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif';

// 软阴影阶梯：卡片 / 弹层用 elevation 表达层级，比纯边框更现代。
// 亮色偏冷灰阴影；暗色用更深的 ambient（避免发灰「脏」）。
const softShadows = (mode: 'light' | 'dark'): string[] => {
  if (mode === 'light') {
    return [
      'none',
      '0 1px 2px rgba(15, 23, 42, 0.04), 0 1px 3px rgba(15, 23, 42, 0.06)',
      '0 2px 6px rgba(15, 23, 42, 0.05), 0 4px 12px rgba(15, 23, 42, 0.06)',
      '0 4px 12px rgba(15, 23, 42, 0.06), 0 8px 24px rgba(15, 23, 42, 0.08)',
      '0 8px 20px rgba(15, 23, 42, 0.08), 0 16px 40px rgba(15, 23, 42, 0.10)',
      '0 12px 28px rgba(15, 23, 42, 0.10), 0 24px 48px rgba(15, 23, 42, 0.12)',
      ...Array(19).fill('0 16px 40px rgba(15, 23, 42, 0.12), 0 32px 64px rgba(15, 23, 42, 0.14)'),
    ];
  }
  return [
    'none',
    '0 1px 2px rgba(0, 0, 0, 0.35), 0 0 0 1px rgba(255, 255, 255, 0.03)',
    '0 4px 12px rgba(0, 0, 0, 0.40), 0 0 0 1px rgba(255, 255, 255, 0.04)',
    '0 8px 24px rgba(0, 0, 0, 0.45), 0 0 0 1px rgba(255, 255, 255, 0.05)',
    '0 12px 32px rgba(0, 0, 0, 0.50), 0 0 0 1px rgba(255, 255, 255, 0.05)',
    '0 16px 40px rgba(0, 0, 0, 0.55), 0 0 0 1px rgba(255, 255, 255, 0.06)',
    ...Array(19).fill('0 24px 56px rgba(0, 0, 0, 0.60), 0 0 0 1px rgba(255, 255, 255, 0.06)'),
  ];
};

// 把 FoundationThemePreset 编译为完整的 MUI Theme。
// MUI palette、components 全套覆盖都在这里集中应用，
// 调用方只需要决定 palette 颜色与 mode，不需要重复声明组件覆盖。
export const buildMuiTheme = (preset: FoundationThemePreset): Theme => {
  const { palette, mode, muiOverrides } = preset;
  const isDark = mode === 'dark';

  const base: ThemeOptions = {
    palette: {
      mode,
      foundation: palette,
      primary: { main: palette.accent, dark: palette.accentHover, contrastText: '#fff' },
      error: { main: palette.status.danger },
      success: { main: palette.status.success },
      warning: { main: palette.status.warning },
      background: { default: palette.bg.content, paper: palette.bg.surface },
      text: {
        primary: palette.text.primary,
        secondary: palette.text.secondary,
        disabled: palette.text.muted,
      },
      divider: palette.divider,
    },
    typography: {
      fontFamily: FONT_STACK,
      fontSize: 14,
      h1: { fontWeight: 700, letterSpacing: '-0.02em', lineHeight: 1.2 },
      h2: { fontWeight: 700, letterSpacing: '-0.015em', lineHeight: 1.25 },
      h3: { fontWeight: 600, letterSpacing: '-0.01em', lineHeight: 1.3 },
      h4: { fontWeight: 600, letterSpacing: '-0.01em' },
      h5: { fontWeight: 600 },
      h6: { fontWeight: 600 },
      button: { textTransform: 'none', fontWeight: 500, letterSpacing: 0 },
      body1: { lineHeight: 1.55 },
      body2: { lineHeight: 1.5 },
      caption: { lineHeight: 1.4, letterSpacing: 0.1 },
    },
    // 方圆设计：shape 基值保持 4，便于 sx 中 borderRadius: 1.5 → 6px、2 → 8px。
    // 按钮 / 输入 / Paper 等在 components 里显式写 8–12px，不依赖 shape 放大。
    shape: {
      borderRadius: 4,
    },
    shadows: softShadows(mode) as Theme['shadows'],
    components: {
      MuiCssBaseline: {
        styleOverrides: {
          ':root': { colorScheme: mode },
          body: {
            margin: 0,
            padding: 0,
            // 可见视口高度（移动端关键）：index.html 内联脚本把 visualViewport.height 写进 --xk-vh。
            // 不能用 100vh —— 手机 100vh 含地址栏遮挡区，比可见区高 → 底部(对话输入条)被推到屏外。
            height: 'var(--xk-vh, 100dvh)',
            overflow: 'hidden',
            backgroundColor: palette.bg.content,
            color: palette.text.primary,
            fontFamily: FONT_STACK,
            userSelect: 'none',
            WebkitFontSmoothing: 'antialiased',
            MozOsxFontSmoothing: 'grayscale',
          },
          '#root': {
            height: 'var(--xk-vh, 100dvh)',
            display: 'flex',
            flexDirection: 'column',
          },
          // 细滚动条：不抢内容焦点
          '*::-webkit-scrollbar': { width: 6, height: 6 },
          '*::-webkit-scrollbar-thumb': {
            backgroundColor: isDark ? 'rgba(255,255,255,0.12)' : 'rgba(15,23,42,0.14)',
            borderRadius: 999,
            border: '1px solid transparent',
            backgroundClip: 'padding-box',
          },
          '*::-webkit-scrollbar-thumb:hover': {
            backgroundColor: isDark ? 'rgba(255,255,255,0.20)' : 'rgba(15,23,42,0.22)',
          },
          '*::-webkit-scrollbar-track': { backgroundColor: 'transparent' },
          // 键盘焦点环：可访问性 + 现代感
          '*:focus-visible': {
            outline: `2px solid ${palette.accent}`,
            outlineOffset: 2,
          },
          // 按钮 / 图标按钮在 focus-visible 时用 box-shadow，避免双重 outline
          '.MuiButtonBase-root:focus-visible': {
            outline: 'none',
            boxShadow: `0 0 0 2px ${palette.bg.content}, 0 0 0 4px ${palette.accent}`,
          },
        },
      },
      MuiButton: {
        defaultProps: { disableElevation: true },
        styleOverrides: {
          root: {
            borderRadius: 8,
            minHeight: 34,
            padding: '6px 16px',
            fontWeight: 500,
            transition:
              'background-color 160ms cubic-bezier(0.4, 0, 0.2, 1), color 160ms cubic-bezier(0.4, 0, 0.2, 1), border-color 160ms cubic-bezier(0.4, 0, 0.2, 1), box-shadow 160ms cubic-bezier(0.4, 0, 0.2, 1), transform 100ms ease',
            '&:active': { transform: 'translateY(0.5px)' },
          },
          contained: {
            boxShadow: 'none',
            '&:hover': {
              boxShadow: isDark
                ? '0 2px 8px rgba(0,0,0,0.35)'
                : '0 2px 8px rgba(37, 99, 235, 0.22)',
            },
          },
          outlined: {
            borderColor: palette.divider,
            backgroundColor: 'transparent',
            '&:hover': {
              borderColor: palette.accent,
              backgroundColor: palette.bg.hover,
            },
          },
          text: {
            '&:hover': { backgroundColor: palette.bg.hover },
          },
          sizeSmall: {
            minHeight: 28,
            padding: '4px 12px',
            fontSize: 13,
            borderRadius: 7,
          },
          sizeLarge: {
            minHeight: 40,
            padding: '8px 20px',
            borderRadius: 10,
          },
        },
      },
      MuiIconButton: {
        styleOverrides: {
          root: {
            borderRadius: 8,
            color: palette.text.secondary,
            backgroundColor: 'transparent',
            transition:
              'background-color 160ms cubic-bezier(0.4, 0, 0.2, 1), color 160ms cubic-bezier(0.4, 0, 0.2, 1)',
            '&:hover': {
              backgroundColor: palette.bg.hover,
              color: palette.text.primary,
            },
          },
          sizeSmall: { borderRadius: 7 },
        },
      },
      MuiPaper: {
        defaultProps: { elevation: 0 },
        styleOverrides: {
          root: {
            backgroundImage: 'none',
            borderRadius: 12,
            backgroundColor: palette.bg.surface,
          },
          outlined: {
            border: `1px solid ${palette.divider}`,
          },
          elevation1: {
            boxShadow: softShadows(mode)[1],
          },
          elevation2: {
            boxShadow: softShadows(mode)[2],
          },
          elevation3: {
            boxShadow: softShadows(mode)[3],
          },
        },
      },
      MuiCard: {
        defaultProps: { elevation: 0 },
        styleOverrides: {
          root: {
            borderRadius: 12,
            border: `1px solid ${palette.divider}`,
            backgroundColor: palette.bg.surface,
            backgroundImage: 'none',
          },
        },
      },
      MuiTextField: {
        defaultProps: { size: 'small', variant: 'outlined' },
      },
      MuiOutlinedInput: {
        styleOverrides: {
          root: {
            borderRadius: 8,
            backgroundColor: palette.bg.elevated,
            transition: 'background-color 160ms ease, box-shadow 160ms ease',
            '& fieldset': {
              borderColor: palette.divider,
              transition: 'border-color 160ms ease',
            },
            '&:hover fieldset': { borderColor: isDark ? 'rgba(148,163,184,0.28)' : 'rgba(15,23,42,0.16)' },
            '&.Mui-focused': {
              backgroundColor: palette.bg.surface,
              '& fieldset': { borderColor: palette.accent, borderWidth: 1.5 },
            },
          },
          input: {
            paddingTop: 9,
            paddingBottom: 9,
          },
        },
      },
      MuiTooltip: {
        defaultProps: { arrow: true, enterDelay: 400 },
        styleOverrides: {
          tooltip: {
            backgroundColor: isDark ? palette.bg.elevated : palette.bg.surface,
            color: palette.text.primary,
            border: `1px solid ${palette.divider}`,
            fontSize: 12,
            fontWeight: 500,
            borderRadius: 8,
            padding: '6px 10px',
            boxShadow: softShadows(mode)[2],
          },
          arrow: {
            color: isDark ? palette.bg.elevated : palette.bg.surface,
            '&::before': {
              border: `1px solid ${palette.divider}`,
            },
          },
        },
      },
      MuiDialog: {
        styleOverrides: {
          paper: {
            borderRadius: 16,
            border: `1px solid ${palette.divider}`,
            backgroundImage: 'none',
            backgroundColor: palette.bg.surface,
            boxShadow: softShadows(mode)[4],
          },
        },
      },
      MuiDialogTitle: {
        styleOverrides: {
          root: {
            fontSize: 17,
            fontWeight: 600,
            letterSpacing: '-0.01em',
            paddingBottom: 8,
          },
        },
      },
      MuiMenu: {
        styleOverrides: {
          paper: {
            borderRadius: 10,
            border: `1px solid ${palette.divider}`,
            backgroundImage: 'none',
            backgroundColor: palette.bg.surface,
            boxShadow: softShadows(mode)[3],
            marginTop: 4,
          },
          list: {
            padding: 4,
          },
        },
      },
      MuiMenuItem: {
        styleOverrides: {
          root: {
            borderRadius: 7,
            minHeight: 36,
            fontSize: 13.5,
            margin: '1px 0',
            transition: 'background-color 120ms ease',
            '&:hover': { backgroundColor: palette.bg.hover },
            '&.Mui-selected': {
              backgroundColor: palette.bg.active,
              color: palette.accent,
              '&:hover': { backgroundColor: palette.bg.active },
            },
          },
        },
      },
      MuiPopover: {
        styleOverrides: {
          paper: {
            borderRadius: 12,
            border: `1px solid ${palette.divider}`,
            backgroundImage: 'none',
            boxShadow: softShadows(mode)[3],
          },
        },
      },
      MuiChip: {
        styleOverrides: {
          root: {
            borderRadius: 8,
            fontWeight: 500,
            height: 26,
          },
          filled: {
            backgroundColor: palette.bg.elevated,
            color: palette.text.secondary,
          },
          outlined: {
            borderColor: palette.divider,
          },
        },
      },
      MuiSwitch: {
        styleOverrides: {
          root: {
            width: 42,
            height: 26,
            padding: 0,
          },
          switchBase: {
            padding: 3,
            '&.Mui-checked': {
              transform: 'translateX(16px)',
              color: '#fff',
              '& + .MuiSwitch-track': {
                backgroundColor: palette.accent,
                opacity: 1,
                border: 0,
              },
            },
          },
          thumb: {
            width: 20,
            height: 20,
            boxShadow: '0 1px 2px rgba(0,0,0,0.18)',
          },
          track: {
            borderRadius: 13,
            backgroundColor: isDark ? 'rgba(255,255,255,0.14)' : 'rgba(15,23,42,0.16)',
            opacity: 1,
          },
        },
      },
      MuiTabs: {
        styleOverrides: {
          indicator: {
            height: 2.5,
            borderRadius: 2,
            backgroundColor: palette.accent,
          },
        },
      },
      MuiTab: {
        styleOverrides: {
          root: {
            textTransform: 'none',
            fontWeight: 500,
            minHeight: 40,
            color: palette.text.secondary,
            '&.Mui-selected': { color: palette.accent },
          },
        },
      },
      MuiDivider: {
        styleOverrides: {
          root: {
            borderColor: palette.divider,
          },
        },
      },
      MuiListItemButton: {
        styleOverrides: {
          root: {
            borderRadius: 8,
            '&.Mui-selected': {
              backgroundColor: palette.bg.active,
              '&:hover': { backgroundColor: palette.bg.active },
            },
            '&:hover': { backgroundColor: palette.bg.hover },
          },
        },
      },
      MuiAlert: {
        styleOverrides: {
          root: {
            borderRadius: 10,
            border: `1px solid ${palette.divider}`,
          },
        },
      },
      MuiLinearProgress: {
        styleOverrides: {
          root: {
            borderRadius: 999,
            height: 4,
            backgroundColor: palette.bg.elevated,
          },
          bar: {
            borderRadius: 999,
          },
        },
      },
      MuiSkeleton: {
        styleOverrides: {
          root: {
            backgroundColor: palette.bg.elevated,
          },
          rounded: {
            borderRadius: 8,
          },
        },
      },
    },
  };

  // createTheme 第二参数支持深合并，preset 可以微调任意 MUI 选项。
  return muiOverrides ? createTheme(base, muiOverrides) : createTheme(base);
};
