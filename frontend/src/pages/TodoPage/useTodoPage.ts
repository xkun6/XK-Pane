import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  TodoService,
  PreferencesService,
  NativeDialogs,
  TerminalService,
  TaskBindingService,
  type Todo,
  type TodoStats,
} from '@/services';
import { useTerminal } from '@/terminal';
import { useRouter } from '@/router';

// ViewModel：三栏 TodoManager 的全部状态与回调（自包含 query 模型，忠实移植 cc-pane useTodoStore）。
// 不依赖全局 Provider；不返回 JSX；不持翻译串——只暴露数据 + 回调 + 选项「值」，标签由 View 经 t() 渲染。
// 需要原生确认文案的操作（删除）由 View 注入已翻译字符串（ConfirmStrings）。

export type ViewMode = 'all' | 'my_day';
export type GroupMode = 'none' | 'tag' | 'status' | 'priority' | 'scope';

// 预定义类型（不可删除），对齐 cc-pane BUILTIN_TODO_TYPES。
export const BUILTIN_TODO_TYPES = ['feature', 'bug', 'docs', 'chore'] as const;
const CUSTOM_TYPES_KEY = 'todo.customTypes';

// 编辑表单（新建/编辑共用，全受控）。
export interface TodoEditForm {
  title: string;
  description: string;
  status: string;
  priority: string;
  scope: string;
  scopeRef: string;
  tags: string[];
  todoType: string;
  dueDate: string;
  reminderAt: string;
  recurrence: string;
}

// 删除确认文案（View 经 t() 注入）。
export interface ConfirmStrings {
  title: string;
  message: string;
  okLabel: string;
  cancelLabel: string;
}

const EMPTY_FORM: TodoEditForm = {
  title: '',
  description: '',
  status: 'todo',
  priority: 'medium',
  scope: 'global',
  scopeRef: '',
  tags: [],
  todoType: '',
  dueDate: '',
  reminderAt: '',
  recurrence: '',
};

// 本地时区今天（YYYY-MM-DD），对齐 cc-pane chrono::Local 的 my_day_date。
function todayLocal(): string {
  const d = new Date();
  const m = `${d.getMonth() + 1}`.padStart(2, '0');
  const day = `${d.getDate()}`.padStart(2, '0');
  return `${d.getFullYear()}-${m}-${day}`;
}

function formFromTodo(t: Todo): TodoEditForm {
  return {
    title: t.title,
    description: t.description,
    status: t.status,
    priority: t.priority,
    scope: t.scope,
    scopeRef: t.scopeRef ?? '',
    tags: t.tags,
    todoType: t.todoType,
    dueDate: t.dueDate ?? '',
    reminderAt: t.reminderAt ?? '',
    recurrence: t.recurrence ?? '',
  };
}

// 状态三态循环 todo → in_progress → done → todo（对齐 cc-pane handleToggleStatus）。
function nextStatus(s: string): string {
  if (s === 'todo') return 'in_progress';
  if (s === 'in_progress') return 'done';
  return 'todo';
}

// 分派 prompt 模板（对齐 ccpanes-dispatch-todos skill 的「任务来源/内容/要求」结构）。
// 这是发给 CLI Agent 的 prompt（机器面，非 UI 文案），故为代码常量、不走 i18n。
function buildDispatchPrompt(todo: Todo): string {
  const lines = ['## 任务来源', '', '来自 XK-Pane Todo 系统的待办任务。', '', '## 任务内容', '', `标题: ${todo.title}`];
  if (todo.description) lines.push(`描述: ${todo.description}`);
  lines.push(`优先级: ${todo.priority}`, '', '## 要求', '', '完成上述任务。完成后告知用户。');
  return lines.join('\n');
}

