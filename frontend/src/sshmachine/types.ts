import type { SshMachine, SshMachineInput, SshConnectivityResult } from '@/services';

// SshMachineContextValue：SSH 机器全局状态 + 动作（纯 Context，范式同 ProviderProvider）。
// 动作约定：调后端 → 刷新列表（与后端插入序保持一致，不乐观拼接）。错误**向上抛**，由
// View 用 NativeDialogs 提示。明文 password 仅在 input 里上行；列表是脱敏 SshMachine。
// 被 sshmachine 设置子页与（后续）启动选机器 UI 共同消费。
export interface SshMachineContextValue {
  machines: SshMachine[];
  loading: boolean; // 仅首屏拉取期 true；mutation 刷新不切
  refresh: () => Promise<void>;
  addMachine: (input: SshMachineInput) => Promise<SshMachine>;
  updateMachine: (id: string, input: SshMachineInput) => Promise<void>;
  removeMachine: (id: string) => Promise<void>;
  checkConnectivity: (id: string) => Promise<SshConnectivityResult>;
}
