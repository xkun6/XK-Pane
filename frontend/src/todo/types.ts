import type { Todo, CreateTodoInput, UpdateTodoInput } from '@/services';

// TodoContextValue：待办全局状态 + 动作（纯 Context，弃 Zustand；范式同 ProviderProvider）。
// 动作约定：调后端 → reload 刷新列表（不乐观拼接，与后端 sort_order 排序一致）；错误向上抛，
// 由 View 用 NativeDialogs 提示。
export interface TodoContextValue {
  todos: Todo[];
  loading: boolean; // 仅首屏拉取期 true
  refresh: () => Promise<void>;
  addTodo: (input: CreateTodoInput) => Promise<Todo>;
  updateTodo: (id: string, input: UpdateTodoInput) => Promise<void>;
  removeTodo: (id: string) => Promise<void>;
  addSubtask: (todoId: string, title: string) => Promise<void>;
  toggleSubtask: (id: string, completed: boolean) => Promise<void>;
  removeSubtask: (id: string) => Promise<void>;
}
