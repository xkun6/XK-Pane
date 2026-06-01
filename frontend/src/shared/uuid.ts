// 安全上下文兜底的随机 id 生成。
//
// `crypto.randomUUID()` 仅在**安全上下文**可用（https 或 localhost/127.0.0.1）。P7 的 Web 访问
// 走「LAN IP + 明文 HTTP」（如 http://192.168.x.x:9317）属**非安全上下文** → `crypto.randomUUID`
// 为 undefined，直接调用抛 `TypeError: crypto.randomUUID is not a function`。由于面板树 id 在
// TerminalProvider 初始化（createInitialTreeState）时就生成、而该 Provider 包裹整个应用，这个抛错会
// 让 React 整棵树挂载失败 → 卡在启动骨架（表现为「纯色」空白页）。
//
// 本 helper 按可用性降级：randomUUID（安全上下文）→ getRandomValues 拼 RFC4122 v4（非安全上下文也可用）
// → Date+Math.random 末路兜底。id 仅作 UI key（pane/tab/历史项），非安全用途，弱随机也无碍。
export function randomId(): string {
  const c: Crypto | undefined = typeof globalThis !== 'undefined' ? globalThis.crypto : undefined;
  if (c && typeof c.randomUUID === 'function') {
    return c.randomUUID();
  }
  if (c && typeof c.getRandomValues === 'function') {
    const b = new Uint8Array(16);
    c.getRandomValues(b);
    b[6] = (b[6]! & 0x0f) | 0x40; // version 4
    b[8] = (b[8]! & 0x3f) | 0x80; // variant 10
    const hex = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
  }
  return `${Date.now().toString(16)}-${Math.random().toString(16).slice(2, 10)}`;
}
