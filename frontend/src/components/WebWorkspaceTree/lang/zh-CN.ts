import type { Messages } from '@/i18n';

// WebWorkspaceTree 中文文案，挂 webWorkspaceTree.* 命名空间（组件自带 lang，自包含）。
export const webWorkspaceTreeZhCN: Messages = {
  webWorkspaceTree: {
    rail: {
      title: '工作区',
      hint: '展开工作区与项目，恢复历史会话或新建会话；桌面端与本机同步显示同一窗口。',
    },
    active: {
      title: '运行中',
      empty: '暂无运行中会话',
      loadError: '读取会话失败',
      refresh: '刷新运行中会话',
    },
    loadError: '读取工作区失败',
    empty: '暂无工作区（请先在桌面端创建工作区并添加项目）。',
    a11y: {
      moreActions: '更多操作',
    },
    menu: {
      newSession: '新建会话',
      copyPath: '复制路径',
      resume: '恢复会话',
      resumeWithReclaude: '以 reclaude 恢复',
      copyId: '复制 ID',
    },
    sessions: {
      loading: '加载会话中…',
      loadError: '读取会话失败',
      empty: '该项目暂无可恢复的历史会话',
      noPrompt: '（无首条 prompt）',
      time: {
        justNow: '刚刚',
        minutes: '{{n}} 分钟前',
        hours: '{{n}} 小时前',
        days: '{{n}} 天前',
      },
    },
    dialog: {
      title: '新建会话',
      target: '项目',
      cliTool: 'CLI 工具',
      prompt: '初始任务（prompt）',
      promptPlaceholder: '描述要让 Agent 执行的任务…',
      launch: '启动',
      cancel: '取消',
      launchError: '启动失败',
    },
    notice: {
      copied: '已复制',
      copyFailed: '复制失败',
      resumeFailed: '恢复会话失败',
    },
  },
};
