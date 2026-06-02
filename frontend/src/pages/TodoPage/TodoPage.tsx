import { useState } from 'react';
import { Box, Button, CircularProgress, Typography, useTheme } from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import ChecklistRoundedIcon from '@mui/icons-material/ChecklistRounded';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import { SortableContext, verticalListSortingStrategy, arrayMove } from '@dnd-kit/sortable';
import { useT } from '@/i18n';
import { useRegisteredProjects } from '@/project';
import { useWorkspaces } from '@/workspace';
import type { Project, Todo } from '@/services';
import { todoPageStyles } from './TodoPage.styles';
import { useTodoPage } from './useTodoPage';
import { TodoSidebar } from './components/TodoSidebar';
import { TodoFilterBar } from './components/TodoFilterBar';
import { SortableTodoListItem } from './components/SortableTodoListItem';
import { TodoTagGroup } from './components/TodoTagGroup';
import { TodoEditor } from './components/TodoEditor';
import { TodoOverview } from './components/TodoOverview';
import { TodoDispatchDialog } from './components/TodoDispatchDialog';

// TodoPage：三栏 TodoManager（左栏导航 / 中栏过滤+列表 / 右栏编辑器+概览）。
// 忠实复刻 cc-pane web/components/todo/TodoManager，状态由 useTodoPage（自包含 query 模型）承载。
export const TodoPage = () => {
  const theme = useTheme();
  const styles = todoPageStyles(theme);
  const t = useT();
  const vm = useTodoPage();
  const projects = useRegisteredProjects();
  const { workspaces } = useWorkspaces();
  const [dispatchTarget, setDispatchTarget] = useState<Todo | null>(null);

  const currentViewLabel =
    vm.viewMode === 'my_day'
      ? t('todo.header.myDay')
      : vm.filterScope
        ? t(`todo.scope.${vm.filterScope}`)
        : t('todo.header.allTasks');

  const handleDelete = (todo: Todo): void => {
    void vm.deleteTodo(todo, {
      title: t('todo.delete.title'),
      message: t('todo.delete.message'),
      okLabel: t('todo.delete.ok'),
      cancelLabel: t('todo.delete.cancel'),
    });
  };

  // 拖拽排序（仅平铺模式）：松手后按新顺序重写 sort_order。对齐 cc-pane handleDragEnd。
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor),
  );
  const handleDragEnd = (event: DragEndEvent): void => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const oldIndex = vm.todos.findIndex((td) => td.id === active.id);
    const newIndex = vm.todos.findIndex((td) => td.id === over.id);
    if (oldIndex === -1 || newIndex === -1) return;
    const reordered = arrayMove(vm.todos, oldIndex, newIndex);
    void vm.reorder(reordered.map((td) => td.id));
  };

  // 分组标签（View 持 t()）：状态/优先级/作用域走 i18n，标签原文，未标记走 todo.untagged。
  const groupLabel = (key: string): string => {
    if (vm.groupMode === 'tag') return key === '__untagged__' ? t('todo.untagged') : key;
    if (vm.groupMode === 'status') return t(`todo.status.${key}`);
    if (vm.groupMode === 'priority') return t(`todo.priority.${key}`);
    if (vm.groupMode === 'scope') return t(`todo.scope.${key}`);
    return key;
  };

  const handleDispatchConfirm = (project: Project, cliTool: string): void => {
    if (dispatchTarget) void vm.dispatchTodo(dispatchTarget, project, cliTool);
    setDispatchTarget(null);
  };

  const showEditor = vm.isCreating || vm.selectedTodo !== null;

  return (
    <Box sx={styles.root}>
      <TodoSidebar
        viewMode={vm.viewMode}
        activeScope={vm.filterScope}
        onViewModeChange={vm.setViewMode}
        onScopeChange={vm.setFilterScope}
      />

      <Box sx={styles.main}>
        <Box sx={styles.mainHeader}>
          <Box>
            <Typography sx={styles.mainTitle}>{currentViewLabel}</Typography>
            <Typography sx={styles.mainSubtitle}>
              {t('todo.header.count', { count: String(vm.total) })}
            </Typography>
          </Box>
          <Button
            variant="contained"
            size="small"
            startIcon={<AddRoundedIcon fontSize="small" />}
            onClick={vm.startCreate}
            sx={styles.newBtn}
          >
            {t('todo.header.newTask')}
          </Button>
        </Box>

        <TodoFilterBar
          searchText={vm.searchText}
          onSearchChange={vm.setSearchText}
          filterStatus={vm.filterStatus}
          onStatusChange={vm.setFilterStatus}
          filterPriority={vm.filterPriority}
          onPriorityChange={vm.setFilterPriority}
          filterType={vm.filterType}
          onTypeChange={vm.setFilterType}
          allTypes={vm.allTypes}
          groupMode={vm.groupMode}
          onGroupModeChange={vm.setGroupMode}
        />

        {vm.loading ? (
          <Box sx={styles.loadingBox}>
            <CircularProgress size={18} />
            <span>{t('todo.loading')}</span>
          </Box>
        ) : vm.todos.length === 0 ? (
          <Box sx={styles.empty}>
            <Box sx={styles.emptyIcon}>
              <ChecklistRoundedIcon fontSize="large" />
            </Box>
            <Typography>{t('todo.empty.title')}</Typography>
            <Button variant="text" onClick={vm.startCreate} sx={styles.emptyLink}>
              {t('todo.empty.create')}
            </Button>
          </Box>
        ) : (
          <Box sx={styles.listScroll}>
            {vm.groups ? (
              [...vm.groups.entries()].map(([key, groupTodos]) => (
                <TodoTagGroup
                  key={key}
                  label={groupLabel(key)}
                  todos={groupTodos}
                  selectedId={vm.selectedTodo?.id ?? null}
                  onSelect={vm.selectTodo}
                  onToggleStatus={vm.toggleStatus}
                  onToggleMyDay={vm.toggleMyDay}
                  onDelete={handleDelete}
                  onDispatch={setDispatchTarget}
                />
              ))
            ) : (
              <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
                <SortableContext items={vm.todos.map((td) => td.id)} strategy={verticalListSortingStrategy}>
                  {vm.todos.map((todo) => (
                    <SortableTodoListItem
                      key={todo.id}
                      todo={todo}
                      selected={vm.selectedTodo?.id === todo.id}
                      onSelect={vm.selectTodo}
                      onToggleStatus={vm.toggleStatus}
                      onToggleMyDay={vm.toggleMyDay}
                      onDelete={handleDelete}
                      onDispatch={setDispatchTarget}
                    />
                  ))}
                </SortableContext>
              </DndContext>
            )}
          </Box>
        )}
      </Box>

      {/* 右栏：showEditor 时 480px 展开编辑器，否则收起为 0 宽（Overview 隐藏，对齐 cc-pane aside）。*/}
      <Box sx={showEditor ? styles.rightPaneWrap : styles.rightPaneWrapCollapsed}>
        {showEditor ? (
          <TodoEditor
            form={vm.editForm}
            isCreating={vm.isCreating}
            editingId={vm.selectedTodo?.id ?? null}
            subtasks={vm.selectedTodo?.subtasks ?? []}
            allTypes={vm.allTypes}
            projects={projects}
            workspaces={workspaces}
            onPatch={vm.patchForm}
            onSave={vm.saveForm}
            onClose={vm.closeEditor}
            onDelete={() => vm.selectedTodo && handleDelete(vm.selectedTodo)}
            onAddCustomType={vm.addCustomType}
            onRemoveCustomType={vm.removeCustomType}
            onAddSubtask={(title) => vm.selectedTodo && vm.addSubtask(vm.selectedTodo.id, title)}
            onToggleSubtask={(sid, completed) => vm.selectedTodo && vm.toggleSubtask(vm.selectedTodo.id, sid, completed)}
            onRemoveSubtask={(sid) => vm.selectedTodo && vm.removeSubtask(vm.selectedTodo.id, sid)}
          />
        ) : (
          <TodoOverview
            stats={vm.stats}
            recent={vm.todos.slice(0, 5)}
            onSelectTodo={vm.selectTodo}
            onCreateNew={vm.startCreate}
            onLoadStats={vm.loadStats}
          />
        )}
      </Box>

      <TodoDispatchDialog
        open={dispatchTarget !== null}
        todo={dispatchTarget}
        projects={projects}
        onClose={() => setDispatchTarget(null)}
        onConfirm={handleDispatchConfirm}
      />
    </Box>
  );
};
