import type { Messages } from '@/i18n';

// GitPage 中文文案，挂在 git.* 命名空间。
export const gitPageZhCN: Messages = {
  git: {
    eyebrow: 'XK-Pane',
    title: 'Git',
    project: '项目',
    refresh: '刷新',
    loading: '加载中…',
    noProjects: '尚无已登记项目。请先在侧边栏添加一个项目。',
    notARepo: '该目录不是 Git 仓库。',
    detached: '(游离 HEAD)',
    dirty: '有改动',
    clean: '干净',
    branches: '本地分支（{{count}}）',
    commits: '最近提交',
    noCommits: '暂无提交。',
    error: {
      title: 'Git 操作失败',
    },
    actions: {
      fetch: '拉取',
      pull: '拉取合并',
      push: '推送',
      stash: '暂存',
      stashPop: '弹出暂存',
    },
    ops: {
      fetch: { title: 'Fetch 完成', done: '已拉取所有远端引用。' },
      pull: { title: 'Pull 完成', done: '已合并远端改动。' },
      push: { title: 'Push 完成', done: '已推送当前分支。' },
      stash: { title: 'Stash 完成', done: '已暂存工作区改动。' },
      stashPop: { title: 'Stash Pop 完成', done: '已弹出最近一次暂存。' },
    },
    pushConfirm: {
      title: '推送到远端',
      message: '确定要把当前分支推送到远端吗？此操作会对外发布提交。',
      ok: '推送',
      cancel: '取消',
    },
    worktree: {
      title: '工作树（{{count}}）',
      main: '主',
      add: '新建工作树',
      remove: '删除工作树',
      namePlaceholder: '工作树名称（同时作分支名，不含 / 或 ..）',
      pickerTitle: '选择新工作树的父目录',
      addedTitle: '工作树已创建',
      addedMessage: '新工作树已创建并检出新分支。',
      removedTitle: '工作树已删除',
      removedMessage: '工作树已移除。',
      removeConfirm: {
        title: '删除工作树',
        message: '确定删除工作树「{{path}}」吗？将强制移除（即使有未提交改动）。',
        ok: '删除',
        cancel: '取消',
      },
    },
  },
};
