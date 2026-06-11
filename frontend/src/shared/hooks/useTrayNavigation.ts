import { useEffect } from 'react';
import { Events } from '@wailsio/runtime';
import { AppEvents } from '@/shared/events';

// 监听后端托盘「设置」菜单事件（NavigateSettings），触发回调（通常 navigate('settings')）。
// shared hook 直接订阅 Wails 应用级总线（事件无 payload，忽略 event 参数）。
export const useTrayNavigation = (onOpenSettings: () => void): void => {
  useEffect(() => {
    const cancel = Events.On(AppEvents.NavigateSettings, () => onOpenSettings());
    return () => {
      if (typeof cancel === 'function') cancel();
    };
  }, [onOpenSettings]);
};
