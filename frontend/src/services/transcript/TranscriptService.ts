// TranscriptService：拉取活跃会话的「结构化对话时间线」（Web/手机的重排视图数据源）。
//
// 为什么存在：XK-Pane 的 PTY 镜像是按某宽度排好版的原始字节（绝对光标定位 + alt-screen），
// 与宽度焊死、无法在窄屏重排。而后端 GET /api/sessions/timeline 从 Claude 的 JSONL 转录
// （排版之前的结构化真相）解析出 user/assistant/thinking/tool 事件，可在手机上自由重排。
// 详见记忆 xkpane-mobile-strategy / 后端 internal/services/transcript。
//
// 桌面（owner）走 Wails binding transcriptsvc.GetTimeline；Web（viewer）走 /api/sessions/timeline。
// 两路同源同语义（后端 transcriptsvc 与 webapi handler 复用同一套 sessionId→转录解析逻辑）。
// after = 已消费行数游标，append-only 的 jsonl 保证增量轮询不漏不重。
import { eventTransport, httpClient, isDesktop } from '@/services/transport';
import { AppEvents } from '@/shared/events';
import { Service as TranscriptBinding } from '@bindings/xkpane/internal/services/transcriptsvc';

export type TimelineKind = 'user' | 'assistant' | 'thinking' | 'tool_use' | 'tool_result';

// AskUserQuestion 的结构化选项 / 问题（对齐后端 transcript.AskOption / AskQuestion）。
// 仅 toolName==='AskUserQuestion' 的 tool_use 事件带 questions，供卡片渲染可点选项 + 作答。
export interface TimelineOption {
  label: string;
  description?: string;
}

export interface TimelineQuestion {
  header?: string;
  question: string;
  multiSelect?: boolean;
  options: TimelineOption[];
}

// 工具结果的结构化细节（对齐后端 transcript.ToolResultDetail）。仅 kind==='tool_result' 事件可能带 detail：
// 'edit' = Edit/Write/MultiEdit 的 structuredPatch 差异；'bash' = Bash 的 stdout/stderr 分离。供卡片做
// 差异 / 输出分离渲染；无 detail 时回退 text 纯文本。
export interface TimelineToolDetail {
  kind: 'edit' | 'bash' | string;
  filePath?: string;
  diff?: string[]; // edit：差异行（每行带前导 +/-/空格）
  added?: number; // edit：新增行数
  removed?: number; // edit：删除行数
  truncated?: boolean; // edit：差异行被截断
  stdout?: string; // bash：标准输出
  stderr?: string; // bash：标准错误
}

export interface TimelineEvent {
  seq: number;
  uuid?: string;
  kind: TimelineKind;
  text?: string;
  toolName?: string;
  toolInput?: string;
  toolUseId?: string;
  isError?: boolean;
  timestamp?: string;
  questions?: TimelineQuestion[];
  detail?: TimelineToolDetail;
}

// 卡片底部状态行指标（对齐后端 hudstats.Metrics，复用 claude-hud 落盘产物）。各项可缺省。
export interface TimelineContextMetric {
  usedPercentage: number; // 原生 used%（取色阈值）
  usedTokens: number; // input+cache_creation+cache_read（不含 output，对齐 claude-hud）
  windowSize: number; // 上下文窗口大小（200000 / 1000000）
}

export interface TimelineUsageWindow {
  usedPercentage: number;
  resetsAtMs?: number; // 重置时刻 unix ms；缺省 = 未知/已过（不显示剩余时间）
}

export interface TimelineMetrics {
  context?: TimelineContextMetric;
  fiveHour?: TimelineUsageWindow;
  sevenDay?: TimelineUsageWindow;
}

// TimelinePending 是后端经轮询回灌的「当前待答 AskUserQuestion」（含 toolUseId）。供晚到 / 重连的
// web viewer 恢复待答态 —— ask-pending 事件 fire-and-forget 无重放、其 tool_use 阻塞期间不落转录，
// 故唯一可靠途径是 timeline 轮询带回。toolUseId 供前端判收尾：它一旦现身累积转录即失效（作答 / 取消）。
export interface TimelinePending {
  toolUseId: string;
  questions: TimelineQuestion[];
}

export interface Timeline {
  supported: boolean;
  events: TimelineEvent[];
  total: number; // 文件总行数 = 下次轮询的 after 游标
  title?: string;
  metrics?: TimelineMetrics;
  pending?: TimelinePending | null; // 当前待答（后端回灌），无则 null/缺省
}

interface TimelineDTO {
  supported?: boolean;
  events?: TimelineEvent[];
  total?: number;
  title?: string;
  metrics?: TimelineMetrics;
  pending?: TimelinePending | null;
}

export const TranscriptService = {
  // 拉取 [after, 末尾) 的结构化时间线事件。桌面走 binding、Web 走 HTTP 端点。
  async getTimeline(sessionId: string, after = 0): Promise<Timeline> {
    if (isDesktop) {
      const res = await TranscriptBinding.GetTimeline(sessionId, after);
      // metrics 是后端新增的嵌套标量字段，经 bindings Object.assign 透传到实例（生成类型未声明），手动 cast 取出。
      const dto = res as unknown as TimelineDTO;
      return {
        supported: res.supported === true,
        events: (res.events ?? []) as TimelineEvent[],
        total: Number(res.total ?? 0),
        ...(res.title ? { title: res.title } : {}),
        ...(dto.metrics ? { metrics: dto.metrics } : {}),
        pending: dto.pending ?? null, // 桌面 binding 暂不返回（owner 走事件即可）；缺省 null
      };
    }
    const res = await httpClient.get<TimelineDTO>(
      `/api/sessions/timeline?sessionId=${encodeURIComponent(sessionId)}&after=${after}`,
    );
    return {
      supported: res.supported === true,
      events: res.events ?? [],
      total: Number(res.total ?? 0),
      title: res.title,
      ...(res.metrics ? { metrics: res.metrics } : {}),
      pending: res.pending ?? null,
    };
  },

  // 订阅「待答 AskUserQuestion」事件（被启动 CLI 的 PreToolUse hook 旁路推送，落盘转录之前即到）。
  // 按 sessionId 过滤，handler 收到该会话当前待答的问题列表；返回取消订阅函数。桌面走 Wails 总线、
  // Web viewer 走 /ws 镜像（事件已入 webapi mirroredEvents 白名单），同一套代码。
  onAskPending(sessionId: string, handler: (questions: TimelineQuestion[]) => void): () => void {
    return eventTransport.on<{ sessionId: string; questions?: TimelineQuestion[] }>(
      AppEvents.TerminalAskPending,
      (d) => {
        if (d && d.sessionId === sessionId) handler(d.questions ?? []);
      },
    );
  },
} as const;
