// AgentCfgService：agentcfg.Service（Provider / MCP server / Skills 三域）的前端封装。
//
// Provider：每个 Provider 存的是「CLI 全局配置文件的完整内容」——claude 对应
// ~/.claude/settings.json 整份 JSON；codex 对应 {auth: <auth.json 对象|null>, config: "<config.toml
// 全文>"} 组装后的 JSON。切换（switchTo）会直写对应 CLI 的 live 配置文件，影响该 CLI 之后
// 启动的所有会话（包括 XK-Pane 之外）。
//
// MCP server / Skills：以 XK-Pane 为唯一事实源（SSOT），按每个 CLI 各自的开关投影到
// ~/.claude.json 的 mcpServers、~/.codex/config.toml 的 [mcp_servers]、~/.claude/skills、
// ~/.codex/skills。「禁用」= 从对应 CLI 移除但 SSOT 保留；Skill 删除会把 SSOT 目录挪进回收
// 目录而非直接销毁。
import {
  Service as Binding,
  AgentProviderInput as RawInput,
  AgentMcpServerInput as RawMcpInput,
} from '@bindings/xkpane/internal/services/agentcfg';
import type {
  AgentProviderDTO as RawProviderDTO,
  AgentMcpServerDTO as RawMcpDTO,
  AgentSkillDTO as RawSkillDTO,
} from '@bindings/xkpane/internal/services/agentcfg/models';

export type AgentAppType = 'claude' | 'codex';

export interface AgentProvider {
  id: string;
  appType: AgentAppType;
  name: string;
  category: string; // official | custom | imported
  isCurrent: boolean;
  sortOrder: number;
  notes: string;
  websiteUrl: string;
  settingsJson: string;
  createdAt: string;
  updatedAt: string;
}

// AppType 仅 add 时生效（update 不允许改）；此处仍要求传入以便类型层面锁死可写字段。
export interface AgentProviderInput {
  appType: AgentAppType;
  name: string;
  category?: string;
  notes?: string;
  websiteUrl?: string;
  settingsJson: string;
}

function toProvider(p: RawProviderDTO): AgentProvider {
  return {
    id: p.id ?? '',
    appType: (p.appType as AgentAppType) ?? 'claude',
    name: p.name ?? '',
    category: p.category ?? 'custom',
    isCurrent: p.isCurrent ?? false,
    sortOrder: p.sortOrder ?? 0,
    notes: p.notes ?? '',
    websiteUrl: p.websiteUrl ?? '',
    settingsJson: p.settingsJson ?? '',
    createdAt: (p.createdAt as unknown as string) ?? '',
    updatedAt: (p.updatedAt as unknown as string) ?? '',
  };
}

function toRawInput(input: AgentProviderInput): RawInput {
  return new RawInput({
    appType: input.appType,
    name: input.name,
    category: input.category ?? '',
    notes: input.notes ?? '',
    websiteUrl: input.websiteUrl ?? '',
    settingsJson: input.settingsJson,
  });
}

