import type { Messages } from '@/i18n';

// MemorysPage English copy, under the memory.* namespace.
export const memorysPageEnUS: Messages = {
  memory: {
    eyebrow: 'XK-Pane',
    title: 'Memory',
    refresh: 'Refresh',
    searchPlaceholder: 'Search title / content / tags…',
    list: 'Memories ({{count}})',
    empty: 'No memories yet. Create one above.',
    editTitle: 'Edit memory',
    save: 'Save',
    stats: {
      total: 'Total {{count}}',
    },
    filter: {
      scope: 'Scope',
      category: 'Category',
      all: 'All',
    },
    scope: {
      global: 'Global',
      workspace: 'Workspace',
      project: 'Project',
      session: 'Session',
    },
    category: {
      decision: 'Decision',
      lesson: 'Lesson',
      preference: 'Preference',
      pattern: 'Pattern',
      fact: 'Fact',
      plan: 'Plan',
    },
    field: {
      titlePlaceholder: 'Title',
      contentPlaceholder: 'Content…',
      titleLabel: 'Title',
      contentLabel: 'Content',
      scope: 'Scope',
      scopeRef: 'Scope reference',
      workspaceName: 'Workspace name',
      projectPath: 'Project path',
      sessionId: 'Session ID',
      category: 'Category',
      importance: 'Importance',
      tags: 'Tags',
      tagsPlaceholder: 'Comma-separated, e.g. auth, style',
    },
    create: {
      title: 'New memory',
      action: 'Create',
      scopeRefRequired: 'This scope requires its reference (workspace name / project path / session ID).',
    },
    saved: {
      title: 'Saved',
      message: 'Memory updated.',
    },
    removeConfirm: {
      title: 'Delete memory',
      message: 'Delete "{{title}}"? This cannot be undone.',
      ok: 'Delete',
      cancel: 'Cancel',
    },
    actions: {
      remove: 'Delete',
    },
    error: {
      title: 'Memory operation failed',
    },
  },
};
