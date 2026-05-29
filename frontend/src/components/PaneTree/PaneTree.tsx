import { SplitView, normalizeSizesToHundred } from '@/components/SplitView';
import { useTerminal, type PaneNode } from '@/terminal';
import { PanelView } from './PanelView';

// 递归渲染外层面板树：split → SplitView 包裹各子 PaneTree（拖拽落地经 normalize→resizePanels）；
// panel → PanelView。等价于 cc-pane 的 PaneContainer + SplitContainer（D2：消费 React Context 而非 store）。
export const PaneTree = ({ node }: { node: PaneNode }) => {
  const { resizePanels } = useTerminal();

  if (node.type === 'panel') {
    return <PanelView panel={node} />;
  }

  return (
    <SplitView
      vertical={node.direction === 'vertical'}
      sizes={node.sizes}
      keys={node.children.map((c) => c.id)}
      onDragEnd={(sizes) => resizePanels(node.id, normalizeSizesToHundred(sizes))}
    >
      {node.children.map((child) => (
        <PaneTree key={child.id} node={child} />
      ))}
    </SplitView>
  );
};
