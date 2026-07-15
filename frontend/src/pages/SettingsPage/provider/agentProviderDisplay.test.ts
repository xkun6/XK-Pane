import { describe, expect, it } from 'vitest';
import { extractBaseUrlHint } from './agentProviderDisplay';

describe('extractBaseUrlHint', () => {
  it('claude：env.ANTHROPIC_BASE_URL 存在则返回其值', () => {
    expect(
      extractBaseUrlHint('claude', '{"env":{"ANTHROPIC_BASE_URL":"https://api.example.com"}}'),
    ).toBe('https://api.example.com');
  });

  it('claude：无 env.ANTHROPIC_BASE_URL → 空字符串', () => {
    expect(extractBaseUrlHint('claude', '{"env":{"ANTHROPIC_API_KEY":"x"}}')).toBe('');
    expect(extractBaseUrlHint('claude', '{}')).toBe('');
  });

  it('codex：从 config 文本正则提取 base_url', () => {
    const settingsJson = JSON.stringify({
      auth: null,
      config: 'model = "gpt-5"\nbase_url = "https://relay.example.com/v1"\n',
    });
    expect(extractBaseUrlHint('codex', settingsJson)).toBe('https://relay.example.com/v1');
  });

  it('codex：config 无 base_url → 空字符串', () => {
    const settingsJson = JSON.stringify({ auth: null, config: 'model = "gpt-5"\n' });
    expect(extractBaseUrlHint('codex', settingsJson)).toBe('');
  });

  it('非法 JSON → 空字符串（不抛错）', () => {
    expect(extractBaseUrlHint('claude', 'not json')).toBe('');
    expect(extractBaseUrlHint('codex', 'not json')).toBe('');
  });
});
