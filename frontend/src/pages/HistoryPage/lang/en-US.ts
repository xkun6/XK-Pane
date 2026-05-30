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
    config: {
      title: 'History config & cleanup',
      enabled: 'Enable local history',
      maxVersionsPerFile: 'Max versions per file',
      maxAgeDays: 'Retention days',
      maxFileSizeMB: 'Max file size (MB)',
      maxTotalSizeMB: 'Max total size (MB)',
      minSaveIntervalSecs: 'Min save interval (s)',
      ignorePatterns: 'Ignore patterns (one glob per line)',
      save: 'Save config',
      saved: {
        title: 'Saved',
        message: 'Local history config updated.',
      },
    },
    stats: {
      title: 'Storage usage',
      summary: '{{versions}} versions · {{files}} files · {{size}}',
      limit: 'limit {{limit}}',
    },
    cleanup: {
      action: 'Clean up now',
      confirm: {
        title: 'Clean up history now',
        message:
          'This deletes expired / over-count / over-total-size old versions per the current config and reclaims orphan blobs. This cannot be undone. Continue?',
        ok: 'Clean up',
        cancel: 'Cancel',
      },
      done: {
        title: 'Cleanup complete',
        message: 'Old versions cleaned per config; the stats below are refreshed.',
      },
    },
    error: {
      title: 'Local history operation failed',
    },
  },
};
