import { useCallback, useState } from 'react';
import { NativeDialogs, type Provider, type ProviderInput } from '@/services';
import { useProviders } from '@/provider';

// provider 设置子页 ViewModel：消费全局 ProviderProvider + 管理表单草稿态 + 编排原生对话框。
// 不返回 JSX、不持翻译串——提示文案由 View（持 t()）传入（MVVM 铁律）。
//
// 脱敏契约：编辑时 apiKey 草稿为 ''（后端不回明文），留空提交 = 保留原密钥（后端 patch）。

const emptyDraft: ProviderInput = {
  name: '',
  providerType: 'anthropic',
  apiKey: '',
  baseUrl: '',
  region: '',
  projectId: '',
  awsProfile: '',
  configDir: '',
  isDefault: false,
};

export interface SubmitStrings {
  errorTitle: string;
}
export interface RemoveStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
  errorTitle: string;
}

export interface UseProviderSettingsResult {
  providers: Provider[];
  loading: boolean;
  formOpen: boolean;
  editingId: string | null; // null = 新增；string = 编辑该 id
  draft: ProviderInput;
  openAdd: () => void;
  openEdit: (p: Provider) => void;
  cancel: () => void;
  setField: <K extends keyof ProviderInput>(key: K, value: ProviderInput[K]) => void;
  submit: (s: SubmitStrings) => Promise<void>;
  remove: (p: Provider, s: RemoveStrings) => Promise<void>;
  setDefault: (p: Provider, s: SubmitStrings) => Promise<void>;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export const useProviderSettings = (): UseProviderSettingsResult => {
  const { providers, loading, addProvider, updateProvider, removeProvider, setDefaultProvider } = useProviders();

  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<ProviderInput>(emptyDraft);

  const openAdd = useCallback(() => {
    setEditingId(null);
    setDraft(emptyDraft);
    setFormOpen(true);
  }, []);

  const openEdit = useCallback((p: Provider) => {
    setEditingId(p.id);
    // apiKey 留空（脱敏，无法回填）；留空提交 = 保留原密钥。
    setDraft({
      name: p.name,
      providerType: p.providerType,
      apiKey: '',
      baseUrl: p.baseUrl,
      region: p.region,
      projectId: p.projectId,
      awsProfile: p.awsProfile,
      configDir: p.configDir,
      isDefault: p.isDefault,
    });
    setFormOpen(true);
  }, []);

  const cancel = useCallback(() => setFormOpen(false), []);

  const setField = useCallback(<K extends keyof ProviderInput>(key: K, value: ProviderInput[K]) => {
    setDraft((prev) => ({ ...prev, [key]: value }));
  }, []);

  const submit = useCallback(
    async (s: SubmitStrings): Promise<void> => {
      try {
        if (editingId) {
          await updateProvider(editingId, draft);
        } else {
          await addProvider(draft);
        }
        setFormOpen(false);
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      }
    },
    [editingId, draft, addProvider, updateProvider],
  );

  const remove = useCallback(
    async (p: Provider, s: RemoveStrings): Promise<void> => {
      const ok = await NativeDialogs.confirm({
        title: s.confirmTitle,
        message: s.confirmMessage,
        okLabel: s.okLabel,
        cancelLabel: s.cancelLabel,
      });
      if (!ok) return;
      try {
        await removeProvider(p.id);
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      }
    },
    [removeProvider],
  );

  const setDefault = useCallback(
    async (p: Provider, s: SubmitStrings): Promise<void> => {
      try {
        await setDefaultProvider(p.id);
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      }
    },
    [setDefaultProvider],
  );

  return { providers, loading, formOpen, editingId, draft, openAdd, openEdit, cancel, setField, submit, remove, setDefault };
};
