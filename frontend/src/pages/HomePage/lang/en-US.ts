import type { Messages } from '@/i18n';

// HomePage English copy. Mounted under the `home.*` namespace.
export const homePageEnUS: Messages = {
  home: {
    greeting: {
      morning: 'Good morning',
      afternoon: 'Good afternoon',
      evening: 'Good evening',
    },
    tagline: 'Your projects, terminal sessions, and Git changes — all in one place.',
    projects: {
      stat: '{{count}} projects',
    },
    actions: {
      title: 'Quick actions',
      terminal: 'Open Terminal',
      git: 'Source Control',
      sessions: 'Sessions',
      settings: 'Settings',
    },
    recent: {
      title: 'Recent launches',
      empty: 'No launches yet. Open a terminal, pick a project, and start a CLI to begin.',
      shell: 'Shell',
      relaunchHint: 'Relaunch in the terminal: {{path}}',
    },
    env: {
      title: 'Environment',
      detecting: 'Detecting…',
      installed: 'Installed',
      notInstalled: 'Not installed',
    },
  },
};
