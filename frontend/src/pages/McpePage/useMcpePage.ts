import { useCallback, useEffect, useState } from 'react';
import { NativeDialogs } from '@/services';
import { isDesktop } from '@/services/transport';
import { useMcpe } from './McpeContext';

export function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

// 容器 ViewModel：挂载拉包 + 新建/刷新。文案（标题）由 View 经参数传入（VM 不持翻译串）。
export const useMcpePage = () => {
  const { packages, activePackageId, loadPackages, createPackage, setActivePackage } = useMcpe();
  const [busy, setBusy] = useState(false);

  // 挂载拉包（仅桌面；viewer 上 service 会 reject）。
  useEffect(() => {
    if (!isDesktop) return;
    void loadPackages().catch(() => {});
  }, [loadPackages]);

  const refresh = useCallback(
    async (errorTitle: string) => {
      try {
        await loadPackages();
      } catch (e) {
        await NativeDialogs.error(errorTitle, messageOf(e));
      }
    },
    [loadPackages],
  );

  // 新建：选目录 → 创建。dialogTitle 用于选目录对话框标题，errorTitle 用于失败提示。
  const create = useCallback(
    async (dialogTitle: string, errorTitle: string) => {
      const dir = await NativeDialogs.openDirectory({ title: dialogTitle });
      if (!dir) return;
      setBusy(true);
      try {
        await createPackage(dir);
      } catch (e) {
        await NativeDialogs.error(errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [createPackage],
  );

  return { packages, activePackageId, busy, refresh, create, setActivePackage };
};
