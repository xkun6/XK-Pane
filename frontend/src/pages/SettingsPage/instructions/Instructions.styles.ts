import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface InstructionsSx {
  sectionGrow: S;
  root: S;
  tabs: S;
  tab: S;
  toolbar: S;
  pathLabel: S;
  toolbarActions: S;
  unsavedBadge: S;
  loadErrorText: S;
  editorContainer: S;
}

export const instructionsStyles = (theme: Theme): InstructionsSx => {
  const fp = theme.palette.foundation;
  return {
    // 让整块 section / root 在右列里纵向撑满，编辑器吃掉剩余高度；
    // 窗口过矮时依赖 editorContainer 的 minHeight 触发外层 detail 滚动。
    sectionGrow: { flex: 1 },
    root: { display: 'flex', flexDirection: 'column', gap: 2, flex: 1 },
    tabs: { minHeight: 34, flexShrink: 0, borderBottom: `1px solid ${fp.divider}` },
    tab: { minHeight: 34, textTransform: 'none', fontSize: 13 },
    toolbar: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1.5 },
    pathLabel: {
      fontSize: 12,
      color: fp.text.muted,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
    },
    toolbarActions: { display: 'flex', alignItems: 'center', gap: 1 },
    unsavedBadge: { fontSize: 12, color: fp.status.warning, fontWeight: 600 },
    loadErrorText: { fontSize: 12, color: fp.status.danger },
    editorContainer: {
      flex: 1,
      minHeight: 360,
      display: 'flex',
      flexDirection: 'column',
      border: `1px solid ${fp.divider}`,
      borderRadius: 1,
      overflow: 'hidden',
    },
  };
};
