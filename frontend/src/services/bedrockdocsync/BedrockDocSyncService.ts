// BedrockDocSyncService：Bedrock 知识库 OTA 文档同步的前端入口（仅 Windows / 桌面）。
//
// 包装后端 internal/services/bedrockdocsync 的 Wails 绑定 + BedrockDocSyncProgress 进度事件订阅。
// OTA 同步是桌面专属（Web 网关未挂载该 service 的 RPC 路由）→ 所有 RPC 在 viewer 直接 reject；
// 进度事件订阅两端同构（viewer 收得到但无面板消费，无害）。对齐 MCPEService 的桌面守护模式。
import { Service as Binding } from '@bindings/xkpane/internal/services/bedrockdocsync';
import type {
  SyncOptions,
  SyncReport,
  SyncStatus,
  ProxyConfig,
  ProbeResult,
  ResetReport,
  ArchiveInfo,
  SyncSourcesConfig,
  Manifest,
  ManifestStats,
  SourceInfo,
} from '@bindings/xkpane/internal/services/bedrockdocsync/models';
import { eventTransport, isDesktop } from '@/services/transport';
import { AppEvents } from '@/shared/events';

// read-model 直接复用后端生成的绑定类型（camelCase JSON tag，零转换透传）。
export type {
  SyncOptions,
  SyncReport,
  SyncStatus,
  ProxyConfig,
  ProbeResult,
  ResetReport,
  ArchiveInfo,
  SyncSourcesConfig,
  Manifest,
  ManifestStats,
  SourceInfo,
};

// 来源标识（对齐后端 SourceSelector）。
export type DocSyncSource = 'mcguide' | 'mconline' | 'mcdocs';

// sync 友好入参（绑定生成的 SyncOptions 字段为「必填但可 undefined」，故对外收口成全可选的 input，
// 内部补齐全字段再过桥）。
export interface SyncInput {
  sources?: DocSyncSource[];
  concurrency?: number;
  mcdocsBaseUrl?: string;
}

// 进度事件 payload（前端重定义，对齐后端 events.BedrockDocSyncProgressPayload，按 stage 取用字段）。
export interface BedrockDocSyncProgress {
  stage: string; // started|gitea_sync|http_fetch|build_index|validating|completed|cancelled|failed|log
  sources?: string[]; // started
  source?: string; // gitea_sync / http_fetch
  status?: string; // gitea_sync
  fetched?: number; // http_fetch
  total?: number; // http_fetch
  kind?: string; // build_index（api / templates）
  entries?: number; // build_index
  manifestPath?: string; // completed
  durationMs?: number; // completed
  error?: string; // failed
  partial?: boolean; // failed
  level?: string; // log（info / warn / error）
  msg?: string; // log
}

const desktopOnly = (): Promise<never> =>
  Promise.reject(new Error('知识库 OTA 同步仅桌面端可用'));

export const BedrockDocSyncService = {
  // 读当前 bedrock-data 状态（present / 大小 / 归档 / manifest / runtime 计数）。
  async status(): Promise<SyncStatus> {
    if (!isDesktop) return desktopOnly();
    return Binding.Status();
  },

  // 跑一次同步。空 input → 后端默认源 [mcguide, mconline]。阻塞至完成，进度经 onProgress 流出。
  // 补齐 SyncOptions 全字段（proxy/sourcesConfig 传 null，让后端读持久化 / env）。
  async sync(input: SyncInput = {}): Promise<SyncReport> {
    if (!isDesktop) return desktopOnly();
    return Binding.Sync({
      sources: input.sources,
      concurrency: input.concurrency ?? null,
      proxy: null,
      mcdocsBaseUrl: input.mcdocsBaseUrl,
      sourcesConfig: null,
    });
  },

  // 取消当前同步（阶段边界中止）。
  async cancel(): Promise<void> {
    if (!isDesktop) return desktopOnly();
    await Binding.Cancel();
  },

  // 归档现有数据并回退内嵌基线。
  async reset(): Promise<ResetReport> {
    if (!isDesktop) return desktopOnly();
    return Binding.Reset();
  },

  async listArchives(): Promise<ArchiveInfo[]> {
    if (!isDesktop) return desktopOnly();
    return Binding.ListArchives();
  },

  // 探测代理（HEAD github.com）；探测成功才持久化 proxy.json。返回探测结果。
  async setProxy(proxy: ProxyConfig): Promise<ProbeResult> {
    if (!isDesktop) return desktopOnly();
    return Binding.SetProxy(proxy);
  },

  async getProxy(): Promise<ProxyConfig> {
    if (!isDesktop) return desktopOnly();
    return Binding.GetProxy();
  },

  // 持久化 per-source remote/branch 覆盖（无探测）。
  async setSources(cfg: SyncSourcesConfig): Promise<void> {
    if (!isDesktop) return desktopOnly();
    await Binding.SetSources(cfg);
  },

  async getSources(): Promise<SyncSourcesConfig> {
    if (!isDesktop) return desktopOnly();
    return Binding.GetSources();
  },

  // 订阅进度事件。返回取消函数，务必在 useEffect 里 return。
  onProgress(handler: (p: BedrockDocSyncProgress) => void): () => void {
    return eventTransport.on<BedrockDocSyncProgress>(AppEvents.BedrockDocSyncProgress, (d) => {
      if (d && d.stage) handler(d);
    });
  },
} as const;
