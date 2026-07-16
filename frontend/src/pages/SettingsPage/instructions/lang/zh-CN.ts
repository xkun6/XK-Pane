import type { Messages } from '@/i18n';

export const instructionsZhCN: Messages = {
  settings: {
    instructions: {
      title: '全局指令',
      hint: '直接编辑各 CLI 的全局指令文件，保存即写入磁盘（首次写入前自动备份 .xkpane-bak）。',
      tabs: {
        claude: 'Claude Code（CLAUDE.md）',
        codex: 'Codex（AGENTS.md）',
      },
      pathLabel: {
        claude: '~/.claude/CLAUDE.md',
        codex: '~/.codex/AGENTS.md',
      },
      save: '保存',
      reload: '重新加载',
      unsaved: '未保存',
      loadError: '读取文件失败',
      error: {
        title: '操作失败',
      },
      reloadConfirm: {
        title: '放弃未保存的修改？',
        message: '重新加载会丢弃当前未保存的修改，确定继续吗？',
        ok: '重新加载',
        cancel: '取消',
      },
    },
  },
};
