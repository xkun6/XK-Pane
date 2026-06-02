import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react';

// 右侧可切换面板（VSCode 右栏风格）的全局开关上下文。
//
// 与「路由页面」并列但语义不同：页面是导航目标（navigate 切换、占满主区），
// 面板是「叠在主区右侧、点同一图标开/关」的辅助视图。Git / 本地历史不再是整页，
// 而是绑定「当前目录」的右侧抽屉 —— 在任何页面（文件页 / 终端页）都能拉出。
//
// 放在 RouterProvider 之上：Sidebar（开关来源）与 AppLayout（抽屉宿主）都在其内，
// 二者通过本 context 共享 active，无需给 Sidebar 传 props（遵「Sidebar 不维护菜单数组」铁律）。
export type RightPanelId = 'git' | 'history';

export interface RightPanelContextValue {
  active: RightPanelId | null;
  toggle: (id: RightPanelId) => void;
  open: (id: RightPanelId) => void;
  close: () => void;
}

const RightPanelContext = createContext<RightPanelContextValue | null>(null);

export const RightPanelProvider = ({ children }: { children: ReactNode }) => {
  const [active, setActive] = useState<RightPanelId | null>(null);

  const toggle = useCallback((id: RightPanelId): void => {
    setActive((cur) => (cur === id ? null : id));
  }, []);
  const open = useCallback((id: RightPanelId): void => setActive(id), []);
  const close = useCallback((): void => setActive(null), []);

  const value = useMemo<RightPanelContextValue>(
    () => ({ active, toggle, open, close }),
    [active, toggle, open, close],
  );

  return <RightPanelContext.Provider value={value}>{children}</RightPanelContext.Provider>;
};

export const useRightPanel = (): RightPanelContextValue => {
  const ctx = useContext(RightPanelContext);
  if (!ctx) {
    throw new Error('useRightPanel 必须在 <RightPanelProvider> 内使用');
  }
  return ctx;
};
