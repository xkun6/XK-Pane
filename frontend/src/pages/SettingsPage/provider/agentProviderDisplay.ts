// 纯函数：从 AgentProvider.settingsJson 中提取列表行展示用的 baseUrl 提示。
// 解析失败一律返回空字符串（仅用于展示，不抛错）。
import type { AgentAppType } from '@/services';

const CODEX_BASE_URL_RE = /base_url\s*=\s*"([^"]+)"/;

export function extractBaseUrlHint(appType: AgentAppType, settingsJson: string): string {
  try {
    if (appType === 'claude' || appType === 'reclaude') {
      const parsed = JSON.parse(settingsJson) as { env?: Record<string, unknown> };
      const v = parsed.env?.ANTHROPIC_BASE_URL;
      return typeof v === 'string' ? v : '';
    }
    const parsed = JSON.parse(settingsJson) as { config?: unknown };
    const config = typeof parsed.config === 'string' ? parsed.config : '';
    const m = CODEX_BASE_URL_RE.exec(config);
    return m?.[1] ?? '';
  } catch {
    return '';
  }
}
