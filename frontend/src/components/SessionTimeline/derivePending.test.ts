import { describe, it, expect } from 'vitest';
import type { TimelineEvent, TimelineQuestion } from '@/services';
import { derivePending } from './derivePending';

const Q: TimelineQuestion[] = [{ question: '选哪个？', options: [{ label: 'A' }, { label: 'B' }] }];

let seq = 0;
const ask = (id: string, questions: TimelineQuestion[] = Q): TimelineEvent => ({
  seq: seq++,
  kind: 'tool_use',
  toolName: 'AskUserQuestion',
  toolUseId: id,
  questions,
});
const result = (id: string): TimelineEvent => ({ seq: seq++, kind: 'tool_result', toolUseId: id });
const other = (): TimelineEvent => ({ seq: seq++, kind: 'assistant', text: 'hi' });

describe('derivePending', () => {
  it('无 AskUserQuestion → null', () => {
    expect(derivePending([other()])).toBeNull();
  });

  it('未作答（有 tool_use 无 tool_result）→ 返回问题集（修复 web 漏事件的核心）', () => {
    expect(derivePending([ask('u1')])).toEqual(Q);
  });

  it('已作答（tool_use 有匹配 tool_result）→ null', () => {
    expect(derivePending([ask('u1'), result('u1')])).toBeNull();
  });

  it('多问题：前者已答、后者未答 → 返回后者', () => {
    const q2: TimelineQuestion[] = [{ question: 'Q2', options: [{ label: 'X' }] }];
    expect(derivePending([ask('u1'), result('u1'), ask('u2', q2)])).toEqual(q2);
  });

  it('无 toolUseId → 当已答（避免与转录只读卡重复渲染）→ null', () => {
    expect(
      derivePending([{ seq: 0, kind: 'tool_use', toolName: 'AskUserQuestion', questions: Q }]),
    ).toBeNull();
  });

  it('空 questions → null', () => {
    expect(derivePending([ask('u1', [])])).toBeNull();
  });
});
