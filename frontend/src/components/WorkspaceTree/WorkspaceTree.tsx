import { useState } from 'react';
import { Box, useTheme } from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import { useT } from '@/i18n';
import { useCliTools } from '@/shared/hooks/useCliTools';
import { TextInputDialog } from '@/components/TextInputDialog';
import type { Workspace, WorkspaceProject } from '@/services';
import { workspaceTreeStyles } from './WorkspaceTree.styles';
import { useWorkspaceTree } from './useWorkspaceTree';
import { WorkspaceItem, type WorkspaceItemStrings } from './WorkspaceItem';
import {
  WorkspaceContextMenu,
  type MenuAnchor,
  type WorkspaceMenuState,
  type WorkspaceMenuStrings,
} from './WorkspaceContextMenu';

// 项目无别名时的显示名（路径末段）。
function basename(path: string): string {
  const norm = path.replace(/[\\/]+$/, '');
  const idx = Math.max(norm.lastIndexOf('/'), norm.lastIndexOf('\\'));
  return idx >= 0 ? norm.slice(idx + 1) : norm;
}
const projectDisplayName = (p: WorkspaceProject): string => p.alias ?? basename(p.path);

interface TextDialogConfig {
  title: string;
  label: string;
  okLabel: string;
  initialValue: string;
  allowEmpty: boolean;
  onSubmit: (value: string) => void;
}

