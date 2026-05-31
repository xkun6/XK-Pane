import { useEffect, useRef } from 'react';
import { Events } from '@wailsio/runtime';
import { AppEvents } from '@/shared/events';

interface WailsEventPayload<T> {
  data: T;
  name: string;
  sender?: string;
}

// 后端 lhwatcher 自动捕获保存版本后广播的载荷（对齐 internal/events.LocalHistoryChangedPayload）。
interface LocalHistoryChangedPayload {
  projectPath: string;
  filePaths: string[];
  versionCount: number;
  timestamp: number;
}

// useLocalHistoryChangeEvent 订阅 localhistory:changed 事件，仅当事件的 projectPath 与传入的
// projectPath 匹配时触发 onChanged（让历史页 live 刷新当前列表）。projectPath 为空时不触发。
// onChanged 用 ref 持有，避免每次回调变化都重订阅。
export const useLocalHistoryChangeEvent = (projectPath: string, onChanged: () => void): void => {
  const cbRef = useRef(onChanged);
  cbRef.current = onChanged;

  useEffect(() => {
    if (!projectPath) return;
    const cancel = Events.On(AppEvents.LocalHistoryChanged, (event: WailsEventPayload<LocalHistoryChangedPayload>) => {
      if (event.data?.projectPath === projectPath) {
        cbRef.current();
      }
    });
    return () => {
      if (typeof cancel === 'function') cancel();
    };
  }, [projectPath]);
};
