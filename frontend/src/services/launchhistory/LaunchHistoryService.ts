// LaunchHistoryService：启动历史（launch_history 表）的前端封装。
//
// 每次启动一个项目终端时记录一条（record），将来「最近启动 / 会话恢复」面板用
// list 读取。后端是 cc-pane LaunchHistoryService 的 MVP 子集（Record + List）。
import { Service as Binding } from '@bindings/xkpane/internal/services/launchhistory';
import type { LaunchHistory as RawLaunchHistory } from '@bindings/xkpane/internal/storage/models';

// RecordSpec 对齐后端 launchhistory.RecordSpec（路径铁律 D7：projectPath = PTY cwd）。
export interface RecordSpec {
  projectId: string;
  projectName: string;
  projectPath: string;
  cliTool: string; // 'none' | 适配器 id
  runtimeKind: string; // local | wsl | ssh
  workspaceName?: string | null;
  workspacePath?: string | null;
}

// LaunchRecord 对外纯接口（与生成类解耦）。resumeSessionId 由后端回填（StartBackfill 轮询扫盘
// 发现本次启动对应的可恢复会话 id）—— 「最近启动」据此 resume 续会话；空 = 尚未回填，点了走新会话。
export interface LaunchRecord {
  id: number;
  projectId: string;
  projectName: string;
  projectPath: string;
  cliTool: string;
  runtimeKind: string;
  launchedAt: string; // 后端 time.Time 序列化（UTC RFC3339）
  workspaceName: string | null;
  workspacePath: string | null;
  /** 可恢复会话 id（后端回填；明文 → CLI resume）；空 = 未回填（无法 resume，点击走新会话）。 */
  resumeSessionId: string | null;
}

function toRecord(r: RawLaunchHistory): LaunchRecord {
  return {
    id: Number(r.id ?? 0),
    projectId: r.projectId ?? '',
    projectName: r.projectName ?? '',
    projectPath: r.projectPath ?? '',
    cliTool: r.cliTool ?? 'none',
    runtimeKind: r.runtimeKind ?? 'local',
    launchedAt: typeof r.launchedAt === 'string' ? r.launchedAt : '',
    workspaceName: r.workspaceName ?? null,
    workspacePath: r.workspacePath ?? null,
    resumeSessionId: r.resumeSessionId ?? null,
  };
}

export const LaunchHistoryService = {
  // 记录一次启动，返回创建出的记录。
  async record(spec: RecordSpec): Promise<LaunchRecord> {
    const r = await Binding.Record({
      projectId: spec.projectId,
      projectName: spec.projectName,
      projectPath: spec.projectPath,
      cliTool: spec.cliTool,
      runtimeKind: spec.runtimeKind,
      workspaceName: spec.workspaceName ?? null,
      workspacePath: spec.workspacePath ?? null,
    });
    return toRecord(r);
  },

  // 最近 limit 条（倒序，最新在前）；limit<=0 不限。
  async list(limit: number): Promise<LaunchRecord[]> {
    const rows = await Binding.List(limit);
    return (rows ?? []).map(toRecord);
  },

  // 会话创建成功后触发：后端后台轮询扫盘发现本次启动的可恢复会话 id，回填到对应启动记录。
  // fire-and-forget（后端立即返回，轮询在后台）；none/裸 shell 后端自动 no-op。
  async startBackfill(spec: {
    projectPath: string;
    workspacePath?: string | null;
    cliTool: string;
    ptySessionId: string;
  }): Promise<void> {
    await Binding.StartBackfill(
      spec.projectPath,
      spec.workspacePath ?? '',
      spec.cliTool,
      spec.ptySessionId,
    );
  },
} as const;
