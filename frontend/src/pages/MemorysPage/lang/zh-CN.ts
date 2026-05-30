import type { Messages } from '@/i18n';

// MemorysPage 中文文案，挂在 memory.* 命名空间。
export const memorysPageZhCN: Messages = {
  memory: {
    eyebrow: 'XK-Pane',
    title: '记忆',
    refresh: '刷新',
    searchPlaceholder: '搜索标题 / 内容 / 标签…',
    list: '记忆（{{count}}）',
    empty: '暂无记忆。在上方新建一条吧。',
    editTitle: '编辑记忆',
    save: '保存',
    stats: {
      total: '共 {{count}}',
    },
    filter: {
      scope: '作用域',
      category: '类别',
      all: '全部',
    },
    scope: {
      global: '全局',
      workspace: '工作区',
      project: '项目',
      session: '会话',
    },
    category: {
      decision: '决策',
      lesson: '教训',
      preference: '偏好',
      pattern: '模式',
      fact: '事实',
      plan: '计划',
    },
    field: {
      titlePlaceholder: '标题',
      contentPlaceholder: '内容…',
      titleLabel: '标题',
      contentLabel: '内容',
      scope: '作用域',
      scopeRef: '作用域引用',
      workspaceName: '工作区名',
      projectPath: '项目路径',
      sessionId: '会话 ID',
      category: '类别',
      importance: '重要度',
      tags: '标签',
      tagsPlaceholder: '逗号分隔，如 auth, style',
    },
    create: {
      title: '新建记忆',
      action: '新建',
      scopeRefRequired: '该作用域需要填写对应的引用（工作区名 / 项目路径 / 会话 ID）。',
    },
    saved: {
      title: '已保存',
      message: '记忆已更新。',
    },
    removeConfirm: {
      title: '删除记忆',
      message: '确定删除「{{title}}」吗？该操作不可撤销。',
      ok: '删除',
      cancel: '取消',
    },
    actions: {
      remove: '删除',
    },
    error: {
      title: '记忆操作失败',
    },
  },
};
