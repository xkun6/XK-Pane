import type { Messages } from '@/i18n';

export const instructionsEnUS: Messages = {
  settings: {
    instructions: {
      title: 'Global instructions',
      hint: 'Edit each CLI’s global instruction file directly — saving writes it to disk immediately (a .xkpane-bak backup is created before the first write).',
      tabs: {
        claude: 'Claude Code (CLAUDE.md)',
        codex: 'Codex (AGENTS.md)',
      },
      pathLabel: {
        claude: '~/.claude/CLAUDE.md',
        codex: '~/.codex/AGENTS.md',
      },
      save: 'Save',
      reload: 'Reload',
      unsaved: 'Unsaved',
      loadError: 'Failed to read the file',
      error: {
        title: 'Operation failed',
      },
      reloadConfirm: {
        title: 'Discard unsaved changes?',
        message: 'Reloading will discard your unsaved changes. Continue?',
        ok: 'Reload',
        cancel: 'Cancel',
      },
    },
  },
};
