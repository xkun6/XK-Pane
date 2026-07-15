// AgentCfgService：cc-switch 式全局配置 Provider（agentcfg.Service）的前端封装。
//
// 每个 Provider 存的是「CLI 全局配置文件的完整内容」——claude 对应 ~/.claude/settings.json
// 整份 JSON；codex 对应 {auth: <auth.json 对象|null>, config: "<config.toml 全文>"} 组装后的
// JSON。切换（switchTo）会直写对应 CLI 的 live 配置文件，影响该 CLI 之后启动的所有会话
// （包括 XK-Pane 之外）。
//
// 本单元只包 Provider 部分（MCP server / Skills 由后续单元扩）。
import {
  Service as Binding,
  AgentProviderInput as RawInput,
} from '@bindings/xkpane/internal/services/agentcfg';
import type { AgentProviderDTO as RawProviderDTO } from '@bindings/xkpane/internal/services/agentcfg/models';

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
} as const;
