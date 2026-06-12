import { describe, expect, it } from 'vitest';
import { comboFromKeyboardEvent } from './shortcutCombo';

const ev = (
  key: string,
  mods: Partial<Record<'ctrlKey' | 'altKey' | 'shiftKey' | 'metaKey', boolean>> = {},
  code?: string,
) => ({
  ctrlKey: false,
  altKey: false,
  shiftKey: false,
  metaKey: false,
  key,
  ...(code !== undefined ? { code } : {}),
  ...mods,
});

describe('comboFromKeyboardEvent', () => {
  it('修饰键 + 字母 → 组合串（字母大写）', () => {
    expect(comboFromKeyboardEvent(ev('s', { ctrlKey: true, altKey: true }, 'KeyS'))).toBe(
      'Ctrl+Alt+S',
    );
  });

  it('修饰键顺序固定 Ctrl→Alt→Shift→Win', () => {
    expect(
      comboFromKeyboardEvent(ev('a', { metaKey: true, shiftKey: true, ctrlKey: true }, 'KeyA')),
    ).toBe('Ctrl+Shift+Win+A');
  });

  it('F 键支持 F1-F24', () => {
    expect(comboFromKeyboardEvent(ev('F12', { ctrlKey: true }, 'F12'))).toBe('Ctrl+F12');
    expect(comboFromKeyboardEvent(ev('f2', { altKey: true }))).toBe('Alt+F2'); // 无 code 回退 key
  });

  it('Shift+数字：key 是上档符号但 code 是 Digit*，仍能录出数字组合', () => {
    expect(comboFromKeyboardEvent(ev('!', { ctrlKey: true, shiftKey: true }, 'Digit1'))).toBe(
      'Ctrl+Shift+1',
    );
  });

  it('无修饰键 → null', () => {
    expect(comboFromKeyboardEvent(ev('s', {}, 'KeyS'))).toBeNull();
  });

  it('只按修饰键（key 为 Control/Alt 等）→ null', () => {
    expect(comboFromKeyboardEvent(ev('Control', { ctrlKey: true }, 'ControlLeft'))).toBeNull();
    expect(comboFromKeyboardEvent(ev('Alt', { altKey: true }, 'AltLeft'))).toBeNull();
  });

  it('不支持的主键 → null', () => {
    expect(comboFromKeyboardEvent(ev('Escape', { ctrlKey: true }, 'Escape'))).toBeNull();
    expect(comboFromKeyboardEvent(ev('F25', { ctrlKey: true }, 'F25'))).toBeNull();
    expect(comboFromKeyboardEvent(ev('!', { ctrlKey: true, shiftKey: true }))).toBeNull(); // 无 code 时上档符号录不出
  });
});
