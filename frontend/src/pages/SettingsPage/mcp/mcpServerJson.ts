// 纯函数：把用户粘贴进 MCP server JSON 编辑器的文本归一化。
// 用户常从 README 复制的配置带包裹层（{"servers": {...}} 或 Claude 风格的
// {"mcpServers": {...}}），本函数负责识别包裹层并解出裸 server 定义；
// 未命中包裹层时按裸定义处理。仅做字符串 → 结果的纯计算，不依赖 React / i18n。

export type NormalizeResult =
  | { kind: 'empty' }
  | { kind: 'invalid' }
  | { kind: 'bare'; json: string } // 裸 server 定义（已 2 空格 pretty-print）
  | { kind: 'unwrapped'; name: string; json: string } // 从包裹层解出的唯一 server
  | { kind: 'multiple'; names: string[] }; // 包裹层含多个 server

const WRAPPER_KEYS = ['servers', 'mcpServers'];

function isPlainObject(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

// 尝试把 obj 当作包裹层解开；未命中包裹层判定时返回 null（交给调用方按裸对象处理）。
function tryUnwrap(obj: Record<string, unknown>): NormalizeResult | null {
  const keys = Object.keys(obj);
  if (keys.length !== 1) return null;
  const wrapperKey = keys[0];
  if (wrapperKey === undefined || !WRAPPER_KEYS.includes(wrapperKey)) return null;

  const inner = obj[wrapperKey];
  if (!isPlainObject(inner)) return null;
  if (!Object.values(inner).every(isPlainObject)) return null;

  const names = Object.keys(inner);
  if (names.length === 0) return { kind: 'invalid' };
  if (names.length === 1) {
    const name = names[0];
    if (name === undefined) return { kind: 'invalid' };
    return { kind: 'unwrapped', name, json: JSON.stringify(inner[name], null, 2) };
  }
  return { kind: 'multiple', names };
}

export function normalizeServerJson(raw: string): NormalizeResult {
  const trimmed = raw.trim();
  if (trimmed === '') return { kind: 'empty' };

  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return { kind: 'invalid' };
  }
  if (!isPlainObject(parsed)) return { kind: 'invalid' };

  const wrapped = tryUnwrap(parsed);
  if (wrapped) return wrapped;

  return { kind: 'bare', json: JSON.stringify(parsed, null, 2) };
}
