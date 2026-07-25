// Primary Device Attributes (DA) 应答 —— 移植自 cc-pane
// web/components/panes/terminalCapabilityReports.ts。
// codex 启动握手会发 `CSI c`(Primary DA)，必须回 `CSI ?1;2c`，否则握手卡住 / 行为异常。
//
// 原先这里还有一个 Kitty 键盘协议应答（`CSI ? u` → 回 `CSI ?0u` 声明不支持），已删：
// xterm 6.1 起内建实现了 Kitty 协议本体，开 `vtExtensions.kittyKeyboard` 即可（见
// useTerminalView.ts）。手写 handler 会 return true 抢在内建之前把它抑制掉，所以两者不能并存。
import { ESC } from './esc';

// 从 params 提取首参数 code（兼容 number 与 number[] 子参数），strict 安全。
function extractCode(params: Array<number | number[]>): number {
  if (params.length === 0) return 0;
  const first = params[0];
  if (first === undefined) return 0;
  return typeof first === 'number' ? first : (first[0] ?? 0);
}

export function buildPrimaryDeviceAttributesReport(
  params: Array<number | number[]>,
  prefix?: string,
): string | null {
  if (prefix) return null;
  if (params.length > 1) return null;
  if (extractCode(params) !== 0) return null;

  // VT100 with advanced video option. Codex only needs a valid DA response.
  return `${ESC}[?1;2c`;
}
