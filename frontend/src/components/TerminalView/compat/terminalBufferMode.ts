// Alternate-buffer 序列处理 —— 移植 cc-pane web/components/panes/terminalBufferMode.ts。
// 对 agent CLI（claude/reclaude/codex），剥离 `CSI ?1049h/l`、`?1047`、`?47` 切屏序列，
// 让其输出留在 normal buffer（滚轮滚历史而非选旧 prompt）。
// cc-pane 用正则；这里用手写扫描 + ESC 常量，彻底避免正则反斜杠在工具链下的转义歧义。
import { ESC } from './esc';

export interface AlternateBufferTransition {
  mode: string;
  action: 'enter' | 'exit';
}

const ALT_MODES = ['1049', '1047', '47'];
const ALT_PREFIX = `${ESC}[?`; // CSI ?

// 在 data 的 idx 处尝试匹配 (1049|1047|47)(h|l)；命中返回 {mode, action, len(数字+终结符长度)}。
function matchAlt(
  data: string,
  idx: number,
): { mode: string; action: 'enter' | 'exit'; len: number } | null {
  for (const mode of ALT_MODES) {
    if (data.startsWith(mode, idx)) {
      const fin = data[idx + mode.length];
      if (fin === 'h' || fin === 'l') {
        return { mode, action: fin === 'h' ? 'enter' : 'exit', len: mode.length + 1 };
      }
    }
  }
  return null;
}

export function detectAlternateBufferTransitions(data: string): AlternateBufferTransition[] {
  const out: AlternateBufferTransition[] = [];
  let i = 0;
  while (i < data.length) {
    const idx = data.indexOf(ALT_PREFIX, i);
    if (idx === -1) break;
    const m = matchAlt(data, idx + ALT_PREFIX.length);
    if (m) {
      out.push({ mode: m.mode, action: m.action });
      i = idx + ALT_PREFIX.length + m.len;
    } else {
      i = idx + ALT_PREFIX.length;
    }
  }
  return out;
}

export function stripAlternateBufferSequences(data: string): string {
  let out = '';
  let i = 0;
  while (i < data.length) {
    const idx = data.indexOf(ALT_PREFIX, i);
    if (idx === -1) {
      out += data.slice(i);
      break;
    }
    const m = matchAlt(data, idx + ALT_PREFIX.length);
    if (m) {
      out += data.slice(i, idx);
      i = idx + ALT_PREFIX.length + m.len;
    } else {
      out += data.slice(i, idx + ALT_PREFIX.length);
      i = idx + ALT_PREFIX.length;
    }
  }
  return out;
}

export function shouldKeepCliOutputInNormalBuffer(cliToolId: string): boolean {
  return cliToolId === 'claude' || cliToolId === 'reclaude' || cliToolId === 'codex';
}
