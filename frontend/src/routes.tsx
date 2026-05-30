import HomeRoundedIcon from '@mui/icons-material/HomeRounded';
import SettingsRoundedIcon from '@mui/icons-material/SettingsRounded';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import ChecklistRoundedIcon from '@mui/icons-material/ChecklistRounded';
import AccountTreeRoundedIcon from '@mui/icons-material/AccountTreeRounded';
import HistoryRoundedIcon from '@mui/icons-material/HistoryRounded';
import DescriptionRoundedIcon from '@mui/icons-material/DescriptionRounded';
import PsychologyRoundedIcon from '@mui/icons-material/PsychologyRounded';
import { lazy } from 'react';
import { HomePage } from '@/pages/HomePage/HomePage';
import { TodoPage } from '@/pages/TodoPage';
import { GitPage } from '@/pages/GitPage';
import { HistoryPage } from '@/pages/HistoryPage';
import { SpecsPage } from '@/pages/SpecsPage';
import { MemorysPage } from '@/pages/MemorysPage';
import { HomePageSkeleton } from '@/pages/HomePage/HomePage.skeleton';
import { SettingsPageSkeleton } from '@/pages/SettingsPage/SettingsPage.skeleton';
import { TerminalPageSkeleton } from '@/pages/TerminalPage/TerminalPage.skeleton';
import type { RouteDefinition } from '@/router';

// SettingsPage 走 React.lazy：调色板编辑器较重，首屏不进入 main bundle。
// HomePage 保持同步加载（首页常用 + 体积小，lazy 反而引入一次额外的闪烁）。
//
// 注意：lazy import 必须从具体子文件 (SettingsPage.tsx) 导入，
// 不要从 index.ts 走 —— 否则与 App.tsx 静态 import 同 index 合并到主 chunk。
const SettingsPage = lazy(async () => {
  const mod = await import('@/pages/SettingsPage/SettingsPage');
  return { default: mod.SettingsPage };
});

// TerminalPage 走 lazy：xterm 较重，拆出主 bundle；keepAlive 保活，避免离开终端页时丢失
// xterm 实例与滚动回溯（会话状态由 TerminalProvider 持有，xterm DOM 由 keepAlive 保留）。
const TerminalPage = lazy(async () => {
  const mod = await import('@/pages/TerminalPage/TerminalPage');
  return { default: mod.TerminalPage };
});

// 路由表是唯一事实源：
// - 新增页面：在这里追加一项即可，Sidebar 会自动渲染对应导航按钮
// - id 必须唯一；slot 决定 Sidebar 把它渲染到顶部还是底部
// - 想要不出现在 Sidebar 但仍可 navigate(id) 进入的页面，设 slot: 'hidden'
// - keepAlive 默认 false（离开即卸载）；若想保留输入 / 滚动 / 局部 state，设 true
// - fallback：lazy 组件加载期间的占位（推荐传该页面的骨架屏组件）
// - labelKey：i18n 路径（约定 'route.<id>'），渲染时 t(labelKey) 翻译；
//   label 是兜底字面量（未配置 labelKey 或翻译失败时使用）
export const routes: RouteDefinition[] = [
  {
    id: 'home',
    labelKey: 'route.home',
    label: 'Home',
    icon: HomeRoundedIcon,
    element: <HomePage />,
    fallback: <HomePageSkeleton />,
    slot: 'primary',
    keepAlive: true, // 演示：离开后再回来，Greet 输入框保留上次内容
  },
  {
    id: 'terminal',
    labelKey: 'route.terminal',
    label: 'Terminal',
    icon: TerminalRoundedIcon,
    element: <TerminalPage />,
    fallback: <TerminalPageSkeleton />,
    slot: 'primary',
    keepAlive: true, // 保活：保留 xterm 实例与滚动回溯（终端页离开后再回不重建会话视图）
  },
  {
    id: 'todo',
    labelKey: 'route.todo',
    label: 'Todos',
    icon: ChecklistRoundedIcon,
    element: <TodoPage />,
    slot: 'primary',
  },
  {
    id: 'git',
    labelKey: 'route.git',
    label: 'Git',
    icon: AccountTreeRoundedIcon,
    element: <GitPage />,
    slot: 'primary',
  },
  {
    id: 'history',
    labelKey: 'route.history',
    label: 'History',
    icon: HistoryRoundedIcon,
    element: <HistoryPage />,
    slot: 'primary',
  },
  {
    id: 'specs',
    labelKey: 'route.specs',
    label: 'Specs',
    icon: DescriptionRoundedIcon,
    element: <SpecsPage />,
    slot: 'primary',
  },
  {
    id: 'memory',
    labelKey: 'route.memory',
    label: 'Memory',
    icon: PsychologyRoundedIcon,
    element: <MemorysPage />,
    slot: 'primary',
  },
  {
    id: 'settings',
    labelKey: 'route.settings',
    label: 'Settings',
    icon: SettingsRoundedIcon,
    element: <SettingsPage />,
    fallback: <SettingsPageSkeleton />,
    slot: 'footer',
  },
];
