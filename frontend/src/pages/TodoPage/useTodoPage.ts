import { useCallback, useState } from 'react';
import { NativeDialogs, type Todo } from '@/services';
import { useTodos } from '@/todo';

// TodoPage ViewModel：消费全局 TodoProvider + 管理「新建待办」输入草稿 + 编排原生对话框。
// 不返回 JSX、不持翻译串——提示文案由 View（持 t()）传入（MVVM 铁律）。

export interface RemoveStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
  errorTitle: string;
}

export interface UseTodoPageResult {
  todos: Todo[];
  loading: boolean;
  newTitle: string;
  setNewTitle: (v: string) => void;
  submitNew: (errorTitle: string) => Promise<void>;
  toggleStatus: (t: Todo, errorTitle: string) => Promise<void>;
  remove: (t: Todo, s: RemoveStrings) => Promise<void>;
  addSubtask: (todoId: string, title: string, errorTitle: string) => Promise<void>;
  toggleSubtask: (id: string, completed: boolean, errorTitle: string) => Promise<void>;
  removeSubtask: (id: string, errorTitle: string) => Promise<void>;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export const useTodoPage = (): UseTodoPageResult => {
  const { todos, loading, addTodo, updateTodo, removeTodo, addSubtask, toggleSubtask, removeSubtask } = useTodos();
  const [newTitle, setNewTitle] = useState('');

  const submitNew = useCallback(
    async (errorTitle: string): Promise<void> => {
      const title = newTitle.trim();
      if (!title) return;
      try {
        await addTodo({ title });
        setNewTitle('');
      } catch (e) {
        await NativeDialogs.error(errorTitle, messageOf(e));
      }
    },
    [newTitle, addTodo],
  );

  const toggleStatus = useCallback(
    async (t: Todo, errorTitle: string): Promise<void> => {
      const next = t.status === 'done' ? 'todo' : 'done';
      try {
        await updateTodo(t.id, { status: next });
      } catch (e) {
        await NativeDialogs.error(errorTitle, messageOf(e));
      }
    },
    [updateTodo],
  );

  const remove = useCallback(
    async (t: Todo, s: RemoveStrings): Promise<void> => {
      const ok = await NativeDialogs.confirm({
        title: s.confirmTitle,
        message: s.confirmMessage,
        okLabel: s.okLabel,
        cancelLabel: s.cancelLabel,
      });
      if (!ok) return;
      try {
        await removeTodo(t.id);
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      }
    },
    [removeTodo],
  );

  const onAddSubtask = useCallback(
    async (todoId: string, title: string, errorTitle: string): Promise<void> => {
      const trimmed = title.trim();
      if (!trimmed) return;
      try {
        await addSubtask(todoId, trimmed);
      } catch (e) {
        await NativeDialogs.error(errorTitle, messageOf(e));
      }
    },
    [addSubtask],
  );

  const onToggleSubtask = useCallback(
    async (id: string, completed: boolean, errorTitle: string): Promise<void> => {
      try {
        await toggleSubtask(id, completed);
      } catch (e) {
        await NativeDialogs.error(errorTitle, messageOf(e));
      }
    },
    [toggleSubtask],
  );

  const onRemoveSubtask = useCallback(
    async (id: string, errorTitle: string): Promise<void> => {
      try {
        await removeSubtask(id);
      } catch (e) {
        await NativeDialogs.error(errorTitle, messageOf(e));
      }
    },
    [removeSubtask],
  );

  return {
    todos,
    loading,
    newTitle,
    setNewTitle,
    submitNew,
    toggleStatus,
    remove,
    addSubtask: onAddSubtask,
    toggleSubtask: onToggleSubtask,
    removeSubtask: onRemoveSubtask,
  };
};
