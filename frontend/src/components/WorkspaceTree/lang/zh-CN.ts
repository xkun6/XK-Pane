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
      launchInWorktree: '在隔离 worktree 中启动',
      worktreeRootConfirm: {
        title: '确认目标仓库',
        message: '该项目位于更上层 git 仓库内，worktree 将创建在以下仓库根上（如有需要还会在该仓库提交一次 .gitignore）：\n\n{{root}}\n\n继续吗？',
        ok: '继续创建',
      },
      importProject: '导入项目',
      importDialogTitle: '选择项目目录',
      addSSHProject: '添加 SSH 项目',
      rename: '重命名',
      setAlias: '设置别名',
      copyPath: '复制路径',
      copyName: '复制项目名称',
      openInExplorer: '在系统文件管理器中打开',
      openInFileBrowser: '在文件浏览器中打开',
      removeProject: '移除项目',
      removeProjectConfirm: {
        title: '移除项目',
        message: '确定从工作空间移除项目「{{name}}」吗？这只从列表移除，不会删除磁盘上的文件。',
      },
    },
    sshDialog: {
      title: '添加 SSH 项目',
      machineLabel: '远程机器',
      pathLabel: '远端路径',
      pathPlaceholder: '留空则用机器默认路径',
      add: '添加',
      noMachines: '还没有 SSH 机器',
      noMachinesHint: '先到左侧「SSH」页添加一台，再回来登记远程项目。',
    },
    error: {
      title: '工作空间操作失败',
    },
    sessions: {
      toggle: '会话历史',
      loading: '扫描会话中…',
      empty: '暂无会话历史',
      noPrompt: '（无摘要）',
      resume: '恢复会话',
      resumeWithReclaude: '以 reclaude 恢复',
      copyId: '复制会话 ID',
      time: {
        justNow: '刚刚',
        minutes: '{{n}} 分钟前',
        hours: '{{n}} 小时前',
        days: '{{n}} 天前',
      },
    },
  },
};
