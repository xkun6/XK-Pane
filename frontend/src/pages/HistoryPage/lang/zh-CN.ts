import type { Messages } from '@/i18n';

// HistoryPage 中文文案，挂在 history.* 命名空间。
export const historyPageZhCN: Messages = {
  history: {
    eyebrow: 'XK-Pane',
    title: '本地历史',
    project: '项目',
    refresh: '刷新',
    noProjects: '尚无已登记项目。请先在侧边栏添加一个项目。',
    snapshotPlaceholder: '项目内相对路径（如 src/main.go），回车快照',
    snapshotAction: '快照',
    recent: '最近变更（{{count}}）',
    empty: '暂无历史记录。在上方为某个文件做一次快照吧。',
    restore: '回滚到此版本',
    snapshot: {
      title: '快照',
      created: '已为该文件保存一个新版本。',
      noChange: '内容未变化，已跳过（与最新版本相同）。',
    },
    restoreConfirm: {
      title: '回滚文件',
      message: '确定把「{{path}}」回滚到该版本吗？将覆盖当前工作树文件（回滚前会自动打标签便于撤销）。',
      ok: '回滚',
      cancel: '取消',
    },
    restored: {
      title: '回滚完成',
      message: '文件已恢复到所选版本。',
    },
    error: {
      title: '本地历史操作失败',
    },
  },
};
