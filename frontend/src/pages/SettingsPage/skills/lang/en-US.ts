import type { Messages } from '@/i18n';

export const skillsEnUS: Messages = {
  settings: {
    skills: {
      title: 'Skills management',
      hint: 'Skills are owned by XK-Pane as the single source of truth (SSOT) and projected per-CLI via the switches below — Claude Code (~/.claude/skills) and Codex (~/.codex/skills). Turning a switch off removes the skill from that CLI while XK-Pane keeps the content.',
      importFolder: 'Import from folder',
      importFolderDialogTitle: 'Choose a skill directory (must contain SKILL.md)',
      importApps: 'Import from existing CLIs',
      empty: 'No skills yet. Click "Import from existing CLIs" to get started, or "Import from folder" to add one.',
      toggles: {
        claude: 'Claude',
        codex: 'Codex',
      },
      remove: 'Remove',
      removeConfirm: {
        title: 'Remove skill',
        message: 'Remove skill "{{name}}"? It will be removed from any CLI where it is enabled, and its SSOT content will be moved to the trash directory rather than deleted outright.',
        ok: 'Remove',
        cancel: 'Cancel',
      },
      importDone: {
        title: 'Import complete',
        message: 'Added {{count}} new skill(s).',
      },
      error: {
        title: 'Skill operation failed',
      },
    },
  },
};
