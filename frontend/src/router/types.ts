import type { ComponentType, ReactNode } from 'react';

// 路由展示槽位：决定 Sidebar 在哪一段渲染该项的导航按钮。
// - 'primary'：顶部主导航
// - 'footer'：底部固定区（设置类入口）
// - 'hidden'：不进入 Sidebar，仅可通过 navigate(id) 进入
export type RouteSlot = 'primary' | 'footer' | 'hidden';

export interface RouteDefinition {
  id: string;                         // 唯一标识，对应 navigate(id) / activeId
  // i18n key，会用 t(labelKey) 翻译；未配置时回落到 label。
  // 路径形式：'route.<id>'（约定，详见 i18n SKILL）。
  labelKey?: string;
  // 兜底 label（无 i18n key 时用 / 翻译失败时显示的字面量）
  label: string;
  icon?: ComponentType<{ fontSize?: 'small' | 'medium' | 'large' | 'inherit' }>;
  // 页面组件实例。面板型条目（panel 已设）没有整页，留空即可——它不经 RouterOutlet 渲染。
  element?: ReactNode;
  // 面板型条目：Sidebar 把该图标渲染成「右侧抽屉开关」而非导航按钮，点击 toggle 对应 RightPanel。
  // 设了 panel 的条目不应再被 navigate（无 element），仅作为 Sidebar 上的切换入口。
  panel?: 'git' | 'history';
  slot?: RouteSlot;                   // 默认 'primary'
  // 是否缓存页面状态：
  // - false（默认）：离开时卸载组件，下次进入重新挂载（state / form / fetch 全部重置）
  // - true：首次访问后常驻 DOM，靠 display:none 切换显隐，重新进入时保留输入 / 滚动 / 局部 state
  // 仅对真正需要保留状态的页面打开（如填到一半的表单、长列表的滚动位置）。
  keepAlive?: boolean;
  // Suspense fallback：当 element 是 React.lazy 时显示的占位（如骨架屏）。
  // 推荐每个页面提供与版式对齐的 <PageNameSkeleton />，避免加载时布局跳动。
  fallback?: ReactNode;
  // 仅桌面可用（依赖本机文件系统等桌面专属能力）。Web（viewer）端 Sidebar 隐藏其入口。
  desktopOnly?: boolean;
}

export interface RouterContextValue {
  routes: RouteDefinition[];          // 全部已注册路由（含 hidden）
  active: RouteDefinition;            // 当前激活路由
  navigate: (id: string) => void;     // 跳转
  primary: RouteDefinition[];         // slot === 'primary'
  footer: RouteDefinition[];          // slot === 'footer'
}
