import type { Messages } from '@/i18n';

// SessionsPage English copy, under the sessions.* namespace.
export const sessionsPageEnUS: Messages = {
  sessions: {
    eyebrow: 'XK-Pane',
    title: 'Sessions',
    active: 'Active ({{count}})',
    untitled: 'Terminal',
    recent: 'Recent Launches',
    noResumable: 'No resumable sessions yet.',
    ungrouped: 'Ungrouped',
    detail: 'Details',
    sessionId: 'Session ID',
    launchTime: 'Launched',
    lastPrompt: 'Last prompt',
    copyId: 'Copy session ID',
    resume: 'Resume',
    deleteRecord: 'Delete record',
    clear: {
      action: 'Clear',
      confirmTitle: 'Clear launch history',
      confirmMessage: 'Clear all launch history? This cannot be undone.',
      ok: 'Clear',
      cancel: 'Cancel',
    },
    error: {
      title: 'Operation failed',
    },
    time: {
      justNow: 'just now',
      minutes: '{{n}}m ago',
      hours: '{{n}}h ago',
      days: '{{n}}d ago',
    },
  },
};
