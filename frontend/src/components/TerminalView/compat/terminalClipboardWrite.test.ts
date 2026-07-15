import { describe, expect, it } from 'vitest';
import { parseOsc52ClipboardText } from './terminalClipboardWrite';

describe('parseOsc52ClipboardText', () => {
  it('解析 c 剪贴板目标的 UTF-8 base64 文本', () => {
    expect(parseOsc52ClipboardText('c;SGVsbG8g5Lit5paH')).toBe('Hello 中文');
  });

  it('忽略 OSC 52 剪贴板读取请求', () => {
    expect(parseOsc52ClipboardText('c;?')).toBeNull();
  });

  it('忽略非剪贴板目标', () => {
    expect(parseOsc52ClipboardText('p;SGVsbG8=')).toBeNull();
  });
});
