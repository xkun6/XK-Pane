// OSC 52：终端内应用请求宿主把文本写入系统剪贴板。
// 格式为 `target;base64`，Claude Code 的 TUI 复制提示对应这类序列。
const TEXT_DECODER = new TextDecoder();

export function parseOsc52ClipboardText(data: string): string | null {
  const sep = data.indexOf(';');
  if (sep <= 0) return null;

  const target = data.slice(0, sep);
  if (!target.includes('c')) return null;

  const payload = data.slice(sep + 1);
  if (!payload || payload === '?') return null;

  try {
    const binary = atob(payload);
    const bytes = Uint8Array.from(binary, (ch) => ch.charCodeAt(0));
    return TEXT_DECODER.decode(bytes);
  } catch {
    return null;
  }
}
