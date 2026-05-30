// 启动记录按工作空间分组 —— 忠实移植 cc-pane web/utils/groupLaunches.ts。
// 纯函数（可 Vitest 覆盖）：只保留有 resumeSessionId 的记录；无 workspaceName 归「未分组」，
// 「未分组」组永远沉底，其余按 workspaceName localeCompare。
import type { LaunchRecord } from '@/services';

export interface WorkspaceGroup {
  workspaceName: string;
  records: LaunchRecord[];
}

export function groupByWorkspace(records: LaunchRecord[], ungroupedLabel: string): WorkspaceGroup[] {
  const filtered = records.filter((r) => !!r.resumeSessionId);
  const map = new Map<string, LaunchRecord[]>();

  for (const record of filtered) {
    const key = record.workspaceName || ungroupedLabel;
    const list = map.get(key);
    if (list) {
      list.push(record);
    } else {
      map.set(key, [record]);
    }
  }

  const groups: WorkspaceGroup[] = [];
  for (const [workspaceName, recs] of map) {
    groups.push({ workspaceName, records: recs });
  }

  groups.sort((a, b) => {
    if (a.workspaceName === ungroupedLabel) return 1;
    if (b.workspaceName === ungroupedLabel) return -1;
    return a.workspaceName.localeCompare(b.workspaceName);
  });

  return groups;
}
