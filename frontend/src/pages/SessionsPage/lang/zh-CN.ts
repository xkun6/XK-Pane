import type { Messages } from '@/i18n';

// SessionsPage 中文文案，挂在 sessions.* 命名空间。
export const sessionsPageZhCN: Messages = {
  sessions: {
    eyebrow: 'XK-Pane',
    title: '会话',
    active: '活跃会话（{{count}}）',
    untitled: '终端',
    recent: '最近启动',
    noResumable: '暂无可恢复的会话。',
    ungrouped: '未分组',
    detail: '详情',
    sessionId: '会话 ID',
    launchTime: '启动时间',
    lastPrompt: '上次输入',
    copyId: '复制会话 ID',
    resume: '恢复',
    deleteRecord: '删除记录',
    clear: {
      action: '清空',
      confirmTitle: '清空启动历史',
      confirmMessage: '确定要清空所有启动历史吗？此操作不可撤销。',
      ok: '清空',
      cancel: '取消',
    },
    error: {
      title: '操作失败',
    },
    time: {
      justNow: '刚刚',
      minutes: '{{n}} 分钟前',
      hours: '{{n}} 小时前',
      days: '{{n}} 天前',
    },
  },
};
