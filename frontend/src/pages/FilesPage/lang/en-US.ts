import type { Messages } from '@/i18n';

// FilesPage English copy, namespace files.*.
export const filesPageEnUS: Messages = {
  files: {
    noFolder: 'No folder open',
    openFolder: 'Open folder',
    openFolderTitle: 'Choose a directory to open in the file browser',
    reveal: 'Reveal in system file manager',
    desktopOnly: 'The file browser is only available in the desktop app.',
    saveErrorTitle: 'Save failed',
    empty: {
      title: 'No folder open yet',
      hint: 'Open a folder to browse and edit. You can also right-click a directory in the workspace and choose “Open in file browser”.',
      openFolder: 'Open folder',
    },
    recent: {
      title: 'Recent',
    },
    tree: {
      ariaLabel: 'File directory tree',
      loading: 'Loading…',
      error: 'Failed to read directory',
      empty: 'Empty directory',
    },
    editor: {
      selectFile: 'Select a file on the left to view / edit',
      save: 'Save',
      dirty: 'Unsaved changes',
      binary: 'Binary file — cannot edit as text.',
      loadError: 'Failed to read file.',
      source: 'Source',
      preview: 'Preview',
    },
  },
};
