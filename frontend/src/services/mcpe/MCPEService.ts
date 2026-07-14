// MCPEService：MC PE 包管理 / 启动 / 日志 的前端入口（仅 Windows 桌面）。
//
// 包装后端 internal/services/mcpe 的 Wails 绑定 + 3 个运行事件订阅（经 eventTransport）。
// MC PE 是 Windows 桌面专属，后端无 HTTP RPC 路由 → 所有 RPC 方法在 viewer 直接 reject；
// 事件订阅两端同构（viewer 收得到但无 MC PE 面板消费，无害）。对应 cc-pane web/services/mcPeService.ts。
import { Service as Binding } from '@bindings/xkpane/internal/services/mcpe';
import type {
  MCPackage,
  MCWorldConfig,
  MCDebugOptions,
  MCRunSessionInfo,
  McdkBridgeStatus,
} from '@bindings/xkpane/internal/storage/models';
import { eventTransport, isDesktop } from '@/services/transport';
import { AppEvents } from '@/shared/events';

// 业务实体直接复用后端 GORM 模型 + 前端契约（camelCase JSON tag，零转换透传）。
export type { MCPackage, MCWorldConfig, MCDebugOptions, MCRunSessionInfo, McdkBridgeStatus };

// 事件 payload 前端重定义（对齐后端 events.MCOutputPayload 等；后端 timestamp/category/level 为 *string → null 可选）。
export interface MCOutputPayload {
  runId: string;
  packageId: string;
  line: string;
  severity: string; // error|warn|success|info|verbose|developer|plain
  timestamp?: string | null;
  category?: string | null;
  level?: string | null;
}

export interface MCLaunchedPayload {
  runId: string;
  packageId: string;
}

export interface MCExitedPayload {
  runId: string;
  packageId: string;
  code: number;
}

const desktopOnly = (): Promise<never> => Promise.reject(new Error('MC PE 仅桌面端可用'));

export const MCPEService = {
  async list(): Promise<MCPackage[]> {
    if (!isDesktop) return desktopOnly();
    return Binding.ListPackages();
  },
  async create(projectPath: string, name: string): Promise<MCPackage> {
    if (!isDesktop) return desktopOnly();
    return Binding.CreatePackage(projectPath, name);
  },
  async update(pkg: MCPackage): Promise<void> {
    if (!isDesktop) return desktopOnly();
    await Binding.UpdatePackage(pkg);
  },
  async remove(id: string): Promise<void> {
    if (!isDesktop) return desktopOnly();
    await Binding.DeletePackage(id);
  },
  async scanLaunchers(): Promise<string[]> {
    if (!isDesktop) return desktopOnly();
    return Binding.ScanLaunchers();
  },
  async launch(id: string): Promise<MCRunSessionInfo> {
    if (!isDesktop) return desktopOnly();
    return Binding.Launch(id);
  },
  async stop(runId: string): Promise<void> {
    if (!isDesktop) return desktopOnly();
    await Binding.Stop(runId);
  },
  async moveWorldToTrash(id: string): Promise<void> {
    if (!isDesktop) return desktopOnly();
    await Binding.MoveWorldToTrash(id);
  },
  async openWorldFolder(id: string): Promise<string> {
    if (!isDesktop) return desktopOnly();
    return Binding.OpenWorldFolder(id);
  },
  // runId 缺省（ConfigPanel 尚未运行时的预览态调用）时按空串处理，等价于「取最新连入的客户端」。
  async getBridgeStatus(runId: string = ''): Promise<McdkBridgeStatus> {
    if (!isDesktop) return desktopOnly();
    return Binding.GetMcdkBridgeStatus(runId);
  },
  // 经 MCDK 桥触发游戏内热重载（fire-and-forget）；桥未连入时后端返回错误。多开时 runId 定向目标客户端。
  async reloadScript(runId: string): Promise<void> {
    if (!isDesktop) return desktopOnly();
    await Binding.ReloadScript(runId);
  },
  async reloadWorld(runId: string): Promise<void> {
    if (!isDesktop) return desktopOnly();
    await Binding.ReloadWorld(runId);
  },

  // 事件订阅（全局，按 runId 在调用方路由）。返回取消函数，务必在 useEffect 里 return。
  onOutput(handler: (p: MCOutputPayload) => void): () => void {
    return eventTransport.on<MCOutputPayload>(AppEvents.MCPeOutput, (d) => {
      if (d && d.runId) handler(d);
    });
  },
  onLaunched(handler: (p: MCLaunchedPayload) => void): () => void {
    return eventTransport.on<MCLaunchedPayload>(AppEvents.MCPeLaunched, (d) => {
      if (d && d.runId) handler(d);
    });
  },
  onExited(handler: (p: MCExitedPayload) => void): () => void {
    return eventTransport.on<MCExitedPayload>(AppEvents.MCPeExited, (d) => {
      if (d && d.runId) handler(d);
    });
  },
} as const;
