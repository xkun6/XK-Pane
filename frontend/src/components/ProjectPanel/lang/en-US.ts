import type { Messages } from '@/i18n';

export const projectPanelEnUS: Messages = {
  projectPanel: {
    title: 'Projects',
    add: 'Add project',
    addDialogTitle: 'Select a project folder',
    empty: 'No projects yet.\nClick + above to add a folder.',
    remove: 'Remove project',
    removeConfirm: {
      title: 'Remove project',
      message: 'Remove project "{{name}}"? This only removes it from the list; files on disk are untouched.',
    },
    error: {
      title: 'Project operation failed',
    },
  },
};
