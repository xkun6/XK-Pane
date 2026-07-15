import { describe, expect, it } from 'vitest';
import { optionalJsonObjectError, requiredJsonObjectError } from './agentProviderJson';

describe('requiredJsonObjectError', () => {
  it('空字符串 → empty', () => {
    expect(requiredJsonObjectError('')).toBe('empty');
    expect(requiredJsonObjectError('   ')).toBe('empty');
  });

  it('合法 JSON 对象 → 无错误', () => {
    expect(requiredJsonObjectError('{}')).toBe('');
    expect(requiredJsonObjectError('{"env":{"ANTHROPIC_API_KEY":"x"}}')).toBe('');
  });

  it('语法错误 / 非对象（数组、字符串）→ invalid', () => {
    expect(requiredJsonObjectError('{')).toBe('invalid');
    expect(requiredJsonObjectError('[]')).toBe('invalid');
    expect(requiredJsonObjectError('"x"')).toBe('invalid');
    expect(requiredJsonObjectError('null')).toBe('invalid');
  });
});

describe('optionalJsonObjectError', () => {
  it('空字符串 → 无错误（= 不管理登录态）', () => {
    expect(optionalJsonObjectError('')).toBe('');
    expect(optionalJsonObjectError('   ')).toBe('');
  });

  it('合法 JSON 对象 → 无错误', () => {
    expect(optionalJsonObjectError('{"OPENAI_API_KEY":"x"}')).toBe('');
  });

  it('非空但非法 → invalid', () => {
    expect(optionalJsonObjectError('{bad')).toBe('invalid');
    expect(optionalJsonObjectError('[1,2]')).toBe('invalid');
  });
});
