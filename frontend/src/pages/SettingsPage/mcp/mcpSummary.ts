// 纯函数：把 MCP server 的 serverJson 解析成列表行里的一句话摘要。
// 有 command → command + args 拼接；没有 command 但有 url → 展示 url；两者都没有 → null
// （由 View 决定 null 时展示的 i18n 占位文案，本函数不持有 t()）。
export function summarizeMcpServer(serverJson: string): string | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(serverJson);
  } catch {
    return null;
  }
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) return null;
  const obj = parsed as Record<string, unknown>;

  if (typeof obj.command === 'string' && obj.command.trim() !== '') {
    const args = Array.isArray(obj.args) ? obj.args.filter((a): a is string => typeof a === 'string') : [];
    return [obj.command, ...args].join(' ');
  }
  if (typeof obj.url === 'string' && obj.url.trim() !== '') {
    return obj.url;
  }
  return null;
}
