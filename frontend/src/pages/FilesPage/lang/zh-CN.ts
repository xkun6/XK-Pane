import type { Messages } from '@/i18n';

// FilesPage 中文文案，命名空间 files.*。
export const filesPageZhCN: Messages = {
  files: {
    noFolder: '未打开目录',
    openFolder: '打开文件夹',
    openFolderTitle: '选择要在文件浏览器中打开的目录',
    reveal: '在系统文件管理器中显示',
    desktopOnly: '文件浏览器仅在桌面应用中可用。',
    saveErrorTitle: '保存失败',
    empty: {
      title: '尚未打开任何目录',
      hint: '打开一个文件夹开始浏览与编辑；也可在左侧工作区右键目录选择「在文件浏览器中打开」。',
      openFolder: '打开文件夹',
    },
    recent: {
      title: '最近打开',
    },
    tree: {
      ariaLabel: '文件目录树',
      loading: '加载中…',
      error: '读取目录失败',
      empty: '空目录',
    },
    search: {
      placeholder: '过滤已加载文件…',
      clear: '清除过滤',
      noMatch: '无匹配项（仅搜索已展开/已加载的范围）',
    },
    editor: {
      selectFile: '从左侧选择一个文件以查看 / 编辑',
      save: '保存',
      dirty: '有未保存的更改',
      binary: '二进制文件，无法以文本方式编辑。',
      loadError: '读取文件失败。',
      source: '源码',
      preview: '预览',
    },
    menu: {
      newDir: '新建文件夹…',
      newFile: '新建文件…',
      rename: '重命名…',
      duplicate: '复制副本',
      trash: '移到回收站',
      copyPath: '复制路径',
      reveal: '在系统文件管理器中显示',
      refresh: '刷新',
      trashTitle: '移到回收站',
      trashDirConfirm: '确定把文件夹「{{name}}」移到回收站吗？可从回收站恢复。',
      trashConfirmBtn: '移到回收站',
    },
    dialog: {
      confirm: '确定',
      errorTitle: '操作失败',
      newFile: { title: '新建文件', label: '文件名' },
      newDir: { title: '新建文件夹', label: '文件夹名' },
      rename: { title: '重命名', label: '新名称' },
    },
  },
};
