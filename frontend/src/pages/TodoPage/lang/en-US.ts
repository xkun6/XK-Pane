import type { Messages } from '@/i18n';

// TodoPage English strings, under the todo.* namespace.
export const todoPageEnUS: Messages = {
  todo: {
    eyebrow: 'XK-Pane',
    title: 'Todos',
    add: 'Add',
    addPlaceholder: 'Add a todo, press Enter',
    addSubtaskPlaceholder: 'Add a subtask',
    empty: 'No todos yet. Add one in the box above.',
    priority: {
      high: 'High',
      medium: 'Medium',
      low: 'Low',
    },
    actions: {
      markDone: 'Mark done',
      markUndone: 'Mark not done',
      remove: 'Delete',
      addSubtask: 'Add subtask',
    },
    removeConfirm: {
      title: 'Delete todo',
      message: 'Delete todo "{{title}}"? This cannot be undone.',
      ok: 'Delete',
      cancel: 'Cancel',
    },
    error: {
      title: 'Todo operation failed',
    },
  },
};