// WorkspaceTree：Workspace 两层折叠树（终端页左列，取代旧 ProjectPanel）。
// 第一层工作空间（pinned 置顶分组）→ 第二层项目节点。自包含消费 WorkspaceContext + TerminalContext。
// 看 / 建 / 删 / 收藏 / 折叠 / 右键菜单（CLI 启动 + 导入项目 / 重命名 / 别名 / 移除 / 复制路径）。
// @dnd-kit 拖拽排序在后续单元加入。
export const WorkspaceTree = () => {
  const theme = useTheme();
  const styles = workspaceTreeStyles(theme);
  const t = useT();
  const vm = useWorkspaceTree();
  const { tools, loading: toolsLoading } = useCliTools();

  const [menu, setMenu] = useState<WorkspaceMenuState | null>(null);
  const [textOpen, setTextOpen] = useState(false);
  const [textConfig, setTextConfig] = useState<TextDialogConfig | null>(null);

  const errTitle = (): string => t('workspaceTree.error.title');
  const openText = (cfg: TextDialogConfig): void => {
    setTextConfig(cfg);
    setTextOpen(true);
  };
  const closeMenu = (): void => setMenu(null);

  const itemStrings: WorkspaceItemStrings = {
    pin: t('workspaceTree.pin'),
    unpin: t('workspaceTree.unpin'),
    expand: t('workspaceTree.expand'),
    collapse: t('workspaceTree.collapse'),
    noProjects: t('workspaceTree.noProjects'),
    projectCountAria: (count) => t('workspaceTree.projectCountAria', { count }),
  };

  const menuStrings: WorkspaceMenuStrings = {
    openTerminal: t('workspaceTree.menu.openTerminal'),
    launchLoading: t('workspaceTree.menu.launchLoading'),
    notInstalled: t('workspaceTree.menu.notInstalled'),
    importProject: t('workspaceTree.menu.importProject'),
    rename: t('workspaceTree.menu.rename'),
    setAlias: t('workspaceTree.menu.setAlias'),
    delete: t('workspaceTree.delete'),
    copyPath: t('workspaceTree.menu.copyPath'),
    removeProject: t('workspaceTree.menu.removeProject'),
  };

  // —— 菜单动作（读取当前 menu 目标后关闭菜单）——
  const handleLaunch = (cliTool: string): void => {
    if (menu?.kind === 'workspace') {
      const ws = menu.workspace;
      vm.launch({ id: ws.id, name: ws.alias ?? ws.name, path: ws.path }, cliTool);
    } else if (menu?.kind === 'project') {
      const p = menu.project;
      vm.launch({ id: p.id, name: projectDisplayName(p), path: p.path }, cliTool);
    }
    closeMenu();
  };

  const handleImportProject = (): void => {
    if (menu?.kind === 'workspace') {
      void vm.importProject(menu.workspace.id, {
        dialogTitle: t('workspaceTree.menu.importDialogTitle'),
        errorTitle: errTitle(),
      });
    }
    closeMenu();
  };

  const handleRename = (): void => {
    if (menu?.kind === 'workspace') {
      const ws = menu.workspace;
      openText({
        title: t('workspaceTree.renameDialog.title'),
        label: t('workspaceTree.renameDialog.label'),
        okLabel: t('workspaceTree.dialog.save'),
        initialValue: ws.name,
        allowEmpty: false,
        onSubmit: (name) => void vm.submitRename(ws.id, name, errTitle()),
      });
    }
    closeMenu();
  };

  const handleSetAlias = (): void => {
    if (menu?.kind === 'workspace') {
      const ws = menu.workspace;
      openText({
        title: t('workspaceTree.aliasDialog.title'),
        label: t('workspaceTree.aliasDialog.label'),
        okLabel: t('workspaceTree.dialog.save'),
        initialValue: ws.alias ?? '',
        allowEmpty: true, // 留空 → 清除别名
        onSubmit: (alias) => void vm.submitAlias(ws.id, alias, errTitle()),
      });
    } else if (menu?.kind === 'project') {
      const p = menu.project;
      openText({
        title: t('workspaceTree.aliasDialog.title'),
        label: t('workspaceTree.aliasDialog.label'),
        okLabel: t('workspaceTree.dialog.save'),
        initialValue: p.alias ?? '',
        allowEmpty: true,
        onSubmit: (alias) => void vm.submitProjectAlias(p.id, alias, errTitle()),
      });
    }
    closeMenu();
  };

  const handleDelete = (): void => {
    if (menu?.kind === 'workspace') {
      const ws = menu.workspace;
      void vm.deleteWorkspace(ws, {
        confirmTitle: t('workspaceTree.deleteConfirm.title'),
        confirmMessage: t('workspaceTree.deleteConfirm.message', { name: ws.alias ?? ws.name }),
        okLabel: t('workspaceTree.delete'),
        cancelLabel: t('common.cancel'),
        errorTitle: errTitle(),
      });
    }
    closeMenu();
  };

  const handleCopyPath = (): void => {
    if (menu?.kind === 'project') vm.copyPath(menu.project.path);
    closeMenu();
  };

  const handleRemoveProject = (): void => {
    if (menu?.kind === 'project') {
      const p = menu.project;
      void vm.removeProject(p.id, {
        confirmTitle: t('workspaceTree.menu.removeProjectConfirm.title'),
        confirmMessage: t('workspaceTree.menu.removeProjectConfirm.message', {
          name: projectDisplayName(p),
        }),
        okLabel: t('workspaceTree.menu.removeProject'),
        cancelLabel: t('common.cancel'),
        errorTitle: errTitle(),
      });
    }
    closeMenu();
  };

  const openCreate = (): void => {
    openText({
      title: t('workspaceTree.dialog.createTitle'),
      label: t('workspaceTree.dialog.nameLabel'),
      okLabel: t('workspaceTree.dialog.create'),
      initialValue: '',
      allowEmpty: false,
      onSubmit: (name) => void vm.submitCreate(name, errTitle()),
    });
  };

  const renderItem = (ws: Workspace) => (
    <WorkspaceItem
      key={ws.id}
      workspace={ws}
      expanded={vm.isExpanded(ws.id)}
      styles={styles}
      strings={itemStrings}
      onToggleExpand={() => vm.toggleExpand(ws.id)}
      onTogglePinned={() => void vm.togglePinned(ws, errTitle())}
      onOpenMenu={(anchor: MenuAnchor) => setMenu({ kind: 'workspace', workspace: ws, anchor })}
      onOpenProjectMenu={(project, anchor) => setMenu({ kind: 'project', project, anchor })}
    />
  );

  const isEmpty = !vm.loading && vm.pinned.length === 0 && vm.normal.length === 0;

  return (
    <Box sx={styles.root}>
      <Box sx={styles.header}>
        <Box component="span" sx={styles.title}>
          {t('workspaceTree.title')}
        </Box>
      </Box>

      <Box sx={styles.list}>
        {isEmpty && (
          <Box sx={styles.empty}>
            <Box sx={styles.emptyText}>{t('workspaceTree.empty')}</Box>
            <Box sx={styles.emptyHint}>{t('workspaceTree.emptyHint')}</Box>
          </Box>
        )}
        {vm.pinned.map(renderItem)}
        {vm.normal.map(renderItem)}
      </Box>

      <Box
        role="button"
        tabIndex={0}
        sx={styles.newButton}
        onClick={openCreate}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            openCreate();
          }
        }}
      >
        <AddRoundedIcon fontSize="small" />
        <Box component="span">{t('workspaceTree.newWorkspace')}</Box>
      </Box>

      <WorkspaceContextMenu
        menu={menu}
        tools={tools}
        toolsLoading={toolsLoading}
        launchDisabled={menu?.kind === 'workspace' && !menu.workspace.path}
        strings={menuStrings}
        onClose={closeMenu}
        onLaunch={handleLaunch}
        onImportProject={handleImportProject}
        onRename={handleRename}
        onSetAlias={handleSetAlias}
        onDelete={handleDelete}
        onCopyPath={handleCopyPath}
        onRemoveProject={handleRemoveProject}
      />

      <TextInputDialog
        open={textOpen}
        title={textConfig?.title ?? ''}
        label={textConfig?.label ?? ''}
        okLabel={textConfig?.okLabel ?? ''}
        cancelLabel={t('common.cancel')}
        initialValue={textConfig?.initialValue ?? ''}
        allowEmpty={textConfig?.allowEmpty ?? false}
        onSubmit={(value) => textConfig?.onSubmit(value)}
        onClose={() => setTextOpen(false)}
      />
    </Box>
  );
};
