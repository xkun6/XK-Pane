import { Box, useTheme } from '@mui/material';
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
import { NativeDialogs } from '@/services';
import { mcpeStyles } from './McpePage.styles';
import { useMcpe, isPackageRunning } from './McpeContext';
import { PackageListItem } from './PackageListItem';
import { messageOf } from './useMcpePage';

// 包列表：行拆到 PackageListItem，整列表可 @dnd-kit 竖向拖拽排序（顺序持久化到后端）。
export const PackageList = () => {
  const theme = useTheme();
  const styles = mcpeStyles(theme);
  const t = useT();
  const { packages, activePackageId, runs, setActivePackage, deletePackage, reorderPackages } = useMcpe();

  const handleDelete = async (id: string, name: string) => {
    const ok = await NativeDialogs.confirm({
      title: t('mcpe.delete.title'),
      message: t('mcpe.delete.message', { name }),
      okLabel: t('mcpe.delete.ok'),
      cancelLabel: t('mcpe.delete.cancel'),
    });
    if (!ok) return;
    try {
      await deletePackage(id);
    } catch (e) {
      await NativeDialogs.error(t('mcpe.error.title'), messageOf(e));
    }
  };

  // 拖拽：距离 6px 才激活，避免与「点击选中包」冲突（同 WorkspaceTree）。
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }));

  const onDragEnd = (event: DragEndEvent): void => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const ids = packages.map((p) => p.id);
    const from = ids.indexOf(String(active.id));
    const to = ids.indexOf(String(over.id));
    if (from === -1 || to === -1) return;
    void reorderPackages(arrayMove(ids, from, to)).catch(async (e) => {
      await NativeDialogs.error(t('mcpe.error.title'), messageOf(e));
    });
  };

  if (packages.length === 0) {
    return <Box sx={styles.listEmpty}>{t('mcpe.list.empty')}</Box>;
  }

  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
      <Box>
        <SortableContext items={packages.map((p) => p.id)} strategy={verticalListSortingStrategy}>
          {packages.map((pkg) => (
            <PackageListItem
              key={pkg.id}
              pkg={pkg}
              active={pkg.id === activePackageId}
              running={isPackageRunning(runs, pkg.id)}
              styles={styles}
              deleteLabel={t('mcpe.action.delete')}
              runningLabel={t('mcpe.list.running')}
              onSelect={() => setActivePackage(pkg.id)}
              onDelete={() => void handleDelete(pkg.id, pkg.name)}
            />
          ))}
        </SortableContext>
      </Box>
    </DndContext>
  );
};
