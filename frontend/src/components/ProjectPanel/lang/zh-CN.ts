import type { Messages } from '@/i18n';

// ProjectPanel 中文文案，挂 projectPanel.* 命名空间（组件自带 lang，自包含）。
export const projectPanelZhCN: Messages = {
  projectPanel: {
    title: '项目',
    add: '添加项目',
    addDialogTitle: '选择项目目录',
    empty: '暂无项目。\n点上方 + 添加一个目录。',
    remove: '移除项目',
    removeConfirm: {
      title: '移除项目',
      message: '确定移除项目「{{name}}」吗？这只从列表移除，不会删除磁盘上的文件。',
    },
    launch: {
      openTerminal: '打开终端',
      loading: '检测 CLI 工具中…',
      notInstalled: '未安装',
    },
    error: {
      title: '项目操作失败',
    },
  },
};
