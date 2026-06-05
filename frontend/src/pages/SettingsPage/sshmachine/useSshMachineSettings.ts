import { useCallback, useState } from 'react';
import {
  NativeDialogs,
  type SshMachine,
  type SshMachineInput,
  type SshConnectivityResult,
} from '@/services';
import { useSshMachines } from '@/sshmachine';

// sshmachine 设置子页 ViewModel：消费全局 SshMachineProvider + 管理表单草稿态 +
// 编排原生对话框 + 连通性测试状态。不返回 JSX、不持翻译串（MVVM 铁律）。
//
// 脱敏契约：编辑时 password 草稿为 ''（后端不回明文），留空提交 = 保留原密码（后端 patch）。

const emptyDraft: SshMachineInput = {
  name: '',
  host: '',
  port: 22,
  user: '',
  authMethod: 'key',
  identityFile: '',
  description: '',
  defaultPath: '',
  tags: [],
  password: '',
  clearStoredPassword: false,
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

// 单机连通性测试状态：testing 进行中；result 最近一次结果（null = 未测）。
export interface TestState {
  testing: boolean;
  result: SshConnectivityResult | null;
}

export interface UseSshMachineSettingsResult {
  machines: SshMachine[];
  loading: boolean;
  formOpen: boolean;
  editingId: string | null; // null = 新增；string = 编辑该 id
  draft: SshMachineInput;
  tests: Record<string, TestState>;
  openAdd: () => void;
  openEdit: (m: SshMachine) => void;
  cancel: () => void;
  setField: <K extends keyof SshMachineInput>(key: K, value: SshMachineInput[K]) => void;
  submit: (s: SubmitStrings) => Promise<void>;
  remove: (m: SshMachine, s: RemoveStrings) => Promise<void>;
  testConnectivity: (m: SshMachine) => Promise<void>;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export const useSshMachineSettings = (): UseSshMachineSettingsResult => {
  const { machines, loading, addMachine, updateMachine, removeMachine, checkConnectivity } = useSshMachines();

  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<SshMachineInput>(emptyDraft);
  const [tests, setTests] = useState<Record<string, TestState>>({});

  const openAdd = useCallback(() => {
    setEditingId(null);
    setDraft(emptyDraft);
    setFormOpen(true);
  }, []);

  const openEdit = useCallback((m: SshMachine) => {
    setEditingId(m.id);
    // password 留空（脱敏，无法回填）；留空提交 = 保留原密码。
    setDraft({
      name: m.name,
      host: m.host,
      port: m.port,
      user: m.user,
      authMethod: m.authMethod,
      identityFile: m.identityFile,
      description: m.description,
      defaultPath: m.defaultPath,
      tags: m.tags,
      password: '',
      clearStoredPassword: false,
    });
    setFormOpen(true);
  }, []);

  const cancel = useCallback(() => setFormOpen(false), []);

  const setField = useCallback(<K extends keyof SshMachineInput>(key: K, value: SshMachineInput[K]) => {
    setDraft((prev) => ({ ...prev, [key]: value }));
  }, []);

  const submit = useCallback(
    async (s: SubmitStrings): Promise<void> => {
      try {
        if (editingId) {
          await updateMachine(editingId, draft);
        } else {
          await addMachine(draft);
        }
        setFormOpen(false);
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      }
    },
    [editingId, draft, addMachine, updateMachine],
  );

  const remove = useCallback(
    async (m: SshMachine, s: RemoveStrings): Promise<void> => {
      const ok = await NativeDialogs.confirm({
        title: s.confirmTitle,
        message: s.confirmMessage,
        okLabel: s.okLabel,
        cancelLabel: s.cancelLabel,
      });
      if (!ok) return;
      try {
        await removeMachine(m.id);
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      }
    },
    [removeMachine],
  );

  const testConnectivity = useCallback(
    async (m: SshMachine): Promise<void> => {
      setTests((prev) => ({ ...prev, [m.id]: { testing: true, result: prev[m.id]?.result ?? null } }));
      try {
        const result = await checkConnectivity(m.id);
        setTests((prev) => ({ ...prev, [m.id]: { testing: false, result } }));
      } catch (e) {
        setTests((prev) => ({
          ...prev,
          [m.id]: { testing: false, result: { reachable: false, message: messageOf(e), latencyMs: null } },
        }));
      }
    },
    [checkConnectivity],
  );

  return { machines, loading, formOpen, editingId, draft, tests, openAdd, openEdit, cancel, setField, submit, remove, testConnectivity };
};
