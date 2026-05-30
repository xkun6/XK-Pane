import { AppLayout } from '@/components/AppLayout';
import { I18nProvider, registerFoundationLocales } from '@/i18n';
import { registerHomePageLocales } from '@/pages/HomePage/lang';
import { registerSettingsPageLocales } from '@/pages/SettingsPage/lang';
import { registerTodoPageLocales } from '@/pages/TodoPage';
import { registerGitPageLocales } from '@/pages/GitPage';
import { registerHistoryPageLocales } from '@/pages/HistoryPage';
import { registerSpecsPageLocales } from '@/pages/SpecsPage';
import { registerMemorysPageLocales } from '@/pages/MemorysPage';
import { registerProjectPanelLocales } from '@/components/ProjectPanel';
import { PreferencesProvider } from '@/preferences';
import { ProjectProvider } from '@/project';
import { ProviderProvider } from '@/provider';
import { TodoProvider } from '@/todo';
import { RouterProvider } from '@/router';
import { routes } from '@/routes';
import { FoundationThemeProvider, registerFoundationThemes } from '@/styles/themes';
import { TerminalProvider } from '@/terminal';

// 启动时注册：
//  1. 内置主题预设（light / dark / obsidian + 已保存的 custom）
//  2. 内置 locale 公用文案（zh-CN / en-US）
//  3. 各页面的语言包（深合并到对应 locale 上）
// 所有 register* 都是幂等，模块多次求值不会重复注册。
// 注意：页面 locale 必须在 registerFoundationLocales() 之后注册，
// 否则 extend 会因目标 locale 不存在而静默丢失（这里顺序已对）。
registerFoundationThemes();
registerFoundationLocales();
registerHomePageLocales();
registerSettingsPageLocales();
registerTodoPageLocales();
registerGitPageLocales();
registerHistoryPageLocales();
registerSpecsPageLocales();
registerMemorysPageLocales();
registerProjectPanelLocales();

// Provider 嵌套顺序（自外向内）：
//  ① I18nProvider —— 文案是任何子组件都可能用到的最基础上下文
//  ② FoundationThemeProvider —— CssBaseline 在这里注入；与 i18n 互不依赖
//  ③ PreferencesProvider —— 行为偏好
//  ④ TerminalProvider —— 终端面板树 / 会话生命周期（全局，跨路由保活；reconcile effect 在此）
//  ⑤ ProjectProvider —— 项目登记表（全局，侧边栏项目树消费；mount 异步拉取）
//  ⑥ ProviderProvider —— 接入 Provider 登记表（全局，设置子页 + 启动选 Provider 消费）
//  ⑦ RouterProvider —— 路由切换时上面都已就绪
export const App = () => {
  return (
    <I18nProvider>
      <FoundationThemeProvider>
        <PreferencesProvider>
          <TerminalProvider>
            <ProjectProvider>
              <ProviderProvider>
                <TodoProvider>
                  <RouterProvider routes={routes}>
                    <AppLayout />
                  </RouterProvider>
                </TodoProvider>
              </ProviderProvider>
            </ProjectProvider>
          </TerminalProvider>
        </PreferencesProvider>
      </FoundationThemeProvider>
    </I18nProvider>
  );
};
