import { useCallback, useEffect, useState } from 'react';
import { AgentCfgService, NativeDialogs, type AgentSkill } from '@/services';

// Skills 子页面的 ViewModel：Skill SSOT 的列表 + 逐 CLI 开关 + 从文件夹 / 现有 CLI 导入 + 删除。
// 不返回 JSX、不持翻译串——提示文案由 View（持 t()）传入（MVVM 铁律）。

export interface ErrorStrings {
  errorTitle: string;
}
export interface RemoveStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
  errorTitle: string;
}
export interface ImportStrings {
  errorTitle: string;
  doneTitle: string;
  doneMessage: (count: number) => string;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export interface UseSkillsResult {
  skills: AgentSkill[];
  loading: boolean;
  refresh: () => Promise<void>;
  importFromFolder: (srcPath: string, strings: ErrorStrings) => Promise<void>;
  importFromApps: (strings: ImportStrings) => Promise<void>;
  toggleEnabled: (
    s: AgentSkill,
    key: 'enabledClaude' | 'enabledCodex',
    value: boolean,
    strings: ErrorStrings,
  ) => Promise<void>;
  remove: (s: AgentSkill, strings: RemoveStrings) => Promise<void>;
}

export const useSkills = (): UseSkillsResult => {
  const [skills, setSkills] = useState<AgentSkill[]>([]);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async (): Promise<void> => {
    setLoading(true);
    try {
      setSkills(await AgentCfgService.listSkills());
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const importFromFolder = useCallback(
    async (srcPath: string, strings: ErrorStrings): Promise<void> => {
      try {
        await AgentCfgService.importSkillDir(srcPath);
        await refresh();
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
      }
    },
    [refresh],
  );

  const importFromApps = useCallback(
    async (strings: ImportStrings): Promise<void> => {
      try {
        const count = await AgentCfgService.importSkillsFromApps();
        await refresh();
        await NativeDialogs.info(strings.doneTitle, strings.doneMessage(count));
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
      }
    },
    [refresh],
  );

  const toggleEnabled = useCallback(
    async (
      s: AgentSkill,
      key: 'enabledClaude' | 'enabledCodex',
      value: boolean,
      strings: ErrorStrings,
    ): Promise<void> => {
      try {
        await AgentCfgService.setSkillEnabled(
          s.id,
          key === 'enabledClaude' ? value : s.enabledClaude,
          key === 'enabledCodex' ? value : s.enabledCodex,
        );
        await refresh();
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
        await refresh();
      }
    },
    [refresh],
  );

  const remove = useCallback(
    async (s: AgentSkill, strings: RemoveStrings): Promise<void> => {
      const ok = await NativeDialogs.confirm({
        title: strings.confirmTitle,
        message: strings.confirmMessage,
        okLabel: strings.okLabel,
        cancelLabel: strings.cancelLabel,
      });
      if (!ok) return;
      try {
        await AgentCfgService.removeSkill(s.id);
        await refresh();
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
      }
    },
    [refresh],
  );

  return { skills, loading, refresh, importFromFolder, importFromApps, toggleEnabled, remove };
};
