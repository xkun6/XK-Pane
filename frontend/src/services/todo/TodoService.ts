// TodoService：待办（todos / todo_subtasks）的前端封装。后端走 SQLite（Holder），
// 本服务把 Wails 生成的 binding 收口为干净的 TS 类型。tags 后端以 JSON 数组存储，跨 IPC 已是 string[]。
import {
  Service as Binding,
  CreateTodoInput as RawCreateInput,
  UpdateTodoInput as RawUpdateInput,
} from '@bindings/xkpane/internal/services/todo';
import type { TodoDTO as RawTodo, TodoStats as RawStats } from '@bindings/xkpane/internal/services/todo/models';
import type { TodoSubtask as RawSubtask } from '@bindings/xkpane/internal/storage/models';

export interface TodoSubtask {
  id: string;
  todoId: string;
  title: string;
  completed: boolean;
  sortOrder: number;
}

export interface Todo {
  id: string;
  title: string;
  description: string;
  status: string; // todo | in_progress | done
  priority: string; // high | medium | low
  scope: string;
  tags: string[];
  dueDate: string | null;
  sortOrder: number;
  myDay: boolean;
  todoType: string;
  subtasks: TodoSubtask[];
}

// 新建：仅 title 必填，其余可选（后端补默认 todo/medium/global）。
export interface CreateTodoInput {
  title: string;
  description?: string;
  status?: string;
  priority?: string;
  tags?: string[];
}

// 编辑：全部可选，仅更新提供的字段（patch）。
export interface UpdateTodoInput {
  title?: string;
  description?: string;
  status?: string;
  priority?: string;
  tags?: string[];
}

// 待办统计（按维度计数 + 过期数，服务端聚合）。
export interface TodoStats {
  total: number;
  byStatus: Record<string, number>;
  byScope: Record<string, number>;
  byPriority: Record<string, number>;
  overdue: number;
}

function toCountMap(m: Record<string, number | undefined> | undefined): Record<string, number> {
  const out: Record<string, number> = {};
  for (const [k, v] of Object.entries(m ?? {})) out[k] = v ?? 0;
  return out;
}

function toSubtask(s: RawSubtask): TodoSubtask {
  return {
    id: s.id ?? '',
    todoId: s.todoId ?? '',
    title: s.title ?? '',
    completed: s.completed ?? false,
    sortOrder: s.sortOrder ?? 0,
  };
}

function toTodo(t: RawTodo): Todo {
  return {
    id: t.id ?? '',
    title: t.title ?? '',
    description: t.description ?? '',
    status: t.status ?? 'todo',
    priority: t.priority ?? 'medium',
    scope: t.scope ?? 'global',
    tags: t.tags ?? [],
    dueDate: t.dueDate ?? null,
    sortOrder: t.sortOrder ?? 0,
    myDay: t.myDay ?? false,
    todoType: t.todoType ?? '',
    subtasks: (t.subtasks ?? []).map(toSubtask),
  };
}

export const TodoService = {
  async list(): Promise<Todo[]> {
    const rows = await Binding.List();
    return (rows ?? []).map(toTodo);
  },

  async get(id: string): Promise<Todo | null> {
    const t = await Binding.Get(id);
    return t ? toTodo(t) : null;
  },

  async create(input: CreateTodoInput): Promise<Todo> {
    // 用生成的 binding 类构造（接受 Partial，自动补其余字段默认；后端按指针 nil 走默认）。
    return toTodo(
      await Binding.Create(
        new RawCreateInput({
          title: input.title,
          description: input.description,
          status: input.status,
          priority: input.priority,
          tags: input.tags,
        }),
      ),
    );
  },

  async update(id: string, input: UpdateTodoInput): Promise<void> {
    await Binding.Update(
      id,
      new RawUpdateInput({
        title: input.title,
        description: input.description,
        status: input.status,
        priority: input.priority,
        tags: input.tags,
      }),
    );
  },

  async remove(id: string): Promise<void> {
    await Binding.Delete(id);
  },

  async addSubtask(todoId: string, title: string): Promise<TodoSubtask> {
    return toSubtask(await Binding.AddSubtask(todoId, title));
  },

  async updateSubtask(id: string, title: string | null, completed: boolean | null): Promise<void> {
    await Binding.UpdateSubtask(id, title, completed);
  },

  async removeSubtask(id: string): Promise<void> {
    await Binding.DeleteSubtask(id);
  },

  // 服务端聚合统计（总数 / 按 status·scope·priority 计数 / 过期数）。scope·scopeRef 可选限定范围。
  async stats(scope?: string, scopeRef?: string): Promise<TodoStats> {
    const s: RawStats = await Binding.Stats(scope ?? null, scopeRef ?? null);
    return {
      total: s.total ?? 0,
      byStatus: toCountMap(s.byStatus),
      byScope: toCountMap(s.byScope),
      byPriority: toCountMap(s.byPriority),
      overdue: s.overdue ?? 0,
    };
  },
} as const;
