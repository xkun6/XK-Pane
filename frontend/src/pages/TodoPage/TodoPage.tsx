import { Box, Button, CircularProgress, Typography, useTheme } from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import ChecklistRoundedIcon from '@mui/icons-material/ChecklistRounded';
import { useT } from '@/i18n';
import type { Todo } from '@/services';
import { todoPageStyles } from './TodoPage.styles';
import { useTodoPage } from './useTodoPage';
import { TodoSidebar } from './components/TodoSidebar';
import { TodoFilterBar } from './components/TodoFilterBar';
import { TodoListItem } from './components/TodoListItem';
import { TodoEditor } from './components/TodoEditor';
import { TodoOverview } from './components/TodoOverview';

// TodoPage：三栏 TodoManager（左栏导航 / 中栏过滤+列表 / 右栏编辑器+概览）。
// 忠实复刻 cc-pane web/components/todo/TodoManager，状态由 useTodoPage（自包含 query 模型）承载。
export const TodoPage = () => {
  const theme = useTheme();
  const styles = todoPageStyles(theme);
  const t = useT();
  const vm = useTodoPage();

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
            {vm.todos.map((todo) => (
              <TodoListItem
                key={todo.id}
                todo={todo}
                selected={vm.selectedTodo?.id === todo.id}
                onSelect={vm.selectTodo}
                onToggleStatus={vm.toggleStatus}
                onToggleMyDay={vm.toggleMyDay}
                onDelete={handleDelete}
              />
            ))}
          </Box>
        )}
      </Box>

      {showEditor ? (
        <TodoEditor
          form={vm.editForm}
          isCreating={vm.isCreating}
          editingId={vm.selectedTodo?.id ?? null}
          subtasks={vm.selectedTodo?.subtasks ?? []}
          allTypes={vm.allTypes}
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
  );
};
