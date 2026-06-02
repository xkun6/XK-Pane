// TaskBindingService：编排任务绑定（task_bindings）的前端封装。记录一次「分派」(Todo→会话)
// 及其状态/进度。后端走 SQLite（Holder），本服务把 Wails binding 收口为干净 TS 类型。
import {
  Service as Binding,
  CreateInput as RawCreate,
  UpdateInput as RawUpdate,
  Query as RawQuery,
} from '@bindings/xkpane/internal/services/taskbinding';
import type { QueryResult as RawQueryResult } from '@bindings/xkpane/internal/services/taskbinding/models';
import type { TaskBinding as RawBinding } from '@bindings/xkpane/internal/storage/models';

export interface TaskBinding {
  id: string;
  title: string;
  prompt: string | null;
  sessionId: string | null;
  todoId: string | null;
  projectPath: string;
  workspaceName: string | null;
  cliTool: string;
  status: string; // pending | running | waiting | completed | failed
  progress: number;
  completionSummary: string | null;
  exitCode: number | null;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
}

// 新建：title / projectPath 必填，其余可选（后端默认 status=pending / progress=0 / cliTool=reclaude）。
export interface CreateTaskBindingInput {
  title: string;
  prompt?: string;
  sessionId?: string;
  todoId?: string;
  projectPath: string;
  workspaceName?: string;
  cliTool?: string;
}

// 编辑：全部可选，仅更新提供的字段（patch）。
export interface UpdateTaskBindingInput {
  title?: string;
  prompt?: string;
  sessionId?: string;
  status?: string;
  progress?: number;
  completionSummary?: string;
  exitCode?: number;
  sortOrder?: number;
}

export interface TaskBindingQuery {
  status?: string;
  projectPath?: string;
  workspaceName?: string;
  search?: string;
  limit?: number;
  offset?: number;
}

export interface TaskBindingQueryResult {
  items: TaskBinding[];
  total: number;
  hasMore: boolean;
}

function toBinding(b: RawBinding): TaskBinding {
  return {
    id: b.id ?? '',
    title: b.title ?? '',
    prompt: b.prompt ?? null,
    sessionId: b.sessionId ?? null,
    todoId: b.todoId ?? null,
    projectPath: b.projectPath ?? '',
    workspaceName: b.workspaceName ?? null,
    cliTool: b.cliTool ?? 'reclaude',
    status: b.status ?? 'pending',
    progress: b.progress ?? 0,
    completionSummary: b.completionSummary ?? null,
    exitCode: b.exitCode ?? null,
    sortOrder: b.sortOrder ?? 0,
    createdAt: (b.createdAt as unknown as string) ?? '',
    updatedAt: (b.updatedAt as unknown as string) ?? '',
  };
}

export const TaskBindingService = {
  async create(input: CreateTaskBindingInput): Promise<TaskBinding> {
    return toBinding(
      await Binding.Create(
        new RawCreate({
          title: input.title,
          prompt: input.prompt,
          sessionId: input.sessionId,
          todoId: input.todoId,
          projectPath: input.projectPath,
          workspaceName: input.workspaceName,
          cliTool: input.cliTool,
        }),
      ),
    );
  },

  async get(id: string): Promise<TaskBinding | null> {
    const b = await Binding.Get(id);
    return b ? toBinding(b) : null;
  },

  async findBySession(sessionId: string): Promise<TaskBinding | null> {
    const b = await Binding.FindBySession(sessionId);
    return b ? toBinding(b) : null;
  },

  async update(id: string, input: UpdateTaskBindingInput): Promise<TaskBinding> {
    return toBinding(
      await Binding.Update(
        id,
        new RawUpdate({
          title: input.title,
          prompt: input.prompt,
          sessionId: input.sessionId,
          status: input.status,
          progress: input.progress,
          completionSummary: input.completionSummary,
          exitCode: input.exitCode,
          sortOrder: input.sortOrder,
        }),
      ),
    );
  },

  async remove(id: string): Promise<void> {
    await Binding.Delete(id);
  },

  async query(q: TaskBindingQuery): Promise<TaskBindingQueryResult> {
    const res: RawQueryResult = await Binding.Query(
      new RawQuery({
        status: q.status,
        projectPath: q.projectPath,
        workspaceName: q.workspaceName,
        search: q.search,
        limit: q.limit,
        offset: q.offset,
      }),
    );
    return {
      items: (res.items ?? []).map(toBinding),
      total: res.total ?? 0,
      hasMore: res.hasMore ?? false,
    };
  },
} as const;
