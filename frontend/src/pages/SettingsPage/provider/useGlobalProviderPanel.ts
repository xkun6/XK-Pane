import { useCallback, useEffect, useState } from 'react';
import {
  AgentCfgService,
  NativeDialogs,
  type AgentAppType,
  type AgentProvider,
  type AgentProviderInput,
} from '@/services';
import { optionalJsonObjectError, requiredJsonObjectError, type JsonFieldError } from './agentProviderJson';

// GlobalProviderPanel 的 ViewModel：cc-switch 式全局配置 Provider 的列表 + 增删改查 + 切换。
// 一份实例对应一个 appType（claude | codex），View 传参决定表单里编辑哪套配置文本。
// 不返回 JSX、不持翻译串——提示文案由 View（持 t()）传入（MVVM 铁律）。

export type ClaudeTemplateId = 'blank' | 'apiKey' | 'thirdParty';

const CLAUDE_TEMPLATES: Record<ClaudeTemplateId, string> = {
  blank: '{}',
  apiKey: JSON.stringify({ env: { ANTHROPIC_API_KEY: '' } }, null, 2),
  thirdParty: JSON.stringify({ env: { ANTHROPIC_AUTH_TOKEN: '', ANTHROPIC_BASE_URL: '' } }, null, 2),
};

export interface GlobalProviderDraft {
  name: string;
  notes: string;
  claudeJson: string;
  codexAuthJson: string;
  codexConfigToml: string;
}

const emptyDraft: GlobalProviderDraft = {
  name: '',
  notes: '',
  claudeJson: '{}',
  codexAuthJson: '',
  codexConfigToml: '',
};

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
export interface ConfirmStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

// 把已有 Provider 的 settingsJson 拆回草稿态（编辑回填）。claude 侧做一次 pretty-print
// 方便编辑；codex 侧拆 auth/config 两块。任一侧解析失败都退回空白草稿而不是抛错——脏数据
// 不该拦住用户打开编辑框。
function draftFromProvider(appType: AgentAppType, p: AgentProvider): GlobalProviderDraft {
  if (appType === 'claude') {
    let pretty = p.settingsJson;
    try {
      pretty = JSON.stringify(JSON.parse(p.settingsJson), null, 2);
    } catch {
      // 保留原始文本，交给用户自己修
    }
    return { name: p.name, notes: p.notes, claudeJson: pretty, codexAuthJson: '', codexConfigToml: '' };
  }
  let authText = '';
  let configText = '';
  try {
    const parsed = JSON.parse(p.settingsJson) as { auth?: unknown; config?: unknown };
    authText = parsed.auth != null ? JSON.stringify(parsed.auth, null, 2) : '';
    configText = typeof parsed.config === 'string' ? parsed.config : '';
  } catch {
    // 保留空白，交给用户自己修
  }
  return { name: p.name, notes: p.notes, claudeJson: '{}', codexAuthJson: authText, codexConfigToml: configText };
}

export interface UseGlobalProviderPanelResult {
  providers: AgentProvider[];
  loading: boolean;
  formOpen: boolean;
  editingId: string | null;
  editingIsCurrent: boolean;
  draft: GlobalProviderDraft;
  claudeJsonError: JsonFieldError;
  codexAuthJsonError: JsonFieldError;
  openAdd: () => void;
  openEdit: (p: AgentProvider) => void;
  cancel: () => void;
  setField: <K extends keyof GlobalProviderDraft>(key: K, value: GlobalProviderDraft[K]) => void;
  applyTemplate: (template: ClaudeTemplateId, strings: ConfirmStrings) => Promise<void>;
  submit: (strings: ErrorStrings) => Promise<void>;
  remove: (p: AgentProvider, strings: RemoveStrings) => Promise<void>;
  switchTo: (p: AgentProvider, strings: ErrorStrings) => Promise<void>;
  importCurrent: (strings: ErrorStrings) => Promise<void>;
}

