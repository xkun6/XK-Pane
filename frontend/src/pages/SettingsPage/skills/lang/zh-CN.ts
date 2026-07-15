import type { Messages } from '@/i18n';

export const skillsZhCN: Messages = {
  settings: {
    skills: {
      title: 'Skills 管理',
      hint: 'Skill 以 XK-Pane 为唯一事实源（SSOT），按下方开关分别投影到 Claude Code（~/.claude/skills）与 Codex（~/.codex/skills）。关闭开关 = 从对应 CLI 移除该 Skill，但 XK-Pane 仍保留内容。',
      importFolder: '从文件夹导入',
      importFolderDialogTitle: '选择 Skill 目录（需含 SKILL.md）',
      importApps: '从现有 CLI 导入',
      empty: '暂无 Skill，可点击「从现有 CLI 导入」快速开始，或「从文件夹导入」新增。',
      toggles: {
        claude: 'Claude',
        codex: 'Codex',
      },
      remove: '删除',
      removeConfirm: {
        title: '删除 Skill',
        message: '确定删除 Skill「{{name}}」吗？将从已启用的 CLI 中移除，SSOT 内容会挪入回收目录而非直接销毁。',
        ok: '删除',
        cancel: '取消',
      },
      importDone: {
        title: '导入完成',
        message: '新增了 {{count}} 个 Skill。',
      },
      error: {
        title: 'Skill 操作失败',
      },
    },
  },
};