export interface AgentMcpServer {
  id: string;
  name: string;
  serverJson: string;
  enabledClaude: boolean;
  enabledCodex: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface AgentMcpServerInput {
  name: string;
  serverJson: string;
  enabledClaude: boolean;
  enabledCodex: boolean;
}

function toMcpServer(m: RawMcpDTO): AgentMcpServer {
  return {
    id: m.id ?? '',
    name: m.name ?? '',
    serverJson: m.serverJson ?? '',
    enabledClaude: m.enabledClaude ?? false,
    enabledCodex: m.enabledCodex ?? false,
    createdAt: (m.createdAt as unknown as string) ?? '',
    updatedAt: (m.updatedAt as unknown as string) ?? '',
  };
}

function toRawMcpInput(input: AgentMcpServerInput): RawMcpInput {
  return new RawMcpInput({
    name: input.name,
    serverJson: input.serverJson,
    enabledClaude: input.enabledClaude,
    enabledCodex: input.enabledCodex,
  });
}

export interface AgentSkill {
  id: string;
  name: string;
  description: string;
  enabledClaude: boolean;
  enabledCodex: boolean;
  createdAt: string;
  updatedAt: string;
}

function toSkill(s: RawSkillDTO): AgentSkill {
  return {
    id: s.id ?? '',
    name: s.name ?? '',
    description: s.description ?? '',
    enabledClaude: s.enabledClaude ?? false,
    enabledCodex: s.enabledCodex ?? false,
    createdAt: (s.createdAt as unknown as string) ?? '',
    updatedAt: (s.updatedAt as unknown as string) ?? '',
  };
}

export const AgentCfgService = {
  // 按 appType 列出全部 Provider（含解密后的明文配置），按 sortOrder/createdAt 排序。
  async list(appType: AgentAppType): Promise<AgentProvider[]> {
    const rows = await Binding.ListAgentProviders(appType);
    return (rows ?? []).map(toProvider);
  },

  // 新增（服务端生成 id）。
  async add(input: AgentProviderInput): Promise<AgentProvider> {
    const dto = await Binding.AddAgentProvider(toRawInput(input));
    if (!dto) throw new Error('agentcfg: AddAgentProvider 返回空');
    return toProvider(dto);
  },

  // 编辑（appType 不可改）。若该行是当前使用中的 Provider，后端会立即把新配置同步写入
  // 对应 CLI 的 live 配置文件。
  async update(id: string, input: AgentProviderInput): Promise<AgentProvider> {
    const dto = await Binding.UpdateAgentProvider(id, toRawInput(input));
    if (!dto) throw new Error('agentcfg: UpdateAgentProvider 返回空');
    return toProvider(dto);
  },

  // 删除。当前使用中的 Provider 后端会拒绝删除。
  async remove(id: string): Promise<void> {
    await Binding.DeleteAgentProvider(id);
  },

  // 切为当前，并原子性写入对应 CLI 的 live 配置文件（switch 是 JS 保留字，方法名用 switchTo）。
  async switchTo(id: string): Promise<void> {
    await Binding.SwitchAgentProvider(id);
  },

  // 从当前 live 配置手动导入一份新 Provider。
  async importCurrent(appType: AgentAppType): Promise<AgentProvider> {
    const dto = await Binding.ImportCurrentAgentConfig(appType);
    if (!dto) throw new Error('agentcfg: ImportCurrentAgentConfig 返回空');
    return toProvider(dto);
  },

  // —— MCP server（SSOT，按 name 排序） ——
  async listMcp(): Promise<AgentMcpServer[]> {
    const rows = await Binding.ListMcpServers();
    return (rows ?? []).map(toMcpServer);
  },

  async addMcp(input: AgentMcpServerInput): Promise<AgentMcpServer> {
    const dto = await Binding.AddMcpServer(toRawMcpInput(input));
    if (!dto) throw new Error('agentcfg: AddMcpServer 返回空');
    return toMcpServer(dto);
  },

  async updateMcp(id: string, input: AgentMcpServerInput): Promise<AgentMcpServer> {
    const dto = await Binding.UpdateMcpServer(id, toRawMcpInput(input));
    if (!dto) throw new Error('agentcfg: UpdateMcpServer 返回空');
    return toMcpServer(dto);
  },

  async removeMcp(id: string): Promise<void> {
    await Binding.DeleteMcpServer(id);
  },

  // 从两侧 live 配置读入 SSOT，返回新增行数。
  async importMcpFromLive(): Promise<number> {
    return Binding.ImportMcpFromLive();
  },

  // —— Skills（SSOT，按 name 排序） ——
  async listSkills(): Promise<AgentSkill[]> {
    const rows = await Binding.ListAgentSkills();
    return (rows ?? []).map(toSkill);
  },

  // 从一个含 SKILL.md 的目录导入（id 取目录名，两开关默认关闭）。
  async importSkillDir(srcPath: string): Promise<AgentSkill> {
    const dto = await Binding.ImportSkillDir(srcPath);
    if (!dto) throw new Error('agentcfg: ImportSkillDir 返回空');
    return toSkill(dto);
  },

  async setSkillEnabled(id: string, enabledClaude: boolean, enabledCodex: boolean): Promise<AgentSkill> {
    const dto = await Binding.SetAgentSkillEnabled(id, enabledClaude, enabledCodex);
    if (!dto) throw new Error('agentcfg: SetAgentSkillEnabled 返回空');
    return toSkill(dto);
  },

  async removeSkill(id: string): Promise<void> {
    await Binding.DeleteAgentSkill(id);
  },

  // 从 ~/.claude/skills 与 ~/.codex/skills 扫入 SSOT，返回新增行数。
  async importSkillsFromApps(): Promise<number> {
    return Binding.ImportSkillsFromApps();
  },
} as const;
