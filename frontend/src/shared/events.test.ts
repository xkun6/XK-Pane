import { describe, expect, it } from 'vitest';
import { AppEvents } from './events';

// 前后端事件名清单断言（P7 §7 铁律 5「事件名统一单一源 + CI 断言防漂移」）。
// 期望值是 internal/events/events.go 常量字符串的镜像 —— 任何一端改名而忘了同步，这里即红。
const EXPECTED_BACKEND_EVENT_NAMES = {
  Time: 'time',
  LocalHistoryChanged: 'localhistory:changed',
  OrchestratorLaunch: 'orchestrator:launch',
  TerminalOutput: 'terminal:output',
  TerminalStatus: 'terminal:status',
  TerminalExit: 'terminal:exit',
  MCPeOutput: 'mc_pe:output',
  MCPeLaunched: 'mc_pe:launched',
  MCPeExited: 'mc_pe:exited',
} as const;

describe('AppEvents ↔ internal/events/events.go', () => {
  it('每个事件名与后端常量字符串完全一致', () => {
    expect({ ...AppEvents }).toEqual(EXPECTED_BACKEND_EVENT_NAMES);
  });

  it('终端镜像三事件存在（Web 只读镜像核心）', () => {
    expect(AppEvents.TerminalOutput).toBe('terminal:output');
    expect(AppEvents.TerminalStatus).toBe('terminal:status');
    expect(AppEvents.TerminalExit).toBe('terminal:exit');
  });
});
