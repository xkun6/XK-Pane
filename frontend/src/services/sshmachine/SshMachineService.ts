// SshMachineService：SSH 机器（ssh_machines 表）的前端封装。
//
// 后端按持久化铁律落 SQLite 且 password 经 cryptox 加密；**明文 password 绝不跨 IPC 回前端**——
// List/Get 返回脱敏视图（仅 hasStoredPassword 标志）。新增/编辑时前端传明文 password；
// 编辑留空 = 保留原密码（后端 patch），clearStoredPassword=true 显式清空。范式同 ProviderService。
import { Service as Binding } from '@bindings/xkpane/internal/services/sshmachine';
import type {
  MachineDTO as RawMachineDTO,
  ConnectivityResult as RawConnectivityResult,
} from '@bindings/xkpane/internal/services/sshmachine/models';

export type SshAuthMethod = 'password' | 'key' | 'agent';

// SshMachine：脱敏视图（前端可见）。无明文 password，仅 hasStoredPassword。
export interface SshMachine {
  id: string;
  name: string;
  host: string;
  port: number;
  user: string;
  authMethod: string; // password | key | agent
  identityFile: string;
  description: string;
  defaultPath: string;
  tags: string[];
  hasStoredPassword: boolean;
}

// SshMachineInput：新增/编辑提交字段。password 明文（编辑留空 = 保留原密码）；
// clearStoredPassword=true 显式清空已存密码。
export interface SshMachineInput {
  name: string;
  host: string;
  port: number;
  user: string;
  authMethod: string;
  identityFile: string;
  description: string;
  defaultPath: string;
  tags: string[];
  password: string;
  clearStoredPassword: boolean;
}

export interface SshConnectivityResult {
  reachable: boolean;
  message: string;
  latencyMs: number | null;
}

function toMachine(m: RawMachineDTO): SshMachine {
  return {
    id: m.id ?? '',
    name: m.name ?? '',
    host: m.host ?? '',
    port: m.port ?? 22,
    user: m.user ?? '',
    authMethod: m.authMethod ?? 'key',
    identityFile: m.identityFile ?? '',
    description: m.description ?? '',
    defaultPath: m.defaultPath ?? '',
    tags: m.tags ?? [],
    hasStoredPassword: m.hasStoredPassword ?? false,
  };
}

// 显式构造提交对象（不直接传接口实例，避免与生成 model 结构耦合）。
function toRawInput(input: SshMachineInput) {
  return {
    name: input.name,
    host: input.host,
    port: input.port,
    user: input.user,
    authMethod: input.authMethod,
    identityFile: input.identityFile,
    description: input.description,
    defaultPath: input.defaultPath,
    tags: input.tags,
    password: input.password,
    clearStoredPassword: input.clearStoredPassword,
  };
}

export const SshMachineService = {
  // 全部机器（脱敏），插入序。
  async list(): Promise<SshMachine[]> {
    const rows = await Binding.List();
    return (rows ?? []).map(toMachine);
  },

  // 按 id 查（脱敏）；不存在返回 null。
  async get(id: string): Promise<SshMachine | null> {
    const m = await Binding.Get(id);
    return m ? toMachine(m) : null;
  },

  // 新增（服务端生成 id），返回脱敏记录。
  async add(input: SshMachineInput): Promise<SshMachine> {
    return toMachine(await Binding.Add(toRawInput(input)));
  },

  // 编辑（patch：password 留空保留原密码，clearStoredPassword 清空）。无匹配抛错。
  async update(id: string, input: SshMachineInput): Promise<void> {
    await Binding.Update(id, toRawInput(input));
  },

  // 删除。无匹配抛错。
  async remove(id: string): Promise<void> {
    await Binding.Remove(id);
  },

  // 连通性测试（非交互 `ssh -o BatchMode=yes ... exit`）。
  async checkConnectivity(id: string): Promise<SshConnectivityResult> {
    const r: RawConnectivityResult = await Binding.CheckConnectivity(id);
    return {
      reachable: r.reachable ?? false,
      message: r.message ?? '',
      latencyMs: r.latencyMs ?? null,
    };
  },
} as const;
