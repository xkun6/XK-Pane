import type { Messages } from '@/i18n';

export const sessionTimelineZhCN: Messages = {
  sessionTimeline: {
    title: '对话',
    empty: '暂无对话内容，发送一条消息开始。',
    thinking: '思考',
    working: '思考中…',
    tool: '工具',
    result: '结果',
    resultLines: '{{n}} 行',
    showTerminal: '切到终端',
    showChat: '切到对话',
    copy: '复制',
    copied: '已复制',
    moreLines: '还有 {{n}} 行',
    todo: {
      title: '待办清单',
      done: '已完成 {{done}}/{{total}}',
      pending: '待办',
      inProgress: '进行中',
      completed: '已完成',
    },
    plan: {
      title: '实施计划',
    },
    diff: {
      truncated: '差异过长，已截断',
    },
    bash: {
      stderr: '错误输出',
      empty: '（无输出）',
    },
    ask: {
      title: '需要你的选择',
      multiSelectHint: '可多选',
      singleSelectHint: '单选',
      submit: '提交',
      submitted: '已提交，等待 Agent…',
      readonly: '请在终端中作答',
    },
    status: {
      context: '上下文',
      usage5h: '用量',
      weekly: '本周',
    },
  },
};
