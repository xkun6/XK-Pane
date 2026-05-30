import type { Messages } from '@/i18n';

// TodoPage 中文文案，挂在 todo.* 命名空间。
export const todoPageZhCN: Messages = {
  todo: {
    eyebrow: 'XK-Pane',
    title: '待办',
    add: '添加',
    addPlaceholder: '添加一个待办，回车确认',
    addSubtaskPlaceholder: '添加子任务',
    empty: '暂无待办。在上方输入框添加一个吧。',
    priority: {
      high: '高',
      medium: '中',
      low: '低',
    },
    actions: {
      markDone: '标记完成',
      markUndone: '标记未完成',
      remove: '删除',
      addSubtask: '添加子任务',
    },
    removeConfirm: {
      title: '删除待办',
      message: '确定删除待办「{{title}}」吗？该操作不可撤销。',
      ok: '删除',
      cancel: '取消',
    },
    error: {
      title: '待办操作失败',
    },
  },
};
