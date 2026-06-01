import type { Messages } from '@/i18n';

// WebWorkspaceTree English copy, under the webWorkspaceTree.* namespace (self-contained lang).
export const webWorkspaceTreeEnUS: Messages = {
  webWorkspaceTree: {
    rail: {
      title: 'Workspaces',
      hint: 'Expand a workspace and project to resume a past session or start a new one — the same window stays in sync on the desktop and this device.',
    },
    active: {
      title: 'Running',
      empty: 'No running sessions',
      loadError: 'Failed to load sessions',
      refresh: 'Refresh running sessions',
    },
    loadError: 'Failed to load workspaces',
    empty: 'No workspaces yet (create one and add projects on the desktop first).',
    a11y: {
      moreActions: 'More actions',
    },
    menu: {
      newSession: 'New session',
      copyPath: 'Copy path',
      resume: 'Resume',
      resumeWithReclaude: 'Resume with reclaude',
      copyId: 'Copy ID',
    },
    sessions: {
      loading: 'Loading sessions…',
      loadError: 'Failed to load sessions',
      empty: 'No resumable sessions for this project',
      noPrompt: '(no first prompt)',
      time: {
        justNow: 'just now',
        minutes: '{{n}} min ago',
        hours: '{{n}} h ago',
        days: '{{n}} d ago',
      },
    },
    dialog: {
      title: 'New session',
      target: 'Project',
      cliTool: 'CLI tool',
      prompt: 'Initial task (prompt)',
      promptPlaceholder: 'Describe the task for the agent…',
      launch: 'Launch',
      cancel: 'Cancel',
      launchError: 'Launch failed',
    },
    notice: {
      copied: 'Copied',
      copyFailed: 'Copy failed',
      resumeFailed: 'Failed to resume session',
    },
  },
};
