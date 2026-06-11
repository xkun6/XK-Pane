import type { Messages } from '@/i18n';

// HistoryPage English strings under the history.* namespace. Keys must mirror zh-CN exactly.
export const historyPageEnUS: Messages = {
  history: {
    eyebrow: 'XK-Pane',
    title: 'Local History',
    project: 'Project',
    refresh: 'Refresh',
    noProjects: 'No projects registered yet. Add a project from the sidebar first.',
    panel: { noDir: 'Open a directory in the Files page to see its local history here.' },
    error: { title: 'Local history operation failed' },

    // Stage 1: recent changed files
    fileList: {
      loading: 'Loading file list…',
      empty: 'No history yet for this project. Once auto-capture is on or you snapshot a file, changes show up here.',
      autoTrack: 'Versions are recorded automatically when files are saved',
    },

    // Stage 2: version timeline + preview
    backToList: 'Back to file list',
    timeline: {
      selectVersion: 'Select a version on the left to view.',
      empty: 'No versions recorded for this file.',
      rightClickHint: 'Right-click a version to tag it',
    },
    tabs: {
      diff: 'Diff',
      content: 'Content',
      deleted: 'Deleted',
      projectRestore: 'Snapshots',
    },
    branchFilter: {
      all: 'All branches',
      unknown: 'Unknown branch',
      label: 'Branch',
    },

    // diff tab
    diff: {
      loading: 'Computing diff…',
      binary: 'Binary file — diff not shown.',
      truncated: 'File too large — diff truncated.',
      identical: 'No differences from the compare target.',
      added: 'Added',
      removed: 'Removed',
      vsWorking: 'this version ↔ working tree',
      betweenVersions: 'previous version ↔ this version',
    },

    // content tab
    content: {
      loading: 'Loading content…',
      empty: '(empty file)',
    },

    // deleted tab
    deleted: {
      empty: 'No deleted files.',
      restore: 'Restore this file',
      restoreConfirm: {
        title: 'Restore deleted file',
        message: 'Restore "{{path}}" to its last version before deletion? It will be written back to the working tree.',
        ok: 'Restore',
        cancel: 'Cancel',
      },
      restored: { title: 'Restored', message: 'The file has been restored to the working tree.' },
    },

    // project-restore tab
    projectRestore: {
      empty: 'No project snapshots / labels yet.',
      restoreTo: 'Restore to this snapshot',
      fileCount: '{{count}} files',
      confirm: {
        title: 'Restore to snapshot',
        message:
          'Restore the project to "{{name}}" ({{count}} files)? It overwrites the current working-tree content of those files (an auto label is taken first so you can undo).',
        ok: 'Restore',
        cancel: 'Cancel',
      },
      restored: { title: 'Restore complete', message: 'Restored {{count}} files to the working tree.' },
      source: {
        git_commit: 'Git commit',
        claude_session: 'Claude session',
        user: 'Manual',
        build: 'Build',
        restore: 'Restore point',
        branch_switch: 'Branch switch',
        auto: 'Auto',
        manual: 'Manual',
      },
    },

    // tagging
    tag: {
      add: 'Add tag',
      title: 'Tag this version',
      label: 'Tag name',
      ok: 'Save tag',
      added: { title: 'Tagged', message: 'The tag has been saved.' },
      failed: 'Failed to add tag',
    },

    // ops: manual snapshot
    snapshotPlaceholder: 'Relative path in project (e.g. src/main.go), Enter to snapshot',
    snapshotAction: 'Snapshot',
    snapshot: {
      title: 'Snapshot',
      created: 'Saved a new version of the file.',
      noChange: 'Content unchanged, skipped (same as latest version).',
    },

    // single-file restore
    restore: 'Restore to this version',
    restoreConfirm: {
      title: 'Restore file',
      message: 'Restore "{{path}}" to this version? It overwrites the working-tree file (an auto label is taken first so you can undo).',
      ok: 'Restore',
      cancel: 'Cancel',
    },
    restored: { title: 'Restore complete', message: 'The file has been restored to the selected version.' },

    // config / stats / cleanup
    config: {
      title: 'History config & cleanup',
      enabled: 'Enable local history (auto-capture)',
      maxVersionsPerFile: 'Max versions per file',
      maxAgeDays: 'Retention days',
      maxFileSizeMB: 'Max file size (MB)',
      maxTotalSizeMB: 'Max total size (MB)',
      minSaveIntervalSecs: 'Min save interval (s)',
      ignorePatterns: 'Ignore patterns (one glob per line)',
      save: 'Save config',
      saved: { title: 'Saved', message: 'Local history config updated.' },
    },
    stats: {
      title: 'Storage usage',
      summary: '{{versions}} versions · {{files}} files · {{size}}',
    },
    cleanup: {
      action: 'Clean up now',
      confirm: {
        title: 'Clean up history now',
        message: 'This deletes expired / over-count / over-total-size old versions per the current config and reclaims orphan blobs. This cannot be undone. Continue?',
        ok: 'Clean up',
        cancel: 'Cancel',
      },
      done: { title: 'Cleanup complete', message: 'Old versions cleaned per config; the stats below are refreshed.' },
    },
  },
};
