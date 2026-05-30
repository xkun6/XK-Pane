import type { Messages } from '@/i18n';

// SpecsPage English copy, under the specs.* namespace.
export const specsPageEnUS: Messages = {
  specs: {
    eyebrow: 'XK-Pane',
    title: 'Specs',
    project: 'Project',
    refresh: 'Refresh',
    noProjects: 'No projects registered yet. Add a project from the sidebar first.',
    createPlaceholder: 'New spec title, press Enter',
    create: 'Create',
    list: 'Specs ({{count}})',
    empty: 'No specs in this project yet. Create one above.',
    save: 'Save',
    contentLoading: 'Loading content…',
    status: {
      draft: 'Draft',
      active: 'Active',
      archived: 'Archived',
    },
    actions: {
      activate: 'Set active',
      archive: 'Archive',
      remove: 'Delete',
    },
    saved: {
      title: 'Saved',
      message: 'Spec content written to file.',
    },
    archiveConfirm: {
      title: 'Archive spec',
      message: 'Archive "{{title}}"? The file moves to the archived/ subdirectory.',
      ok: 'Archive',
      cancel: 'Cancel',
    },
    removeConfirm: {
      title: 'Delete spec',
      message: 'Delete "{{title}}"? Its .md file is removed too. This cannot be undone.',
      ok: 'Delete',
      cancel: 'Cancel',
    },
    error: {
      title: 'Spec operation failed',
    },
  },
};
