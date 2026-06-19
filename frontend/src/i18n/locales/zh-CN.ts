import type { Locale } from '../types';

// 简体中文（默认）—— 公用文案。
// 仅放"跨页面 / 框架级"的 keys：
//   - app.*：应用元数据
//   - sidebar.* / titleBar.*：基座组件
//   - route.*：路由 label（多处消费：Sidebar tooltip / 设置页 / 未来面包屑）
//   - common.*：通用术语（保存 / 取消 / 加载中）
// 页面级文案（home / settings 等）不要放这里 —— 各 page 自带 lang/ 目录注册。
//
// 命名约定：点路径分组；模板插值用 {{var}}（与 t(key, vars) 配合）。
export const zhCN: Locale = {
  code: 'zh-CN',
  englishName: 'Chinese (Simplified)',
  nativeName: '简体中文',
  messages: {
    app: {
      title: 'XK-Pane',
    },
    sidebar: {
      brand: 'X',
      navAriaLabel: '主导航',
      usage: {
        ariaLabel: 'Claude Code token 用量',
        title: 'Claude Code token 用量',
        source: '本地会话日志统计',
        last5h: '近5h',
        today: '今日',
        week: '本周',
        month: '本月',
        input: '输入',
        output: '输出',
        cacheRead: '缓存读',
        cacheCreate: '缓存写',
        msgs: '消息',
      },
    },
    titleBar: {
      controls: {
        minimise: '最小化',
        maximise: '最大化 / 还原',
        close: '关闭',
      },
    },
    route: {
      home: '首页',
      terminal: '终端',
      files: '文件',
      git: 'Git',
      history: '本地历史',
      ssh: 'SSH',
      mcpe: 'MC PE',
      todo: '待办',
      settings: '设置',
    },
    rightPanel: {
      close: '收起面板',
    },
    terminal: {
      tab: {
        close: '关闭标签',
        rename: '重命名',
        renameTitle: '重命名标签',
        renamePlaceholder: '标签名称',
        pin: '置顶',
        unpin: '取消置顶',
      },
      view: {
        group: '视图切换',
        card: '卡片',
        terminal: '终端',
      },
    },
    common: {
      save: '保存',
      cancel: '取消',
      loading: '加载中…',
    },
  },
};
