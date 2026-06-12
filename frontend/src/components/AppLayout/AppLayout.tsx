import { useCallback, useEffect } from 'react';
import { Box, useMediaQuery, useTheme } from '@mui/material';
import { TitleBar } from '@/components/TitleBar';
import { Sidebar } from '@/components/Sidebar';
import { ScreenshotBridge } from '@/components/ScreenshotBridge';
import { RouterOutlet, useRouter } from '@/router';
import { RightPanelDrawer } from '@/rightpanel';
import { lifecycleRole } from '@/services/transport';
import { useTrayNavigation } from '@/shared/hooks/useTrayNavigation';
import { appLayoutStyles } from './AppLayout.styles';

export interface AppLayoutProps {
  // 可选：覆盖标题栏标题；不传则 TitleBar 使用 i18n 默认值（app.title）
  title?: string;
}

// 软件基座外壳：
//  ┌─────────────── TitleBar ───────────────┐
//  │ Sidebar │  RouterOutlet (当前页面)     │
//  └─────────┴──────────────────────────────┘
// AppLayout 不接收 children —— 主内容由 RouterOutlet 从路由表渲染。
// Web（viewer）窄屏：屏蔽左侧导航栏并锁定终端页（用户诉求「仅保留终端界面」）；工作区选择仍由
// TerminalPage 的移动端弹出按钮（folder → Drawer 工作区树）承担，不依赖导航栏。桌面/宽屏不受影响。
export const AppLayout = ({ title }: AppLayoutProps) => {
  const theme = useTheme();
  const styles = appLayoutStyles(theme);
  const isNarrow = useMediaQuery(theme.breakpoints.down('sm'));
  const { navigate, active } = useRouter();
  const hideChrome = lifecycleRole === 'viewer' && isNarrow;

  useEffect(() => {
    if (hideChrome && active.id !== 'terminal') navigate('terminal');
  }, [hideChrome, active.id, navigate]);

  // 托盘「设置」菜单 → 跳转设置页（窗口已被托盘 showMain 唤回，这里仅切路由）。
  useTrayNavigation(useCallback(() => navigate('settings'), [navigate]));

  return (
    <>
      <TitleBar {...(title !== undefined ? { title } : {})} />
      <Box sx={styles.root}>
        {!hideChrome && <Sidebar />}
        <Box component="main" sx={styles.main}>
          <RouterOutlet />
        </Box>
        {/* 右侧抽屉（Git / 本地历史）：未开启时渲染 null，不占布局；与主区并列在 flex row 内推开内容 */}
        {!hideChrome && <RightPanelDrawer />}
      </Box>
      {/* 截图闭环桥（常驻无界面）：screenshot:saved → 路径写活跃终端 + snackbar */}
      <ScreenshotBridge />
    </>
  );
};
