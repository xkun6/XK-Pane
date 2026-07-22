// CliUpdateService：cliupdate.Service（CLI 工具版本检测与一键升级）的前端封装。
//
// 用于 Provider 设置页 Claude / Reclaude / Codex / Grok Build 标签顶部的版本条：check 拿本地版本 +
// npm 最新版，upgrade 触发一键升级（claude/grok 优先官方自升级命令、失败回退 npm 重装；
// codex 直接 npm——上游 `codex update` 有报成功但未实际更新的假成功问题；reclaude 与 claude 同包）。
import { Service as Binding } from '@bindings/xkpane/internal/services/cliupdate';
import type {
  ToolStatusDTO as RawToolStatusDTO,
  UpgradeCliResultDTO as RawUpgradeCliResultDTO,
  InstallCliResultDTO as RawInstallCliResultDTO,
} from '@bindings/xkpane/internal/services/cliupdate/models';

export interface CliToolStatus {
  tool: string;
  installed: boolean;
  installable: boolean;
  currentVersion: string;
  latestVersion: string;
  updateAvailable: boolean;
  detail: string;
}

export interface CliUpgradeResult {
  tool: string;
  ok: boolean;
  newVersion: string;
  message: string;
}

function toStatus(dto: RawToolStatusDTO): CliToolStatus {
  return {
    tool: dto.tool ?? '',
    installed: dto.installed ?? false,
    installable: dto.installable ?? false,
    currentVersion: dto.currentVersion ?? '',
    latestVersion: dto.latestVersion ?? '',
    updateAvailable: dto.updateAvailable ?? false,
    detail: dto.detail ?? '',
  };
}

function toUpgradeResult(dto: RawUpgradeCliResultDTO): CliUpgradeResult {
  return {
    tool: dto.tool ?? '',
    ok: dto.ok ?? false,
    newVersion: dto.newVersion ?? '',
    message: dto.message ?? '',
  };
}

export interface CliInstallResult {
  tool: string;
  ok: boolean;
  newVersion: string;
  message: string;
}

function toInstallResult(dto: RawInstallCliResultDTO): CliInstallResult {
  return {
    tool: dto.tool ?? '',
    ok: dto.ok ?? false,
    newVersion: dto.newVersion ?? '',
    message: dto.message ?? '',
  };
}

export const CliUpdateService = {
  // 检测 tool（claude | reclaude | codex | grok）的本地版本与 npm 最新版本。
  async check(tool: 'claude' | 'reclaude' | 'codex' | 'grok'): Promise<CliToolStatus> {
    const dto = await Binding.CheckCliTool(tool);
    if (!dto) throw new Error('cliupdate: CheckCliTool 返回空');
    return toStatus(dto);
  },

  // 一键升级 tool。
  async upgrade(tool: 'claude' | 'reclaude' | 'codex' | 'grok'): Promise<CliUpgradeResult> {
    const dto = await Binding.UpgradeCliTool(tool);
    if (!dto) throw new Error('cliupdate: UpgradeCliTool 返回空');
    return toUpgradeResult(dto);
  },

  // 一键安装未检测到的 tool（仅 claude/codex/grok 有 npm 包，reclaude 会被后端拒绝）。
  async install(tool: 'claude' | 'reclaude' | 'codex' | 'grok'): Promise<CliInstallResult> {
    const dto = await Binding.InstallCliTool(tool);
    if (!dto) throw new Error('cliupdate: InstallCliTool 返回空');
    return toInstallResult(dto);
  },
} as const;
