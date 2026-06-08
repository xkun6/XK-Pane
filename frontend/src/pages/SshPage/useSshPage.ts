import { useCallback, useEffect, useRef, useState } from 'react';
import {
  NativeDialogs,
  type CliTool,
  type SshMachine,
  type SshMachineInput,
  type SshConnectivityResult,
} from '@/services';
import type { SSHSpec } from '@/services/terminal/TerminalService';
import { useSshMachines } from '@/sshmachine';
import { useCliTools } from '@/shared/hooks/useCliTools';
import { useTerminal } from '@/terminal';
import { useRouter } from '@/router';

// SSH 连接中心页 ViewModel：消费全局 SshMachineProvider（机器 CRUD）+ useCliTools（连接选 CLI）
// + useTerminal（启动远程会话）+ useRouter（连接后跳终端页）。不返回 JSX、不持翻译串（MVVM 铁律）。
//
// 连接合成：从脱敏机器 DTO 拷连接字段 → runtimeKind='ssh' + ssh: SSHSpec，复用 launchProject 整条
// PTY 管线（本地 ssh 客户端拉起远端会话）。密码不经前端——后端按 machineId 解密自动应答。
// 脱敏契约：编辑时 password 草稿为 ''（后端不回明文），留空提交 = 保留原密码。

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

export interface UseSshPageResult {
  machines: SshMachine[];
  loading: boolean;
  tools: CliTool[];
  toolsLoading: boolean;
  tests: Record<string, TestState>;
  checkingAll: boolean;
  formOpen: boolean;
  editingId: string | null; // null = 新增；string = 编辑该 id
  draft: SshMachineInput;
  openAdd: () => void;
  openEdit: (m: SshMachine) => void;
  cancel: () => void;
  setField: <K extends keyof SshMachineInput>(key: K, value: SshMachineInput[K]) => void;
  submit: (s: SubmitStrings) => Promise<void>;
  remove: (m: SshMachine, s: RemoveStrings) => Promise<void>;
  testConnectivity: (m: SshMachine) => Promise<void>;
  testAll: () => Promise<void>;
  connect: (m: SshMachine, cliTool: string) => void;
  copyConnection: (m: SshMachine) => void;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

// 连接目标串（user@host[:port]）：端口 22 省略。
function connectionTarget(m: SshMachine): string {
  const host = m.user ? `${m.user}@${m.host}` : m.host;
  return m.port && m.port !== 22 ? `${host}:${m.port}` : host;
}

export const useSshPage = (): UseSshPageResult => {
  const { machines, loading, addMachine, updateMachine, removeMachine, checkConnectivity } =
    useSshMachines();
  const { tools, loading: toolsLoading } = useCliTools();
  const { launchProject } = useTerminal();
  const { navigate } = useRouter();

  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<SshMachineInput>(emptyDraft);
  const [tests, setTests] = useState<Record<string, TestState>>({});
  const [checkingAll, setCheckingAll] = useState(false);
  const abortRef = useRef(false);
  const autoCheckedRef = useRef(false);

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

  const setField = useCallback(
    <K extends keyof SshMachineInput>(key: K, value: SshMachineInput[K]) => {
      setDraft((prev) => ({ ...prev, [key]: value }));
    },
    [],
  );

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

  // 串行逐机检测（避免并发 SSH 爆发）；checkingAll 期间禁重入。
  const testAll = useCallback(async (): Promise<void> => {
    if (checkingAll || machines.length === 0) return;
    setCheckingAll(true);
    abortRef.current = false;
    for (const m of machines) {
      if (abortRef.current) break;
      await testConnectivity(m);
    }
    setCheckingAll(false);
  }, [checkingAll, machines, testConnectivity]);

  // 首次机器列表加载后自动测一次在线状态（仅一次）。
  useEffect(() => {
    if (machines.length > 0 && !autoCheckedRef.current && !checkingAll) {
      autoCheckedRef.current = true;
      const timer = setTimeout(() => void testAll(), 400);
      return () => clearTimeout(timer);
    }
    return undefined;
  }, [machines.length, checkingAll, testAll]);

  // 卸载时中止串行检测。
  useEffect(
    () => () => {
      abortRef.current = true;
    },
    [],
  );

  // 连接：合成 SSHSpec + 合成项目键 → launchProject（runtimeKind='ssh'）→ 跳终端页。
  // cliTool='none' = 远端裸 shell；其余 = 远端起对应 CLI。
  const connect = useCallback(
    (m: SshMachine, cliTool: string): void => {
      const remotePath = m.defaultPath || '~';
      const ssh: SSHSpec = {
        machineId: m.id,
        host: m.host,
        port: m.port,
        user: m.user,
        remotePath,
        identityFile: m.identityFile,
        authMethod: m.authMethod,
      };
      const userPart = m.user ? `${m.user}@` : '';
      const portPart = m.port && m.port !== 22 ? `:${m.port}` : '';
      // 合成项目键（仅作标识 / 启动历史软关联，后端不当本地 cwd 用）。
      const projectPath = `ssh://${userPart}${m.host}${portPart}/${remotePath}`;
      launchProject({
        projectId: `ssh:${m.id}`,
        projectName: m.name,
        projectPath,
        cliTool,
        runtimeKind: 'ssh',
        ssh,
      });
      navigate('terminal');
    },
    [launchProject, navigate],
  );

  const copyConnection = useCallback((m: SshMachine): void => {
    void navigator.clipboard?.writeText(connectionTarget(m)).catch(() => {});
  }, []);

  return {
    machines,
    loading,
    tools,
    toolsLoading,
    tests,
    checkingAll,
    formOpen,
    editingId,
    draft,
    openAdd,
    openEdit,
    cancel,
    setField,
    submit,
    remove,
    testConnectivity,
    testAll,
    connect,
    copyConnection,
  };
};
