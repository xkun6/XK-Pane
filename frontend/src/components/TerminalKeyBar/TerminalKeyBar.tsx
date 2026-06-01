import { Box, Button, useTheme } from '@mui/material';
import KeyboardArrowUpRoundedIcon from '@mui/icons-material/KeyboardArrowUpRounded';
import KeyboardArrowDownRoundedIcon from '@mui/icons-material/KeyboardArrowDownRounded';
import KeyboardArrowLeftRoundedIcon from '@mui/icons-material/KeyboardArrowLeftRounded';
import KeyboardArrowRightRoundedIcon from '@mui/icons-material/KeyboardArrowRightRounded';
import { useT } from '@/i18n';
import { terminalKeyBarStyles } from './TerminalKeyBar.styles';
import { useTerminalKeyBar, type KeyId } from './useTerminalKeyBar';

export interface TerminalKeyBarProps {
  /** 把按键序列写入会话 PTY（= TerminalService.write）。 */
  onKey: (data: string) => void;
  /** 会话未绑定时禁用。 */
  disabled?: boolean;
}

const arrowIcon: Record<string, typeof KeyboardArrowUpRoundedIcon> = {
  up: KeyboardArrowUpRoundedIcon,
  down: KeyboardArrowDownRoundedIcon,
  left: KeyboardArrowLeftRoundedIcon,
  right: KeyboardArrowRightRoundedIcon,
};

// TerminalKeyBar：移动端虚拟键栏（Esc/Tab/Ctrl+C/方向键）。仅 viewer 渲染（在 TerminalView 内）。
// 关键：按钮 onMouseDown 阻止默认，避免抢走 xterm 隐藏 textarea 的焦点（否则点一下键就收起软键盘）。
export const TerminalKeyBar = ({ onKey, disabled }: TerminalKeyBarProps) => {
  const theme = useTheme();
  const styles = terminalKeyBarStyles(theme);
  const t = useT();
  const { keys } = useTerminalKeyBar();

  return (
    <Box sx={styles.bar} role="toolbar" aria-label={t('terminalKeyBar.toolbar')}>
      {keys.map((k) => {
        const Arrow = k.kind === 'arrow' ? arrowIcon[k.id as KeyId] : undefined;
        return (
          <Button
            key={k.id}
            sx={styles.key}
            disabled={disabled}
            aria-label={t(k.ariaKey)}
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => onKey(k.seq)}
          >
            {Arrow ? <Arrow fontSize="small" /> : k.labelKey ? t(k.labelKey) : k.id}
          </Button>
        );
      })}
    </Box>
  );
};
