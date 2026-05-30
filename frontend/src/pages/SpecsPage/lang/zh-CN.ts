import type { Messages } from '@/i18n';

// SpecsPage 中文文案，挂在 specs.* 命名空间。
export const specsPageZhCN: Messages = {
  specs: {
    eyebrow: 'XK-Pane',
    title: '需求规格',
    project: '项目',
    refresh: '刷新',
    noProjects: '尚无已登记项目。请先在侧边栏添加一个项目。',
    createPlaceholder: '新规格标题，回车创建',
    create: '新建',
    list: '规格（{{count}}）',
    empty: '该项目暂无规格。在上方新建一个吧。',
    save: '保存',
    contentLoading: '正在读取内容…',
    status: {
      draft: '草稿',
      active: '激活',
      archived: '已归档',
    },
    actions: {
      activate: '设为激活',
      archive: '归档',
      remove: '删除',
    },
    saved: {
      title: '已保存',
      message: '规格内容已写入文件。',
    },
    archiveConfirm: {
      title: '归档规格',
      message: '确定归档「{{title}}」吗？文件将移动到 archived/ 子目录。',
      ok: '归档',
      cancel: '取消',
    },
    removeConfirm: {
      title: '删除规格',
      message: '确定删除「{{title}}」吗？将一并删除其 .md 文件，且不可撤销。',
      ok: '删除',
      cancel: '取消',
    },
    error: {
      title: '需求规格操作失败',
    },
  },
};
