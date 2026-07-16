// CliUpdateService：cliupdate.Service（CLI 工具版本检测与一键升级）的前端封装。
//
// 用于 Provider 设置页 Claude / Codex 标签顶部的版本条：check 拿本地版本 + npm 最新版，
// upgrade 触发一键升级（claude 优先官方 claude update、失败回退 npm 重装；codex 直接 npm）。
import { Service as Binding } from '@bindings/xkpane/internal/services/cliupdate';
import type {
  ToolStatusDTO as RawToolStatusDTO,
  UpgradeCliResultDTO as RawUpgradeCliResultDTO,
} from '@bindings/xkpane/internal/services/cliupdate/models';

export interface CliToolStatus {
  tool: string;
  installed: boolean;
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

export const CliUpdateService = {
  // 检测 tool（claude | codex）的本地版本与 npm 最新版本。
  async check(tool: 'claude' | 'codex'): Promise<CliToolStatus> {
    const dto = await Binding.CheckCliTool(tool);
    if (!dto) throw new Error('cliupdate: CheckCliTool 返回空');
    return toStatus(dto);
  },

  // 一键升级 tool。
  async upgrade(tool: 'claude' | 'codex'): Promise<CliUpgradeResult> {
    const dto = await Binding.UpgradeCliTool(tool);
    if (!dto) throw new Error('cliupdate: UpgradeCliTool 返回空');
    return toUpgradeResult(dto);
  },
} as const;
