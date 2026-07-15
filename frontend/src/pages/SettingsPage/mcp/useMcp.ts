import { useCallback, useEffect, useState } from 'react';
import { AgentCfgService, NativeDialogs, type AgentMcpServer, type AgentMcpServerInput } from '@/services';
import { normalizeServerJson } from './mcpServerJson';

// Mcp 子页面的 ViewModel：MCP server SSOT 的列表 + 增删改 + 逐 CLI 开关 + 从 live 导入。
// 不返回 JSX、不持翻译串——提示文案由 View（持 t()）传入（MVVM 铁律）。

const NAME_PATTERN = /^[A-Za-z0-9._-]+$/;

export interface McpDraft {
  name: string;
  serverJson: string;
  enabledClaude: boolean;
  enabledCodex: boolean;
}

const emptyDraft: McpDraft = { name: '', serverJson: '', enabledClaude: false, enabledCodex: false };

export type JsonFieldError = '' | 'empty' | 'invalid' | 'multiple';

function draftFromServer(s: AgentMcpServer): McpDraft {
  let pretty = s.serverJson;
  try {
    pretty = JSON.stringify(JSON.parse(s.serverJson), null, 2);
  } catch {
    // 保留原始文本，交给用户自己修
  }
  return { name: s.name, serverJson: pretty, enabledClaude: s.enabledClaude, enabledCodex: s.enabledCodex };
}

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

export interface UseMcpResult {
  servers: AgentMcpServer[];
  loading: boolean;
  formOpen: boolean;
  editingId: string | null;
  draft: McpDraft;
  nameValid: boolean;
  jsonError: JsonFieldError;
  openAdd: () => void;
  openEdit: (s: AgentMcpServer) => void;
  cancel: () => void;
  setField: <K extends keyof McpDraft>(key: K, value: McpDraft[K]) => void;
  formatJson: () => void;
  submit: (strings: ErrorStrings) => Promise<void>;
  remove: (s: AgentMcpServer, strings: RemoveStrings) => Promise<void>;
  toggleEnabled: (
    s: AgentMcpServer,
    key: 'enabledClaude' | 'enabledCodex',
    value: boolean,
    strings: ErrorStrings,
  ) => Promise<void>;
  importFromLive: (strings: ImportStrings) => Promise<void>;
}

export const useMcp = (): UseMcpResult => {
  const [servers, setServers] = useState<AgentMcpServer[]>([]);
  const [loading, setLoading] = useState(true);
  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<McpDraft>(emptyDraft);

  const refresh = useCallback(async (): Promise<void> => {
    setLoading(true);
    try {
      setServers(await AgentCfgService.listMcp());
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const openAdd = useCallback(() => {
    setEditingId(null);
    setDraft(emptyDraft);
    setFormOpen(true);
  }, []);

  const openEdit = useCallback((s: AgentMcpServer) => {
    setEditingId(s.id);
    setDraft(draftFromServer(s));
    setFormOpen(true);
  }, []);

  const cancel = useCallback(() => setFormOpen(false), []);

  const setField = useCallback(<K extends keyof McpDraft>(key: K, value: McpDraft[K]) => {
    if (key === 'serverJson') {
      const result = normalizeServerJson(value as McpDraft['serverJson']);
      if (result.kind === 'unwrapped') {
        // 粘贴包裹层：编辑器内容保留原样，名称字段自动回填解出的 server 名
        setDraft((prev) => ({ ...prev, serverJson: value as McpDraft['serverJson'], name: result.name }));
        return;
      }
    }
    setDraft((prev) => ({ ...prev, [key]: value }));
  }, []);

  const formatJson = useCallback(() => {
    setDraft((prev) => {
      const result = normalizeServerJson(prev.serverJson);
      if (result.kind === 'bare') return { ...prev, serverJson: result.json };
      if (result.kind === 'unwrapped') return { ...prev, serverJson: result.json, name: result.name };
      return prev;
    });
  }, []);

  const submit = useCallback(
    async (strings: ErrorStrings): Promise<void> => {
      const result = normalizeServerJson(draft.serverJson);
      if (result.kind !== 'bare' && result.kind !== 'unwrapped') return;
      const input: AgentMcpServerInput = {
        name: draft.name,
        serverJson: result.json,
        enabledClaude: draft.enabledClaude,
        enabledCodex: draft.enabledCodex,
      };
      try {
        if (editingId) {
          await AgentCfgService.updateMcp(editingId, input);
        } else {
          await AgentCfgService.addMcp(input);
        }
        setFormOpen(false);
        await refresh();
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
      }
    },
    [draft, editingId, refresh],
  );

  const remove = useCallback(
    async (s: AgentMcpServer, strings: RemoveStrings): Promise<void> => {
      const ok = await NativeDialogs.confirm({
        title: strings.confirmTitle,
        message: strings.confirmMessage,
        okLabel: strings.okLabel,
        cancelLabel: strings.cancelLabel,
      });
      if (!ok) return;
      try {
        await AgentCfgService.removeMcp(s.id);
        await refresh();
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
      }
    },
    [refresh],
  );

  const toggleEnabled = useCallback(
    async (
      s: AgentMcpServer,
      key: 'enabledClaude' | 'enabledCodex',
      value: boolean,
      strings: ErrorStrings,
    ): Promise<void> => {
      try {
        await AgentCfgService.updateMcp(s.id, {
          name: s.name,
          serverJson: s.serverJson,
          enabledClaude: key === 'enabledClaude' ? value : s.enabledClaude,
          enabledCodex: key === 'enabledCodex' ? value : s.enabledCodex,
        });
        await refresh();
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
        await refresh();
      }
    },
    [refresh],
  );

  const importFromLive = useCallback(
    async (strings: ImportStrings): Promise<void> => {
      try {
        const count = await AgentCfgService.importMcpFromLive();
        await refresh();
        await NativeDialogs.info(strings.doneTitle, strings.doneMessage(count));
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
      }
    },
    [refresh],
  );

  const jsonResult = normalizeServerJson(draft.serverJson);
  const jsonError: JsonFieldError =
    jsonResult.kind === 'bare' || jsonResult.kind === 'unwrapped' ? '' : jsonResult.kind;

  return {
    servers,
    loading,
    formOpen,
    editingId,
    draft,
    nameValid: NAME_PATTERN.test(draft.name),
    jsonError,
    openAdd,
    openEdit,
    cancel,
    setField,
    formatJson,
    submit,
    remove,
    toggleEnabled,
    importFromLive,
  };
};
