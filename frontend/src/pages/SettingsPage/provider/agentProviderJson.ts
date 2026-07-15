// 纯函数：校验 GlobalProviderPanel 表单里两块 JSON 文本域的合法性。不做 i18n（只返回语义
// 错误码），文案由 View 按 code 选 t() key。

export type JsonFieldError = '' | 'empty' | 'invalid';

function isJsonObject(trimmed: string): boolean {
  try {
    const v: unknown = JSON.parse(trimmed);
    return typeof v === 'object' && v !== null && !Array.isArray(v);
  } catch {
    return false;
  }
}

// claude settings.json 编辑器：必须是非空的合法 JSON 对象。
export function requiredJsonObjectError(raw: string): JsonFieldError {
  const trimmed = raw.trim();
  if (trimmed === '') return 'empty';
  return isJsonObject(trimmed) ? '' : 'invalid';
}

// codex auth.json 编辑器：留空合法（= 不管理登录态），非空则必须是合法 JSON 对象。
export function optionalJsonObjectError(raw: string): JsonFieldError {
  const trimmed = raw.trim();
  if (trimmed === '') return '';
  return isJsonObject(trimmed) ? '' : 'invalid';
}
