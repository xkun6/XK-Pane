import { describe, expect, it } from 'vitest';
import { formatPastedPaths } from './terminalClipboardPaste';

describe('formatPastedPaths', () => {
  it('单路径无空白：原样返回（claude 切分正则要求裸路径开头）', () => {
    expect(formatPastedPaths(['C:\\shots\\a.png'])).toBe('C:\\shots\\a.png');
  });

  it('单路径含空格：包双引号（shell 不断词，claude removeOuterQuotes 兼容）', () => {
    expect(formatPastedPaths(['C:\\Users\\John Doe\\a.png'])).toBe('"C:\\Users\\John Doe\\a.png"');
  });

  it('多路径：空格裸拼不加引号（保 claude 多路径 lookahead 切分）', () => {
    expect(formatPastedPaths(['C:\\a.png', 'C:\\b c.png'])).toBe('C:\\a.png C:\\b c.png');
  });

  it('空列表：空串', () => {
    expect(formatPastedPaths([])).toBe('');
  });
});
