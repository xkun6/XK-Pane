import type { Messages } from '@/i18n';

// HistoryPage English copy, under the history.* namespace.
export const historyPageEnUS: Messages = {
  history: {
    eyebrow: 'XK-Pane',
    title: 'Local History',
    project: 'Project',
    refresh: 'Refresh',
    noProjects: 'No projects registered yet. Add a project from the sidebar first.',
    snapshotPlaceholder: 'Relative path in project (e.g. src/main.go), Enter to snapshot',
    snapshotAction: 'Snapshot',
    recent: 'Recent changes ({{count}})',
    empty: 'No history yet. Take a snapshot of a file above.',
    restore: 'Restore to this version',
    snapshot: {
      title: 'Snapshot',
      created: 'Saved a new version of the file.',
      noChange: 'Content unchanged, skipped (same as latest version).',
    },
    restoreConfirm: {
      title: 'Restore file',
      message:
        'Restore "{{path}}" to this version? It overwrites the working-tree file (an auto label is taken first so you can undo).',
      ok: 'Restore',
      cancel: 'Cancel',
    },
    restored: {
      title: 'Restore complete',
      message: 'The file has been restored to the selected version.',
    },
    diff: {
      title: 'Diff: {{path}} (this version ↔ working tree)',
      loading: 'Computing diff…',
      binary: 'Binary file — diff not shown.',
      truncated: 'File too large — diff truncated.',
      identical: 'No differences from the current working-tree file.',
      added: 'Added',
      removed: 'Removed',
    },
    error: {
      title: 'Local history operation failed',
    },
  },
};
