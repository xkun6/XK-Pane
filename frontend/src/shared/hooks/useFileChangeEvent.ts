import { useEffect, useRef } from 'react';
import { Events } from '@wailsio/runtime';
import { AppEvents } from '@/shared/events';

interface WailsEventPayload<T> {
  data: T;
  name: string;
  sender?: string;
}

// 后端 filewatch 广播的载荷（对齐 internal/events.FileChangedPayload）。
interface FileChangedPayload {
  root: string;
  dirs: string[]; // 发生增删改的目录绝对路径集
}

// useFileChangeEvent 订阅 file:changed 事件，把发生变更的目录集交给 onChanged（让文件树刷新）。
// onChanged 用 ref 持有，避免每次回调变化都重订阅。桌面专属；Web 端永不触发。
export const useFileChangeEvent = (onChanged: (dirs: string[]) => void): void => {
  const cbRef = useRef(onChanged);
  cbRef.current = onChanged;

  useEffect(() => {
    const cancel = Events.On(AppEvents.FileChanged, (event: WailsEventPayload<FileChangedPayload>) => {
      const dirs = event.data?.dirs;
      if (Array.isArray(dirs) && dirs.length > 0) cbRef.current(dirs);
    });
    return () => {
      if (typeof cancel === 'function') cancel();
    };
  }, []);
};
