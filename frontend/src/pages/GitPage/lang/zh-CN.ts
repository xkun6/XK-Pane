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
  },
};
