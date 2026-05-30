// CliToolService：CLI 工具注册表的前端入口（对齐 cc-pane web/services/cliToolService.ts）。
//
// 后端 clitools service 暴露：
//   - ListTools()  静态元信息（快，不检测安装）
//   - DetectAll()  检测安装状态 / 版本 / 能力（慢，跑 which + --version）
// 启动菜单用 DetectAll 的结果决定每个 CLI 是否可点、显示版本、读 capabilities。
import { Service as Binding } from '@bindings/xkpane/internal/services/clitools';
import type { ToolInfo as RawToolInfo } from '@bindings/xkpane/internal/services/cliadapters/models';

export interface CliToolCapabilities {
  supportsProvider: boolean;
  supportsResume: boolean;
  supportsMcp: boolean;
  supportsSystemPrompt: boolean;
  supportsWorkspace: boolean;
  supportsProjectHooks: boolean;
  compatibleProviderTypes: string[];
}

export interface CliTool {
  id: string;
  displayName: string;
  executable: string;
  installed: boolean;
  version: string;
  path: string;
  capabilities: CliToolCapabilities | null;
}

function toCapabilities(c: RawToolInfo['capabilities']): CliToolCapabilities | null {
  if (!c) return null;
  return {
    supportsProvider: c.supportsProvider ?? false,
    supportsResume: c.supportsResume ?? false,
    supportsMcp: c.supportsMcp ?? false,
    supportsSystemPrompt: c.supportsSystemPrompt ?? false,
    supportsWorkspace: c.supportsWorkspace ?? false,
    supportsProjectHooks: c.supportsProjectHooks ?? false,
    compatibleProviderTypes: c.compatibleProviderTypes ?? [],
  };
}

function toCliTool(t: RawToolInfo): CliTool {
  return {
    id: t.id ?? '',
    displayName: t.displayName ?? '',
    executable: t.executable ?? '',
    installed: t.installed ?? false,
    version: t.version ?? '',
    path: t.path ?? '',
    capabilities: toCapabilities(t.capabilities),
  };
}

export const CliToolService = {
  // 静态工具列表（不检测安装状态）。
  async listTools(): Promise<CliTool[]> {
    const rows = await Binding.ListTools();
    return (rows ?? []).map(toCliTool);
  },

  // 检测全部工具（安装状态 / 版本 / 能力）。
  async detectAll(): Promise<CliTool[]> {
    const rows = await Binding.DetectAll();
    return (rows ?? []).map(toCliTool);
  },
} as const;
