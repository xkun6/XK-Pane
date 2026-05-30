import type { Messages } from '@/i18n';

// GitPage English copy, under the git.* namespace.
export const gitPageEnUS: Messages = {
  git: {
    eyebrow: 'XK-Pane',
    title: 'Git',
    project: 'Project',
    refresh: 'Refresh',
    loading: 'Loading…',
    noProjects: 'No projects registered yet. Add a project from the sidebar first.',
    notARepo: 'This directory is not a Git repository.',
    detached: '(detached HEAD)',
    dirty: 'Dirty',
    clean: 'Clean',
    branches: 'Local branches ({{count}})',
    commits: 'Recent commits',
    noCommits: 'No commits yet.',
    error: {
      title: 'Git operation failed',
    },
  },
};
