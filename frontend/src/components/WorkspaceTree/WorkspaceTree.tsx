import { useState } from 'react';
import { Box, useTheme } from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import {
  closestCenter,
  DndContext,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import { SortableContext, arrayMove, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { useT } from '@/i18n';
import { useCliTools } from '@/shared/hooks/useCliTools';
import { useSshMachines } from '@/sshmachine';
import { TextInputDialog } from '@/components/TextInputDialog';
import type { Workspace } from '@/services';
import { workspaceTreeStyles } from './WorkspaceTree.styles';
import { useWorkspaceTree } from './useWorkspaceTree';
import { WorkspaceItem, type WorkspaceItemStrings } from './WorkspaceItem';
import {
  WorkspaceContextMenu,
  type MenuAnchor,
  type WorkspaceMenuState,
  type WorkspaceMenuStrings,
} from './WorkspaceContextMenu';
import {
  AddSshProjectDialog,
  type AddSshProjectDialogStrings,
  type SshProjectDraft,
} from './AddSshProjectDialog';
import { projectDisplayName, sshAddress } from './projectDisplay';

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
  const { machines } = useSshMachines();

  const [menu, setMenuState] = useState<WorkspaceMenuState | null>(null);
  const [textOpen, setTextOpen] = useState(false);
  const [textConfig, setTextConfig] = useState<TextDialogConfig | null>(null);
  // 「添加 SSH 项目」对话框：记住目标工作空间 id（从工作空间右键菜单打开）。
  const [sshTargetWsId, setSshTargetWsId] = useState<string | null>(null);
  // 「在隔离 worktree 中启动」模式开关：每次打开菜单重置为关（一次性选择，不持久）。
  const [worktreeMode, setWorktreeMode] = useState(false);

  const errTitle = (): string => t('workspaceTree.error.title');
  const openText = (cfg: TextDialogConfig): void => {
    setTextConfig(cfg);
    setTextOpen(true);
  };
  const setMenu = (next: WorkspaceMenuState | null): void => {
    setWorktreeMode(false);
    setMenuState(next);
  };
  const closeMenu = (): void => setMenu(null);

  // 拖拽：距离 6px 才激活，避免与点击展开 / 弹菜单冲突。
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }));

  // 拖拽结束：工作空间限同组（pinned/normal）排序；项目限同一工作空间内排序。跨组 / 跨空间忽略。
  const onDragEnd = (event: DragEndEvent): void => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const activeId = String(active.id);
    const overId = String(over.id);

    const inPinned = vm.pinned.some((w) => w.id === activeId);
    const inNormal = vm.normal.some((w) => w.id === activeId);
    if (inPinned || inNormal) {
      const group = inPinned ? vm.pinned : vm.normal;
      const ids = group.map((w) => w.id);
      const from = ids.indexOf(activeId);
      const to = ids.indexOf(overId);
      if (from === -1 || to === -1) return; // over 在另一组 → 忽略（跨组靠星标，不靠拖拽）
      void vm.reorderWorkspaces(arrayMove(ids, from, to), errTitle());
      return;
    }

    // 项目：找归属工作空间，限同空间内排序。
    const all = [...vm.pinned, ...vm.normal];
    const owner = all.find((w) => w.projects.some((p) => p.id === activeId));
    const overOwner = all.find((w) => w.projects.some((p) => p.id === overId));
    if (!owner || !overOwner || owner.id !== overOwner.id) return;
    const ids = owner.projects.map((p) => p.id);
    const from = ids.indexOf(activeId);
    const to = ids.indexOf(overId);
    if (from === -1 || to === -1) return;
    void vm.reorderProjects(owner.id, arrayMove(ids, from, to), errTitle());
  };

  const itemStrings: WorkspaceItemStrings = {
    pin: t('workspaceTree.pin'),
    unpin: t('workspaceTree.unpin'),
    expand: t('workspaceTree.expand'),
    collapse: t('workspaceTree.collapse'),
    noProjects: t('workspaceTree.noProjects'),
    sessionsToggle: t('workspaceTree.sessions.toggle'),
    projectCountAria: (count) => t('workspaceTree.projectCountAria', { count }),
  };

  const sshDialogStrings: AddSshProjectDialogStrings = {
    title: t('workspaceTree.sshDialog.title'),
    machineLabel: t('workspaceTree.sshDialog.machineLabel'),
    pathLabel: t('workspaceTree.sshDialog.pathLabel'),
    pathPlaceholder: t('workspaceTree.sshDialog.pathPlaceholder'),
    add: t('workspaceTree.sshDialog.add'),
    cancel: t('common.cancel'),
    noMachines: t('workspaceTree.sshDialog.noMachines'),
    noMachinesHint: t('workspaceTree.sshDialog.noMachinesHint'),
  };

  const menuStrings: WorkspaceMenuStrings = {
    openTerminal: t('workspaceTree.menu.openTerminal'),
    launchLoading: t('workspaceTree.menu.launchLoading'),
    notInstalled: t('workspaceTree.menu.notInstalled'),
    launchInWorktree: t('workspaceTree.menu.launchInWorktree'),
    importProject: t('workspaceTree.menu.importProject'),
    addSSHProject: t('workspaceTree.menu.addSSHProject'),
    rename: t('workspaceTree.menu.rename'),
    setAlias: t('workspaceTree.menu.setAlias'),
    delete: t('workspaceTree.delete'),
    copyPath: t('workspaceTree.menu.copyPath'),
    copyName: t('workspaceTree.menu.copyName'),
    removeProject: t('workspaceTree.menu.removeProject'),
    openInExplorer: t('workspaceTree.menu.openInExplorer'),
    openInFileBrowser: t('workspaceTree.menu.openInFileBrowser'),
  };

  // —— 菜单动作（读取当前 menu 目标后关闭菜单）——
  const handleLaunch = (cliTool: string): void => {
    if (menu?.kind === 'workspace') {
      const ws = menu.workspace;
      vm.launch({ id: ws.id, name: ws.alias ?? ws.name, path: ws.path }, cliTool);
    } else if (menu?.kind === 'project') {
      const p = menu.project;
      const target = { id: p.id, name: projectDisplayName(p), path: p.path, ssh: p.ssh };
      // worktree 模式（仅本地项目可勾选）：先建隔离 worktree 再启动，失败弹原生错误框。
      if (worktreeMode && !p.ssh) {
        void vm.launchInWorktree(target, cliTool, errTitle());
      } else {
        vm.launch(target, cliTool);
      }
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

  // 「添加 SSH 项目」：记住目标工作空间 → 打开机器选择对话框（关闭右键菜单）。
  const handleAddSSHProject = (): void => {
    if (menu?.kind === 'workspace') setSshTargetWsId(menu.workspace.id);
    closeMenu();
  };

  // 对话框提交：按 draft.machineId 取机器 DTO，合成 SshProjectInfo（remotePath 留空兜底 defaultPath）。
  const submitSSHProject = (draft: SshProjectDraft): void => {
    const wsId = sshTargetWsId;
    const m = machines.find((x) => x.id === draft.machineId);
    setSshTargetWsId(null);
    if (!wsId || !m) return;
    void vm.addSSHProject(
      wsId,
      {
        machineId: m.id,
        host: m.host,
        port: m.port,
        user: m.user,
        remotePath: draft.remotePath || m.defaultPath,
        identityFile: m.identityFile,
        authMethod: m.authMethod,
      },
      errTitle(),
    );
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
    // SSH 项目复制友好地址 user@host:remotePath；本地项目复制磁盘路径（合成键无复制价值）。
    if (menu?.kind === 'project') {
      const p = menu.project;
      vm.copyPath(p.ssh ? sshAddress(p) : p.path);
    }
    closeMenu();
  };

  // 复制项目显示名（别名优先 → SSH user@host → 路径末段），复用 copyPath 的剪贴板写入。
  const handleCopyName = (): void => {
    if (menu?.kind === 'project') vm.copyPath(projectDisplayName(menu.project));
    closeMenu();
  };

  const handleOpenInExplorer = (): void => {
    if (menu?.kind === 'project') void vm.openInExplorer(menu.project.path, errTitle());
    closeMenu();
  };

  const handleOpenInFileBrowser = (): void => {
    if (menu?.kind === 'project') vm.openInFileBrowser(menu.project.path);
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

      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
        <Box sx={styles.list}>
          {isEmpty && (
            <Box sx={styles.empty}>
              <Box sx={styles.emptyText}>{t('workspaceTree.empty')}</Box>
              <Box sx={styles.emptyHint}>{t('workspaceTree.emptyHint')}</Box>
            </Box>
          )}
          <SortableContext items={vm.pinned.map((w) => w.id)} strategy={verticalListSortingStrategy}>
            {vm.pinned.map(renderItem)}
          </SortableContext>
          <SortableContext items={vm.normal.map((w) => w.id)} strategy={verticalListSortingStrategy}>
            {vm.normal.map(renderItem)}
          </SortableContext>
        </Box>
      </DndContext>

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
        worktreeMode={worktreeMode}
        strings={menuStrings}
        onClose={closeMenu}
        onLaunch={handleLaunch}
        onToggleWorktreeMode={() => setWorktreeMode((v) => !v)}
        onImportProject={handleImportProject}
        onAddSSHProject={handleAddSSHProject}
        onRename={handleRename}
        onSetAlias={handleSetAlias}
        onDelete={handleDelete}
        onCopyPath={handleCopyPath}
        onCopyName={handleCopyName}
        onRemoveProject={handleRemoveProject}
        onOpenInExplorer={handleOpenInExplorer}
        onOpenInFileBrowser={handleOpenInFileBrowser}
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

      <AddSshProjectDialog
        open={sshTargetWsId !== null}
        machines={machines}
        strings={sshDialogStrings}
        onSubmit={submitSSHProject}
        onClose={() => setSshTargetWsId(null)}
      />
    </Box>
  );
};
