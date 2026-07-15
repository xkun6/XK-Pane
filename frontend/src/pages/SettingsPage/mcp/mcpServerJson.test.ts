import { describe, expect, it } from 'vitest';
import { normalizeServerJson } from './mcpServerJson';

describe('normalizeServerJson', () => {
  it('空字符串 → empty', () => {
    expect(normalizeServerJson('')).toEqual({ kind: 'empty' });
    expect(normalizeServerJson('   ')).toEqual({ kind: 'empty' });
  });

  it('非法 JSON → invalid', () => {
    expect(normalizeServerJson('{')).toEqual({ kind: 'invalid' });
  });

  it('数组 → invalid', () => {
    expect(normalizeServerJson('[1,2,3]')).toEqual({ kind: 'invalid' });
  });

  it('裸 server 定义 → bare（pretty-print）', () => {
    const input = '{"type":"sse","url":"http://127.0.0.1:19816/sse"}';
    expect(normalizeServerJson(input)).toEqual({
      kind: 'bare',
      json: JSON.stringify({ type: 'sse', url: 'http://127.0.0.1:19816/sse' }, null, 2),
    });
  });

  it('servers 包裹层单条 → unwrapped', () => {
    const input = '{ "servers": { "kkwe-mcp": { "type": "sse", "url": "http://127.0.0.1:19816/sse" } } }';
    const result = normalizeServerJson(input);
    expect(result).toEqual({
      kind: 'unwrapped',
      name: 'kkwe-mcp',
      json: JSON.stringify({ type: 'sse', url: 'http://127.0.0.1:19816/sse' }, null, 2),
    });
  });

  it('mcpServers 包裹层单条 → unwrapped', () => {
    const input = '{"mcpServers": {"filesystem": {"command": "npx", "args": ["-y", "x"]}}}';
    const result = normalizeServerJson(input);
    expect(result).toEqual({
      kind: 'unwrapped',
      name: 'filesystem',
      json: JSON.stringify({ command: 'npx', args: ['-y', 'x'] }, null, 2),
    });
  });

  it('包裹层含多个 server → multiple', () => {
    const input = '{"mcpServers": {"a": {"command": "x"}, "b": {"command": "y"}}}';
    const result = normalizeServerJson(input);
    expect(result.kind).toBe('multiple');
    expect(result.kind === 'multiple' && result.names).toEqual(['a', 'b']);
  });

  it('空包裹层（0 个条目）→ invalid', () => {
    expect(normalizeServerJson('{"servers": {}}')).toEqual({ kind: 'invalid' });
    expect(normalizeServerJson('{"mcpServers": {}}')).toEqual({ kind: 'invalid' });
  });

  it('顶层单键但非 servers/mcpServers → 按裸对象处理', () => {
    const input = '{"command": "npx"}';
    expect(normalizeServerJson(input)).toEqual({
      kind: 'bare',
      json: JSON.stringify({ command: 'npx' }, null, 2),
    });
  });

  it('包裹键值不是普通对象（如数组）→ 按裸对象处理', () => {
    const input = '{"servers": [1, 2, 3]}';
    expect(normalizeServerJson(input)).toEqual({
      kind: 'bare',
      json: JSON.stringify({ servers: [1, 2, 3] }, null, 2),
    });
  });

  it('包裹层内某个条目不是普通对象 → 按裸对象处理', () => {
    const input = '{"servers": {"a": "not-an-object"}}';
    expect(normalizeServerJson(input)).toEqual({
      kind: 'bare',
      json: JSON.stringify({ servers: { a: 'not-an-object' } }, null, 2),
    });
  });

  it('格式化幂等：对已 pretty 的裸定义再跑一次结果不变', () => {
    const first = normalizeServerJson('{"type":"sse","url":"http://x"}');
    expect(first.kind).toBe('bare');
    const pretty = first.kind === 'bare' ? first.json : '';
    const second = normalizeServerJson(pretty);
    expect(second).toEqual(first);
  });

  it('格式化幂等：对已解包的裸定义再跑一次仍是 bare 且内容不变', () => {
    const wrapped = normalizeServerJson('{"servers": {"kkwe-mcp": {"type": "sse", "url": "http://x"}}}');
    expect(wrapped.kind).toBe('unwrapped');
    const unwrappedJson = wrapped.kind === 'unwrapped' ? wrapped.json : '';
    const second = normalizeServerJson(unwrappedJson);
    expect(second).toEqual({ kind: 'bare', json: unwrappedJson });
  });
});
