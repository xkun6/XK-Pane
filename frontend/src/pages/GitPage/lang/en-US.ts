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
    actions: {
      fetch: 'Fetch',
      pull: 'Pull',
      push: 'Push',
      stash: 'Stash',
      stashPop: 'Stash Pop',
    },
    ops: {
      fetch: { title: 'Fetch complete', done: 'Fetched all remote refs.' },
      pull: { title: 'Pull complete', done: 'Merged remote changes.' },
      push: { title: 'Push complete', done: 'Pushed the current branch.' },
      stash: { title: 'Stash complete', done: 'Stashed working tree changes.' },
      stashPop: { title: 'Stash Pop complete', done: 'Popped the latest stash.' },
    },
    pushConfirm: {
      title: 'Push to remote',
      message: 'Push the current branch to the remote? This publishes commits externally.',
      ok: 'Push',
      cancel: 'Cancel',
    },
  },
};
