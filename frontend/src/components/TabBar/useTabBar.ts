// TabBar 的 ViewModel：仅管「活动标签滚动入视」——多标签横向溢出时，切到的活动标签自动滚入可见区。
// 标签选择/关闭/新建为纯回调，由 View 直接透传（无业务状态）。
import { useCallback, useEffect, useRef } from 'react';

export interface TabBarModel {
  scrollRef: React.RefObject<HTMLDivElement | null>;
  registerTabNode: (tabId: string, node: HTMLDivElement | null) => void;
}

export function useTabBar(activeTabId: string, tabCount: number): TabBarModel {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const tabNodes = useRef(new Map<string, HTMLDivElement>());

  const registerTabNode = useCallback((tabId: string, node: HTMLDivElement | null) => {
    if (node) tabNodes.current.set(tabId, node);
    else tabNodes.current.delete(tabId);
  }, []);

  useEffect(() => {
    const node = tabNodes.current.get(activeTabId);
    if (!node || typeof node.scrollIntoView !== 'function') return;
    node.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'nearest' });
  }, [activeTabId, tabCount]);

  return { scrollRef, registerTabNode };
}
