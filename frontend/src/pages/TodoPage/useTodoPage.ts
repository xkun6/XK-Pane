import { useCallback, useEffect, useRef, useState } from 'react';
import { NativeDialogs, TodoService, type Todo, type TodoStats } from '@/services';
import { useTodos } from '@/todo';

// TodoPage ViewModel：消费全局 TodoProvider + 管理「新建待办」输入草稿 + 过滤/搜索 + 编排原生对话框。
// 不返回 JSX、不持翻译串——提示文案由 View（持 t()）传入（MVVM 铁律）。
//
// 列表数据源「切 List→Query」：无过滤时直接用 Provider 全量 todos（不分页）；任一过滤/搜索激活时
// 切到服务端 `Query`（消费已就位的后端，让过滤可验）。Provider 仍是变更权威——mutation 走 Provider，
// 其 todos 变化既触发统计重取、也触发已激活过滤的重查询。

export interface RemoveStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
  errorTitle: string;
}

export interface UseTodoPageResult {
  todos: Todo[]; // 展示用列表：无过滤=Provider 全量，有过滤=Query 结果
  loading: boolean;
  stats: TodoStats | null;
  statusFilter: string;
  setStatusFilter: (v: string) => void;
  priorityFilter: string;
  setPriorityFilter: (v: string) => void;
  search: string;
  setSearch: (v: string) => void;
  hasActiveFilter: boolean;
  querying: boolean;
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
  const [stats, setStats] = useState<TodoStats | null>(null);
  const [statusFilter, setStatusFilter] = useState('');
  const [priorityFilter, setPriorityFilter] = useState('');
  const [search, setSearch] = useState('');
  const [queryItems, setQueryItems] = useState<Todo[]>([]);
  const [querying, setQuerying] = useState(false);
  const queryReqRef = useRef(0);

  const hasActiveFilter = statusFilter !== '' || priorityFilter !== '' || search.trim() !== '';

  // 统计由后端服务端聚合（含 overdue 日期逻辑），始终反映全量（不随过滤变）。每当 Provider todos
  // 变化（增/删/勾选）重取，保持同步。
  useEffect(() => {
    let alive = true;
    TodoService.stats()
      .then((s) => {
        if (alive) setStats(s);
      })
      .catch(() => {
        /* 统计失败不打扰主流程（列表仍可用） */
      });
    return () => {
      alive = false;
    };
  }, [todos]);

  // 过滤/搜索激活时走服务端 Query；防抖 200ms（搜索连打合并），reqId 丢弃过期响应。todos 入依赖
  // 使 mutation 后自动重查（保持过滤视图与底层一致）。limit 取大值：桌面单库，过滤已收窄结果集。
  useEffect(() => {
    if (!hasActiveFilter) {
      queryReqRef.current++;
      setQueryItems([]);
      setQuerying(false);
      return;
    }
    const myReq = ++queryReqRef.current;
    setQuerying(true);
    const handle = setTimeout(() => {
      TodoService.query({
        status: statusFilter || undefined,
        priority: priorityFilter || undefined,
        search: search.trim() || undefined,
        limit: 500,
      })
        .then((r) => {
          if (myReq === queryReqRef.current) setQueryItems(r.items);
        })
        .catch(() => {
          if (myReq === queryReqRef.current) setQueryItems([]);
        })
        .finally(() => {
          if (myReq === queryReqRef.current) setQuerying(false);
        });
    }, 200);
    return () => clearTimeout(handle);
  }, [statusFilter, priorityFilter, search, todos, hasActiveFilter]);

  const displayedTodos = hasActiveFilter ? queryItems : todos;

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
    todos: displayedTodos,
    loading,
    stats,
    statusFilter,
    setStatusFilter,
    priorityFilter,
    setPriorityFilter,
    search,
    setSearch,
    hasActiveFilter,
    querying,
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
