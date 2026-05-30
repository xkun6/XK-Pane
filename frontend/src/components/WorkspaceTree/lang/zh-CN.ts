import type { Messages } from '@/i18n';

// WorkspaceTree 中文文案，挂 workspaceTree.* 命名空间（组件自带 lang，自包含）。
export const workspaceTreeZhCN: Messages = {
  workspaceTree: {
    title: '工作空间',
    empty: '还没有工作空间',
    emptyHint: '新建一个来组织你的项目',
    newWorkspace: '新建工作空间',
    noProjects: '暂无项目',
    projectCountAria: '{{count}} 个项目',
    pin: '置顶',
    unpin: '取消置顶',
    delete: '删除',
    expand: '展开',
    collapse: '收起',
    deleteConfirm: {
      title: '删除工作空间',
      message: '确定删除工作空间「{{name}}」吗？其下项目归属会一并移除（不会删除磁盘上的文件）。',
    },
    dialog: {
      createTitle: '新建工作空间',
      nameLabel: '工作空间名称',
      create: '创建',
      save: '保存',
    },
    renameDialog: {
      title: '重命名工作空间',
      label: '新名称',
    },
    aliasDialog: {
      title: '设置别名',
      label: '别名（留空则清除）',
    },
    menu: {
      openTerminal: '打开终端',
      launchLoading: '检测 CLI 工具中…',
      notInstalled: '未安装',
      importProject: '导入项目',
      importDialogTitle: '选择项目目录',
      rename: '重命名',
      setAlias: '设置别名',
      copyPath: '复制路径',
      removeProject: '移除项目',
      removeProjectConfirm: {
        title: '移除项目',
        message: '确定从工作空间移除项目「{{name}}」吗？这只从列表移除，不会删除磁盘上的文件。',
      },
    },
    error: {
      title: '工作空间操作失败',
    },
  },
};
