import { describe, expect, it } from 'vitest';
import { groupByWorkspace } from './groupLaunches';
import type { LaunchRecord } from '@/services';

function rec(p: Partial<LaunchRecord>): LaunchRecord {
  return {
    id: 0,
    projectId: '',
    projectName: '',
    projectPath: '',
    cliTool: 'claude',
    runtimeKind: 'local',
    launchedAt: '',
    workspaceName: null,
    workspacePath: null,
    resumeSessionId: null,
    lastPrompt: null,
    providerId: null,
    launchCwd: null,
    ...p,
  };
}

describe('groupByWorkspace', () => {
  it('drops records without resumeSessionId', () => {
    const groups = groupByWorkspace([rec({ id: 1, workspaceName: 'A' })], '未分组');
    expect(groups).toEqual([]);
  });

  it('groups by workspaceName', () => {
    const groups = groupByWorkspace(
      [
        rec({ id: 1, workspaceName: 'A', resumeSessionId: 's1' }),
        rec({ id: 2, workspaceName: 'A', resumeSessionId: 's2' }),
        rec({ id: 3, workspaceName: 'B', resumeSessionId: 's3' }),
      ],
      '未分组',
    );
    expect(groups.map((g) => g.workspaceName)).toEqual(['A', 'B']);
    expect(groups[0]!.records).toHaveLength(2);
  });

  it('ungrouped sinks to bottom', () => {
    const groups = groupByWorkspace(
      [
        rec({ id: 1, workspaceName: null, resumeSessionId: 's1' }),
        rec({ id: 2, workspaceName: 'Z', resumeSessionId: 's2' }),
        rec({ id: 3, workspaceName: 'A', resumeSessionId: 's3' }),
      ],
      '未分组',
    );
    expect(groups.map((g) => g.workspaceName)).toEqual(['A', 'Z', '未分组']);
  });
});
