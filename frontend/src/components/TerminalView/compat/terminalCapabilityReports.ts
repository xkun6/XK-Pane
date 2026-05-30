// Primary Device Attributes (DA) + Kitty 键盘协议 capability 应答 ——
// 忠实移植 cc-pane web/components/panes/terminalCapabilityReports.ts。
// codex 启动握手会发 `CSI c`(Primary DA) 与 `CSI ? u`(Kitty 键盘协议查询)，
// 必须分别回 `CSI ?1;2c` 与 `CSI ?0u`，否则握手卡住 / 行为异常。
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

export function buildKittyKeyboardProtocolReport(
  params: Array<number | number[]>,
  prefix?: string,
): string | null {
  if (prefix !== '?') return null;
  if (params.length > 1) return null;
  if (extractCode(params) !== 0) return null;

  // xterm.js does not implement Kitty keyboard protocol flags.
  return `${ESC}[?0u`;
}
