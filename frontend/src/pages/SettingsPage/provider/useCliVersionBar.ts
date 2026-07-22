import { useCallback, useEffect, useState } from 'react';
import { CliUpdateService, NativeDialogs, type AgentAppType, type CliToolStatus } from '@/services';

// CliVersionBar 的 ViewModel：claude / codex CLI 版本检测 + 一键升级。
// 不返回 JSX、不持翻译串——对话框文案由 View（持 t()）传入（MVVM 铁律，照抄
// useGlobalProviderPanel 的 ErrorStrings 模式）。检测是背景行为，失败只降级、不弹窗。

export interface UpgradeStrings {
  doneTitle: string;
  doneMessage: (version: string) => string;
  warnTitle: string;
  failTitle: string;
}

export interface InstallStrings {
  doneTitle: string;
  doneMessage: (version: string) => string;
  warnTitle: string;
  failTitle: string;
}

export interface UseCliVersionBarResult {
  status: CliToolStatus | null; // null = 尚未取到
  checking: boolean;
  upgrading: boolean;
  installing: boolean;
  refresh: () => Promise<void>;
  upgrade: (strings: UpgradeStrings) => Promise<void>;
  install: (strings: InstallStrings) => Promise<void>;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export const useCliVersionBar = (appType: AgentAppType): UseCliVersionBarResult => {
  const [status, setStatus] = useState<CliToolStatus | null>(null);
  const [checking, setChecking] = useState(false);
  const [upgrading, setUpgrading] = useState(false);
  const [installing, setInstalling] = useState(false);

  const refresh = useCallback(async (): Promise<void> => {
    setChecking(true);
    try {
      setStatus(await CliUpdateService.check(appType));
    } catch {
      setStatus(null);
    } finally {
      setChecking(false);
    }
  }, [appType]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const upgrade = useCallback(
    async (strings: UpgradeStrings): Promise<void> => {
      setUpgrading(true);
      try {
        const result = await CliUpdateService.upgrade(appType);
        // 先复位按钮态、后台触发复检，再弹结果——状态复位不依赖弹窗 promise
        //（曾因 Windows 上 info/warning/error 弹窗 promise 不 resolve 卡死「升级中」形态）。
        setUpgrading(false);
        void refresh();
        if (result.ok && result.message === '') {
          await NativeDialogs.info(strings.doneTitle, strings.doneMessage(result.newVersion));
        } else if (result.ok) {
          await NativeDialogs.warning(strings.warnTitle, result.message);
        } else {
          await NativeDialogs.error(strings.failTitle, result.message);
        }
      } catch (e) {
        setUpgrading(false);
        void refresh();
        await NativeDialogs.error(strings.failTitle, messageOf(e));
      } finally {
        setUpgrading(false);
      }
    },
    [appType, refresh],
  );

  const install = useCallback(
    async (strings: InstallStrings): Promise<void> => {
      setInstalling(true);
      try {
        const result = await CliUpdateService.install(appType);
        // 与 upgrade 同款：先复位按钮态、后台复检，再弹结果（弹窗 promise 不参与状态复位）。
        setInstalling(false);
        void refresh();
        if (result.ok && result.message === '') {
          await NativeDialogs.info(strings.doneTitle, strings.doneMessage(result.newVersion));
        } else if (result.ok) {
          await NativeDialogs.warning(strings.warnTitle, result.message);
        } else {
          await NativeDialogs.error(strings.failTitle, result.message);
        }
      } catch (e) {
        setInstalling(false);
        void refresh();
        await NativeDialogs.error(strings.failTitle, messageOf(e));
      } finally {
        setInstalling(false);
      }
    },
    [appType, refresh],
  );

  return { status, checking, upgrading, installing, refresh, upgrade, install };
};
