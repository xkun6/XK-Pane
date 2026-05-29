// ProviderService：接入 Provider（providers 表）的前端封装。
//
// 后端按 D8 落 SQLite 且 apiKey 经 cryptox 加密；**明文 apiKey 绝不跨 IPC 回前端**——
// List/Get 返回脱敏视图（仅 hasApiKey 标志）。新增/编辑时前端传明文 apiKey；编辑留空 =
// 保留原密钥（后端 patch）。详见 findings J9。
import { Service as Binding } from '@bindings/xkpane/internal/services/provider';
import type { ProviderDTO as RawProviderDTO } from '@bindings/xkpane/internal/services/provider/models';

// Provider：脱敏视图（前端可见）。无明文 apiKey，仅 hasApiKey。
export interface Provider {
  id: string;
  name: string;
  providerType: string; // anthropic|bedrock|vertex|proxy|config_profile|open_ai|gemini|kimi|glm|cursor|opencode
  hasApiKey: boolean;
  baseUrl: string;
  region: string;
  projectId: string;
  awsProfile: string;
  configDir: string;
  isDefault: boolean;
}

// ProviderInput：新增/编辑提交字段。apiKey 明文（编辑留空 = 保留原密钥）。
export interface ProviderInput {
  name: string;
  providerType: string;
  apiKey: string;
  baseUrl: string;
  region: string;
  projectId: string;
  awsProfile: string;
  configDir: string;
  isDefault: boolean;
}

function toProvider(p: RawProviderDTO): Provider {
  return {
    id: p.id ?? '',
    name: p.name ?? '',
    providerType: p.providerType ?? 'anthropic',
    hasApiKey: p.hasApiKey ?? false,
    baseUrl: p.baseUrl ?? '',
    region: p.region ?? '',
    projectId: p.projectId ?? '',
    awsProfile: p.awsProfile ?? '',
    configDir: p.configDir ?? '',
    isDefault: p.isDefault ?? false,
  };
}

// 显式构造提交对象（不直接传接口实例，避免与生成 model 结构耦合）。
function toRawInput(input: ProviderInput) {
  return {
    name: input.name,
    providerType: input.providerType,
    apiKey: input.apiKey,
    baseUrl: input.baseUrl,
    region: input.region,
    projectId: input.projectId,
    awsProfile: input.awsProfile,
    configDir: input.configDir,
    isDefault: input.isDefault,
  };
}

export const ProviderService = {
  // 全部 provider（脱敏），插入序。
  async list(): Promise<Provider[]> {
    const rows = await Binding.List();
    return (rows ?? []).map(toProvider);
  },

  // 按 id 查（脱敏）；不存在返回 null。
  async get(id: string): Promise<Provider | null> {
    const p = await Binding.Get(id);
    return p ? toProvider(p) : null;
  },

  // 新增（服务端生成 id），返回脱敏记录。
  async add(input: ProviderInput): Promise<Provider> {
    return toProvider(await Binding.Add(toRawInput(input)));
  },

  // 编辑（patch：apiKey 留空保留原密钥；不动 isDefault）。无匹配抛错。
  async update(id: string, input: ProviderInput): Promise<void> {
    await Binding.Update(id, toRawInput(input));
  },

  // 删除（删默认会提升最早的剩余为默认）。无匹配抛错。
  async remove(id: string): Promise<void> {
    await Binding.Remove(id);
  },

  // 设为唯一默认。未知 id 抛错。
  async setDefault(id: string): Promise<void> {
    await Binding.SetDefault(id);
  },
} as const;
