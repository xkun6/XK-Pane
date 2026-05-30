import { describe, expect, it } from 'vitest';
import { applyStatus, emptyStatusMap, isSessionActive } from './terminalStatus';

describe('applyStatus', () => {
  it('writes a new status', () => {
    const next = applyStatus(emptyStatusMap, { sessionId: 's1', status: 'active' });
    expect(next).toEqual({ s1: 'active' });
  });

  it('returns same map reference when status unchanged (no re-render)', () => {
    const map = applyStatus(emptyStatusMap, { sessionId: 's1', status: 'active' });
    const again = applyStatus(map, { sessionId: 's1', status: 'active' });
    expect(again).toBe(map);
  });

  it('overwrites with latest status', () => {
    const map = applyStatus(emptyStatusMap, { sessionId: 's1', status: 'idle' });
    const next = applyStatus(map, { sessionId: 's1', status: 'active' });
    expect(next.s1).toBe('active');
  });

  it('deletes entry on exited', () => {
    const map = applyStatus(emptyStatusMap, { sessionId: 's1', status: 'active' });
    const next = applyStatus(map, { sessionId: 's1', status: 'exited' });
    expect('s1' in next).toBe(false);
  });

  it('exited on unknown session returns same map', () => {
    const next = applyStatus(emptyStatusMap, { sessionId: 'ghost', status: 'exited' });
    expect(next).toBe(emptyStatusMap);
  });

  it('does not mutate the input map', () => {
    const map = applyStatus(emptyStatusMap, { sessionId: 's1', status: 'active' });
    applyStatus(map, { sessionId: 's2', status: 'active' });
    expect(map).toEqual({ s1: 'active' });
  });
});

describe('isSessionActive', () => {
  it('true only for active', () => {
    const map = applyStatus(emptyStatusMap, { sessionId: 's1', status: 'active' });
    expect(isSessionActive(map, 's1')).toBe(true);
    expect(isSessionActive(map, 's2')).toBe(false);
    expect(isSessionActive(map, null)).toBe(false);
    expect(isSessionActive(map, undefined)).toBe(false);
  });

  it('false for idle / non-active', () => {
    const map = applyStatus(emptyStatusMap, { sessionId: 's1', status: 'idle' });
    expect(isSessionActive(map, 's1')).toBe(false);
  });
});
