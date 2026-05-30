import type { Messages } from '@/i18n';

// HomePage 中文文案。挂在 home.* 命名空间下。
// 由 src/pages/HomePage/lang/index.ts 通过 localeRegistry.extend('zh-CN', ...) 注入。
export const homePageZhCN: Messages = {
  home: {
    greeting: {
      morning: '上午好',
      afternoon: '下午好',
      evening: '晚上好',
    },
    tagline: '管理你的项目、终端会话与 Git 改动，一切尽在一处。',
    projects: {
      stat: '{{count}} 个项目',
    },
    actions: {
      title: '快捷操作',
      terminal: '打开终端',
      git: '源代码管理',
      history: '本地历史',
      settings: '设置',
    },
    recent: {
      title: '最近启动',
      empty: '还没有启动记录。打开终端、选择项目并启动一个 CLI 即可开始。',
      shell: '终端',
      relaunchHint: '在终端中重新启动：{{path}}',
    },
    env: {
      title: '环境检测',
      detecting: '检测中…',
      installed: '已安装',
      notInstalled: '未安装',
    },
  },
};
