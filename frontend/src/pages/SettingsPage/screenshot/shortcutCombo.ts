// 快捷键录制纯函数：KeyboardEvent → "Ctrl+Alt+S" 组合串。
// 规则与后端 screenshotsvc.parseShortcut 对齐：至少 1 个修饰键 + 1 个主键（A-Z / 0-9 / F1-F24）。
// 主键优先用 e.code（物理键位，免疫 Shift 上档与键盘布局：Shift+1 的 key 是 '!' 但 code 是
// 'Digit1'），无 code 时回退 e.key。只按修饰键 / 不支持的主键 / 无修饰键时返回 null。

export interface ComboKeySource {
  ctrlKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  metaKey: boolean;
  key: string;
  code?: string;
}

export const comboFromKeyboardEvent = (e: ComboKeySource): string | null => {
  const key = normalizeMainKey(e.key, e.code);
  if (!key) return null;
  const mods: string[] = [];
  if (e.ctrlKey) mods.push('Ctrl');
  if (e.altKey) mods.push('Alt');
  if (e.shiftKey) mods.push('Shift');
  if (e.metaKey) mods.push('Win');
  if (mods.length === 0) return null;
  return [...mods, key].join('+');
};

const normalizeMainKey = (key: string, code?: string): string | null => {
  if (code) {
    const letter = /^Key([A-Z])$/.exec(code);
    if (letter) return letter[1]!;
    const digit = /^Digit([0-9])$/.exec(code);
    if (digit) return digit[1]!;
    if (/^F([1-9]|1[0-9]|2[0-4])$/.test(code)) return code;
    return null;
  }
  if (/^[a-z0-9]$/i.test(key)) return key.toUpperCase();
  if (/^F([1-9]|1[0-9]|2[0-4])$/i.test(key)) return key.toUpperCase();
  return null;
};