export const useTodoPage = () => {
  const [viewMode, setViewMode] = useState<ViewMode>('all');
  const [filterScope, setFilterScope] = useState<string | null>(null);
  const [filterStatus, setFilterStatus] = useState<string | null>(null);
  const [filterPriority, setFilterPriority] = useState<string | null>(null);
  const [filterType, setFilterType] = useState<string | null>(null);
  const [searchText, setSearchText] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [groupMode, setGroupMode] = useState<GroupMode>('none');

  const [todos, setTodos] = useState<Todo[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);

  const [selectedTodo, setSelectedTodo] = useState<Todo | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [editForm, setEditForm] = useState<TodoEditForm>(EMPTY_FORM);

  const [stats, setStats] = useState<TodoStats | null>(null);
  const [customTypes, setCustomTypes] = useState<string[]>([]);

  // 分派依赖：终端启动入口（newTab 绑定预建会话）+ 路由跳转。
  const { newTab, activePaneId } = useTerminal();
  const { navigate } = useRouter();

  const aliveRef = useRef(true);
  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
    };
  }, []);

  // 自定义类型：走 SQLite Preferences（禁 localStorage 铁律），mount 拉取。
  useEffect(() => {
    PreferencesService.get<string[]>(CUSTOM_TYPES_KEY)
      .then((v) => {
        if (aliveRef.current && Array.isArray(v)) setCustomTypes(v);
      })
      .catch(() => {});
  }, []);

  // 搜索 300ms 去抖。
  useEffect(() => {
    const id = setTimeout(() => setDebouncedSearch(searchText.trim()), 300);
    return () => clearTimeout(id);
  }, [searchText]);

  const loadList = useCallback(async (): Promise<void> => {
    setLoading(true);
    try {
      const res = await TodoService.query({
        status: filterStatus ?? undefined,
        priority: filterPriority ?? undefined,
        scope: filterScope ?? undefined,
        todoType: filterType ?? undefined,
        search: debouncedSearch || undefined,
        myDay: viewMode === 'my_day' ? true : undefined,
        limit: 200,
      });
      if (!aliveRef.current) return;
      setTodos(res.items);
      setTotal(res.total);
    } catch {
      // 后端未就绪 / 异常 —— 保持当前列表
    } finally {
      if (aliveRef.current) setLoading(false);
    }
  }, [filterStatus, filterPriority, filterScope, filterType, debouncedSearch, viewMode]);

  // 过滤 / 视图 / 作用域 / 搜索变化即重查（对齐 cc-pane 每个 setter 后 loadList）。
  useEffect(() => {
    void loadList();
  }, [loadList]);

  // 列表加载后自动选中第一条（对齐 cc-pane）：有任务时编辑器自动出现；空态右栏收起为 0 宽。
  useEffect(() => {
    if (!loading && todos.length > 0 && !selectedTodo && !isCreating) {
      const first = todos[0];
      if (first) setSelectedTodo(first);
    }
  }, [loading, todos, selectedTodo, isCreating]);

  const loadStats = useCallback(async (): Promise<void> => {
    try {
      const s = await TodoService.stats(filterScope ?? undefined);
      if (aliveRef.current) setStats(s);
    } catch {
      // 统计失败不影响主流程
    }
  }, [filterScope]);

  // 选中项变化 / 进入新建：同步 editForm。
  useEffect(() => {
    if (isCreating) {
      setEditForm({ ...EMPTY_FORM, scope: filterScope ?? 'global' });
    } else if (selectedTodo) {
      setEditForm(formFromTodo(selectedTodo));
    }
  }, [selectedTodo, isCreating, filterScope]);

  const refreshSelected = useCallback(async (id: string): Promise<void> => {
    const fresh = await TodoService.get(id);
    if (!aliveRef.current) return;
    if (fresh) {
      setSelectedTodo((prev) => (prev?.id === id ? fresh : prev));
      setTodos((list) => list.map((t) => (t.id === id ? fresh : t)));
    }
  }, []);

  // ── 选择 / 新建 / 关闭 ──────────────────────────────────────────────────
  const selectTodo = useCallback((todo: Todo | null): void => {
    setIsCreating(false);
    setSelectedTodo(todo);
  }, []);

  const startCreate = useCallback((): void => {
    setSelectedTodo(null);
    setIsCreating(true);
  }, []);

  const closeEditor = useCallback((): void => {
    setSelectedTodo(null);
    setIsCreating(false);
  }, []);

  const patchForm = useCallback((patch: Partial<TodoEditForm>): void => {
    setEditForm((f) => ({ ...f, ...patch }));
  }, []);

  // ── 保存（新建 / 编辑）──────────────────────────────────────────────────
  const saveForm = useCallback(async (): Promise<void> => {
    const title = editForm.title.trim();
    if (!title) return;
    if (isCreating) {
      const created = await TodoService.create({
        title,
        description: editForm.description || undefined,
        status: editForm.status,
        priority: editForm.priority,
        scope: editForm.scope,
        scopeRef: editForm.scopeRef.trim() || undefined,
        tags: editForm.tags,
        todoType: editForm.todoType || undefined,
        dueDate: editForm.dueDate || undefined,
        reminderAt: editForm.reminderAt || undefined,
        recurrence: editForm.recurrence || undefined,
      });
      await loadList();
      if (aliveRef.current) {
        setIsCreating(false);
        setSelectedTodo(created);
      }
    } else if (selectedTodo) {
      await TodoService.update(selectedTodo.id, {
        title,
        description: editForm.description,
        status: editForm.status,
        priority: editForm.priority,
        scope: editForm.scope,
        scopeRef: editForm.scopeRef.trim(),
        tags: editForm.tags,
        todoType: editForm.todoType,
        dueDate: editForm.dueDate,
        reminderAt: editForm.reminderAt,
        recurrence: editForm.recurrence,
      });
      await loadList();
      await refreshSelected(selectedTodo.id);
    }
  }, [editForm, isCreating, selectedTodo, loadList, refreshSelected]);

  // ── 行内交互 ────────────────────────────────────────────────────────────
  const toggleStatus = useCallback(
    async (todo: Todo): Promise<void> => {
      await TodoService.update(todo.id, { status: nextStatus(todo.status) });
      await loadList();
      if (selectedTodo?.id === todo.id) await refreshSelected(todo.id);
    },
    [loadList, refreshSelected, selectedTodo],
  );

  const toggleMyDay = useCallback(
    async (todo: Todo): Promise<void> => {
      const next = !todo.myDay;
      await TodoService.update(todo.id, { myDay: next, myDayDate: next ? todayLocal() : '' });
      await loadList();
      if (selectedTodo?.id === todo.id) await refreshSelected(todo.id);
    },
    [loadList, refreshSelected, selectedTodo],
  );

  // 删除：View 注入已翻译的确认文案（MVVM 铁律：VM 不持翻译串）。
  const deleteTodo = useCallback(
    async (todo: Todo, strings: ConfirmStrings): Promise<void> => {
      const ok = await NativeDialogs.confirm({
        title: strings.title,
        message: strings.message,
        okLabel: strings.okLabel,
        cancelLabel: strings.cancelLabel,
      }).catch(() => false);
      if (!ok) return;
      await TodoService.remove(todo.id);
      await loadList();
      if (aliveRef.current && selectedTodo?.id === todo.id) closeEditor();
    },
    [loadList, selectedTodo, closeEditor],
  );

  const reorder = useCallback(
    async (ids: string[]): Promise<void> => {
      await TodoService.reorder(ids);
      await loadList();
    },
    [loadList],
  );

  // ── 分派（结合 ccpane）：把 Todo 交给一个 Agent 会话执行 ──────────────────
  // 1) 先建会话（注入 todo 内容作 initialPrompt）→ 2) newTab 绑定该会话（编排场景，reconcile 不再建）
  // → 3) 落库 TaskBinding(todoId↔sessionId) → 4) Todo 标 in_progress → 5) 跳转终端页。
  const dispatchTodo = useCallback(
    async (todo: Todo, project: { id: string; name: string; path: string }, cliTool: string): Promise<void> => {
      const prompt = buildDispatchPrompt(todo);
      const sessionId = await TerminalService.create({
        projectPath: project.path,
        cliTool,
        initialPrompt: prompt,
      });
      newTab(activePaneId, {
        projectId: project.id,
        projectPath: project.path,
        cliTool,
        sessionId,
      });
      await TaskBindingService.create({
        todoId: todo.id,
        sessionId,
        projectPath: project.path,
        title: todo.title,
        prompt,
        cliTool,
      }).catch(() => {
        // TaskBinding 落库失败不阻断分派（会话已起）
      });
      await TodoService.update(todo.id, { status: 'in_progress' });
      await loadList();
      if (selectedTodo?.id === todo.id) await refreshSelected(todo.id);
      navigate('terminal');
    },
    [newTab, activePaneId, navigate, loadList, selectedTodo, refreshSelected],
  );

  // ── 子任务 ──────────────────────────────────────────────────────────────
  const addSubtask = useCallback(
    async (todoId: string, title: string): Promise<void> => {
      const t = title.trim();
      if (!t) return;
      await TodoService.addSubtask(todoId, t);
      await refreshSelected(todoId);
    },
    [refreshSelected],
  );

  const toggleSubtask = useCallback(
    async (todoId: string, subtaskId: string, completed: boolean): Promise<void> => {
      await TodoService.updateSubtask(subtaskId, null, completed);
      await refreshSelected(todoId);
    },
    [refreshSelected],
  );

  const removeSubtask = useCallback(
    async (todoId: string, subtaskId: string): Promise<void> => {
      await TodoService.removeSubtask(subtaskId);
      await refreshSelected(todoId);
    },
    [refreshSelected],
  );

  // ── 自定义类型（走 Preferences）──────────────────────────────────────────
  const persistCustomTypes = useCallback((next: string[]): void => {
    setCustomTypes(next);
    void PreferencesService.set(CUSTOM_TYPES_KEY, next);
  }, []);

  const addCustomType = useCallback(
    (type: string): void => {
      const t = type.trim().toLowerCase();
      if (!t || customTypes.includes(t) || (BUILTIN_TODO_TYPES as readonly string[]).includes(t)) return;
      persistCustomTypes([...customTypes, t]);
    },
    [customTypes, persistCustomTypes],
  );

  const removeCustomType = useCallback(
    (type: string): void => {
      persistCustomTypes(customTypes.filter((t) => t !== type));
    },
    [customTypes, persistCustomTypes],
  );

  // 客户端分组（groupMode !== none 时按 tag/status/priority/scope 分组），对齐 cc-pane groups useMemo。
  const groups = useMemo((): Map<string, Todo[]> | null => {
    if (groupMode === 'none') return null;
    const result = new Map<string, Todo[]>();
    for (const todo of todos) {
      const keys =
        groupMode === 'tag'
          ? todo.tags.length > 0
            ? todo.tags
            : ['__untagged__']
          : groupMode === 'status'
            ? [todo.status]
            : groupMode === 'priority'
              ? [todo.priority]
              : [todo.scope];
      for (const key of keys) {
        const list = result.get(key) ?? [];
        list.push(todo);
        result.set(key, list);
      }
    }
    return result;
  }, [todos, groupMode]);

  // 全部类型选项（内置 + 自定义去重），供过滤条 / 编辑器类型药丸用。
  const allTypes = useMemo(
    () => [
      ...BUILTIN_TODO_TYPES,
      ...customTypes.filter((t) => !(BUILTIN_TODO_TYPES as readonly string[]).includes(t)),
    ],
    [customTypes],
  );

  return {
    // 状态
    viewMode,
    filterScope,
    filterStatus,
    filterPriority,
    filterType,
    searchText,
    groupMode,
    todos,
    total,
    loading,
    selectedTodo,
    isCreating,
    editForm,
    stats,
    customTypes,
    allTypes,
    groups,
    // 导航 / 过滤 setter
    setViewMode,
    setFilterScope,
    setFilterStatus,
    setFilterPriority,
    setFilterType,
    setSearchText,
    setGroupMode,
    // 选择 / 编辑
    selectTodo,
    startCreate,
    closeEditor,
    patchForm,
    saveForm,
    // 行内交互
    toggleStatus,
    toggleMyDay,
    deleteTodo,
    reorder,
    dispatchTodo,
    // 子任务
    addSubtask,
    toggleSubtask,
    removeSubtask,
    // 类型
    addCustomType,
    removeCustomType,
    // 统计
    loadStats,
  };
};

export type TodoPageVM = ReturnType<typeof useTodoPage>;
