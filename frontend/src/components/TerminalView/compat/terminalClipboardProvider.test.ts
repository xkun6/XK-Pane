import { describe, expect, it, vi } from 'vitest';
import { createTerminalClipboardProvider } from './terminalClipboardProvider';

describe('createTerminalClipboardProvider', () => {
  it('把解码后的文本交给注入的原生写入通道', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    const provider = createTerminalClipboardProvider(writeText);

    await provider.writeText('c', 'Hello 中文');

    expect(writeText).toHaveBeenCalledWith('Hello 中文');
  });

  it('空文本跳过写入（不把用户剪贴板清空）', () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    const provider = createTerminalClipboardProvider(writeText);

    provider.writeText('c', '');

    expect(writeText).not.toHaveBeenCalled();
  });

  it('写入失败不抛出（终端应用不该因剪贴板失败而中断）', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('denied'));
    const provider = createTerminalClipboardProvider(writeText);

    await expect(provider.writeText('c', 'x')).resolves.toBeUndefined();
  });

  it('不支持读剪贴板：恒返回空串，不泄露给终端内的程序', () => {
    const provider = createTerminalClipboardProvider(vi.fn());

    expect(provider.readText('c')).toBe('');
  });
});
