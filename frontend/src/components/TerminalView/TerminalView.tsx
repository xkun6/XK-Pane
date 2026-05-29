import { Box, useTheme } from '@mui/material';
import '@xterm/xterm/css/xterm.css';
import { terminalViewStyles } from './TerminalView.styles';
import { useTerminalView, type UseTerminalViewArgs } from './useTerminalView';

export type TerminalViewProps = UseTerminalViewArgs;

// TerminalView：把一个后端 PTY 会话渲染为可交互终端（xterm）。
// 视图只负责挂载 host 容器；全部 xterm 生命周期 / I/O 接线在 useTerminalView。
export const TerminalView = (props: TerminalViewProps) => {
  const theme = useTheme();
  const styles = terminalViewStyles(theme);
  const { hostRef } = useTerminalView(props);
  return <Box ref={hostRef} sx={styles.host} />;
};