export const useGlobalProviderPanel = (appType: AgentAppType): UseGlobalProviderPanelResult => {
  const [providers, setProviders] = useState<AgentProvider[]>([]);
  const [loading, setLoading] = useState(true);
  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingIsCurrent, setEditingIsCurrent] = useState(false);
  const [draft, setDraft] = useState<GlobalProviderDraft>(emptyDraft);

  const refresh = useCallback(async (): Promise<void> => {
    setLoading(true);
    try {
      setProviders(await AgentCfgService.list(appType));
    } finally {
      setLoading(false);
    }
  }, [appType]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // live 文件外部改动 → 后端已同步 current Provider，刷新列表（编辑中的草稿不碰）
  useEffect(() => {
    return AgentCfgService.onLiveChanged((payload) => {
      if (payload.kind === 'provider' && payload.appType === appType) {
        void refresh();
      }
    });
  }, [appType, refresh]);

  const openAdd = useCallback(() => {
    setEditingId(null);
    setEditingIsCurrent(false);
    setDraft(emptyDraft);
    setFormOpen(true);
  }, []);

  const openEdit = useCallback(
    (p: AgentProvider) => {
      setEditingId(p.id);
      setEditingIsCurrent(p.isCurrent);
      setDraft(draftFromProvider(appType, p));
      setFormOpen(true);
    },
    [appType],
  );

  const cancel = useCallback(() => setFormOpen(false), []);

  const setField = useCallback(<K extends keyof GlobalProviderDraft>(key: K, value: GlobalProviderDraft[K]) => {
    setDraft((prev) => ({ ...prev, [key]: value }));
  }, []);

  const applyTemplate = useCallback(
    async (template: ClaudeTemplateId, strings: ConfirmStrings): Promise<void> => {
      const next = CLAUDE_TEMPLATES[template];
      const current = draft.claudeJson.trim();
      if (current !== '' && current !== '{}') {
        const ok = await NativeDialogs.confirm({
          title: strings.confirmTitle,
          message: strings.confirmMessage,
          okLabel: strings.okLabel,
          cancelLabel: strings.cancelLabel,
        });
        if (!ok) return;
      }
      setDraft((prev) => ({ ...prev, claudeJson: next }));
    },
    [draft.claudeJson],
  );

  const submit = useCallback(
    async (strings: ErrorStrings): Promise<void> => {
      const settingsJson =
        appType === 'claude'
          ? draft.claudeJson.trim()
          : JSON.stringify({
              auth: draft.codexAuthJson.trim() === '' ? null : (JSON.parse(draft.codexAuthJson) as unknown),
              config: draft.codexConfigToml,
            });
      const input: AgentProviderInput = {
        appType,
        name: draft.name,
        notes: draft.notes,
        settingsJson,
      };
      try {
        if (editingId) {
          await AgentCfgService.update(editingId, input);
        } else {
          await AgentCfgService.add(input);
        }
        setFormOpen(false);
        await refresh();
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
      }
    },
    [appType, draft, editingId, refresh],
  );

  const remove = useCallback(
    async (p: AgentProvider, strings: RemoveStrings): Promise<void> => {
      const ok = await NativeDialogs.confirm({
        title: strings.confirmTitle,
        message: strings.confirmMessage,
        okLabel: strings.okLabel,
        cancelLabel: strings.cancelLabel,
      });
      if (!ok) return;
      try {
        await AgentCfgService.remove(p.id);
        await refresh();
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
      }
    },
    [refresh],
  );

  const switchTo = useCallback(
    async (p: AgentProvider, strings: ErrorStrings): Promise<void> => {
      try {
        await AgentCfgService.switchTo(p.id);
        await refresh();
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
      }
    },
    [refresh],
  );

  const importCurrent = useCallback(
    async (strings: ErrorStrings): Promise<void> => {
      try {
        await AgentCfgService.importCurrent(appType);
        await refresh();
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
      }
    },
    [appType, refresh],
  );

  return {
    providers,
    loading,
    formOpen,
    editingId,
    editingIsCurrent,
    draft,
    claudeJsonError: requiredJsonObjectError(draft.claudeJson),
    codexAuthJsonError: optionalJsonObjectError(draft.codexAuthJson),
    openAdd,
    openEdit,
    cancel,
    setField,
    applyTemplate,
    submit,
    remove,
    switchTo,
    importCurrent,
  };
};
