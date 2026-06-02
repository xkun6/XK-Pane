import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { TodoService, type Todo, type CreateTodoInput, type UpdateTodoInput } from '@/services';
import type { TodoContextValue } from './types';

const TodoContext = createContext<TodoContextValue | null>(null);

// 待办全局 Provider（SQLite 后端，弃 Zustand）。范式同 ProviderProvider：首屏空列表立即渲染，
// mount 异步拉取；每次 mutation 调后端后 reload 刷新。错误不吞——向上抛给调用方。
export interface TodoProviderProps {
  children: ReactNode;
}

export const TodoProvider = ({ children }: TodoProviderProps) => {
  const [todos, setTodos] = useState<Todo[]>([]);
  const [loading, setLoading] = useState(true);

  const aliveRef = useRef(true);
  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
    };
  }, []);

  const reload = useCallback(async (): Promise<void> => {
    const rows = await TodoService.list();
    if (aliveRef.current) setTodos(rows);
  }, []);

  useEffect(() => {
    let cancelled = false;
    TodoService.list()
      .then((rows) => {
        if (!cancelled && aliveRef.current) setTodos(rows);
      })
      .catch(() => {
        // 后端未就绪 / 异常 —— 保持空列表
      })
      .finally(() => {
        if (!cancelled && aliveRef.current) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const refresh = useCallback((): Promise<void> => reload(), [reload]);

  const addTodo = useCallback(
    async (input: CreateTodoInput): Promise<Todo> => {
      const created = await TodoService.create(input);
      await reload();
      return created;
    },
    [reload],
  );

  const updateTodo = useCallback(
    async (id: string, input: UpdateTodoInput): Promise<void> => {
      await TodoService.update(id, input);
      await reload();
    },
    [reload],
  );

  const removeTodo = useCallback(
    async (id: string): Promise<void> => {
      await TodoService.remove(id);
      await reload();
    },
    [reload],
  );

  const addSubtask = useCallback(
    async (todoId: string, title: string): Promise<void> => {
      await TodoService.addSubtask(todoId, title);
      await reload();
    },
    [reload],
  );

  const toggleSubtask = useCallback(
    async (id: string, completed: boolean): Promise<void> => {
      await TodoService.updateSubtask(id, null, completed);
      await reload();
    },
    [reload],
  );

  const removeSubtask = useCallback(
    async (id: string): Promise<void> => {
      await TodoService.removeSubtask(id);
      await reload();
    },
    [reload],
  );

  const value = useMemo<TodoContextValue>(
    () => ({ todos, loading, refresh, addTodo, updateTodo, removeTodo, addSubtask, toggleSubtask, removeSubtask }),
    [todos, loading, refresh, addTodo, updateTodo, removeTodo, addSubtask, toggleSubtask, removeSubtask],
  );

  return <TodoContext.Provider value={value}>{children}</TodoContext.Provider>;
};

export const useTodos = (): TodoContextValue => {
  const ctx = useContext(TodoContext);
  if (!ctx) {
    throw new Error('useTodos must be used inside <TodoProvider>');
  }
  return ctx;
};
