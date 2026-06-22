import { useEffect, useMemo, useRef, useState } from 'react';
import {
  TranscriptService,
  TerminalService,
  type TimelineEvent,
  type TimelineQuestion,
  type TimelineMetrics,
} from '@/services';
import { derivePending } from './derivePending';

// 轮询间隔：转录是 append-only 文件，~1.5s 拉一次增量足够「准实时」，不压后端。
const POLL_MS = 1500;
// 「工作中」静默窗口(ms)：Claude 工作时 TUI 自旋/计时器持续吐 PTY 输出；停止输出超此时长 → 视为空闲。
// 取 2s 留足渲染节流余量（自旋至少 ~1Hz），又不致工作结束后指示器滞留太久。
const BUSY_QUIET_MS = 2000;

export interface UseSessionTimelineArgs {
  sessionId: string;
  active: boolean; // 仅可见时轮询（隐藏即停，省资源）
}

export interface UseSessionTimelineResult {
  events: TimelineEvent[];
  title: string;
  supported: boolean | null; // null = 首拉前未知
  loading: boolean;
  // 当前【待答】的 AskUserQuestion 问题（来自 PreToolUse hook 旁路事件，落盘转录之前即到）。
  // null = 无待答。答完（转录增长）后自动清除。卡片据此渲染可作答的待答卡（与只读历史区分）。
  pending: TimelineQuestion[] | null;
  // 会话「工作中」liveness：基于 PTY 输出活跃度推断（工作时 TUI 自旋持续吐字节；空闲于提示符则静默）。
  // 供卡片底部渲染「思考中…」指示器，让用户确信 Agent 仍在跑、未因意外中断。
  busy: boolean;
  // 状态行指标（上下文/5h/周限，复用 claude-hud 产物）。null = 暂不可用。每轮轮询刷新（含剩余时间倒计）。
  metrics: TimelineMetrics | null;
}

// useSessionTimeline：按游标增量轮询活跃会话的结构化时间线，累积事件。
// 会话切换重置；supported===false 时自停轮询（非 claude 引擎无结构化转录）。
export function useSessionTimeline({ sessionId, active }: UseSessionTimelineArgs): UseSessionTimelineResult {
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [title, setTitle] = useState('');
  const [supported, setSupported] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(true);
  const [pending, setPending] = useState<TimelineQuestion[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [metrics, setMetrics] = useState<TimelineMetrics | null>(null);
  const busyRef = useRef(false); // 仅在 idle↔busy 跳变时 setState，避免每个输出批次都重渲
  const cursorRef = useRef(0);
  const answeredAskCountRef = useRef(0); // 「已答 AskUserQuestion」数（供 ask-pending 回调即时读取设基线）
  const pendingAnsweredBaseRef = useRef(0); // 设 pending 时的已答数：之后超过它 = 该问题已答 → 清 pending

  // 会话切换：清空累积、重置游标 + 待答态 + 工作态。
  useEffect(() => {
    cursorRef.current = 0;
    setEvents([]);
    setTitle('');
    setSupported(null);
    setLoading(true);
    setPending(null);
    setMetrics(null);
    busyRef.current = false;
    setBusy(false);
  }, [sessionId]);

  // 「工作中」liveness：订阅会话 PTY 输出（桌面 Wails 总线 / Web /ws 镜像同一套），有输出即 busy，
  // 静默超 BUSY_QUIET_MS 即落 idle。仅卡片可见(active)时订阅。只在 idle↔busy 跳变时 setState。
  useEffect(() => {
    if (!sessionId || !active) {
      busyRef.current = false;
      setBusy(false);
      return;
    }
    let idleTimer: ReturnType<typeof setTimeout> | undefined;
    const off = TerminalService.onOutput(sessionId, () => {
      if (!busyRef.current) {
        busyRef.current = true;
        setBusy(true);
      }
      if (idleTimer) clearTimeout(idleTimer);
      idleTimer = setTimeout(() => {
        busyRef.current = false;
        setBusy(false);
      }, BUSY_QUIET_MS);
    });
    return () => {
      off();
      if (idleTimer) clearTimeout(idleTimer);
      busyRef.current = false;
      setBusy(false);
    };
  }, [sessionId, active]);

  // 「已答 AskUserQuestion」数：tool_use(AskUserQuestion) 且其 toolUseId 存在匹配的 tool_result。
  // ★ 命门：待答期间转录会先落该问题【自己】的 assistant tool_use 行（无 result —— 实测 jsonl 里 tool_use 与
  //   tool_result 分属相邻两行、tool_use 在前并于作答前即写入）。若按「事件总数增长」判定已答，会把这条
  //   tool_use 误当成已答而提前清掉底部可点的交互卡 → 退化成中间那张只读卡（不可点）。故只认 tool_result 落地。
  const answeredAskCount = useMemo(() => {
    const resultIds = new Set<string>();
    for (const ev of events) if (ev.kind === 'tool_result' && ev.toolUseId) resultIds.add(ev.toolUseId);
    let n = 0;
    for (const ev of events)
      if (ev.kind === 'tool_use' && ev.toolName === 'AskUserQuestion' && ev.toolUseId && resultIds.has(ev.toolUseId)) n++;
    return n;
  }, [events]);

  // 维护已答数 ref（ask-pending 回调在另一闭包里，靠 ref 读最新值）。
  useEffect(() => {
    answeredAskCountRef.current = answeredAskCount;
  }, [answeredAskCount]);

  // 转录派生的待答：事件漏掉时的可靠回退（尤其 web viewer 事后挂载错过 fire-and-forget 事件）。详见 derivePending。
  const derivedPending = useMemo(() => derivePending(events), [events]);

  // 订阅「待答 AskUserQuestion」：始终监听（不漏 —— 事件 fire-and-forget 无重放），按会话过滤。
  useEffect(() => {
    if (!sessionId) return;
    const off = TranscriptService.onAskPending(sessionId, (questions) => {
      setPending(questions.length ? questions : null);
      pendingAnsweredBaseRef.current = answeredAskCountRef.current; // 记录设 pending 时的已答数
    });
    return off;
  }, [sessionId]);

  // 答完自动清除：待答期间 Claude 阻塞，唯一新增是该问题自己的 assistant tool_use（无 result，不计入
  // answeredAskCount）；一旦答完（终端或卡片）落 tool_result → answeredAskCount 增长 → 清 pending
  //（之后由转录的只读历史卡接管展示，无缝交接、不重叠）。
  useEffect(() => {
    if (pending && answeredAskCount > pendingAnsweredBaseRef.current) setPending(null);
  }, [answeredAskCount, pending]);

  useEffect(() => {
    if (!sessionId || !active) return;
    let alive = true;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const tick = async (): Promise<void> => {
      let reschedule = true;
      try {
        const tl = await TranscriptService.getTimeline(sessionId, cursorRef.current);
        if (!alive) return;
        setSupported(tl.supported);
        setLoading(false);
        if (!tl.supported) {
          reschedule = false; // 非 claude：无结构化转录，停轮询，前端回退 xterm
          return;
        }
        if (tl.title) setTitle(tl.title);
        setMetrics(tl.metrics ?? null); // 每轮刷新（剩余时间倒计 / 用量变化）
        if (tl.total > cursorRef.current) {
          cursorRef.current = tl.total;
          if (tl.events.length > 0) setEvents((prev) => [...prev, ...tl.events]);
        }
      } catch {
        if (alive) setLoading(false); // 瞬时错误：保留已有事件，下轮重试
      } finally {
        if (alive && active && reschedule) timer = setTimeout(() => void tick(), POLL_MS);
      }
    };

    void tick();
    return () => {
      alive = false;
      if (timer) clearTimeout(timer);
    };
  }, [sessionId, active]);

  // 事件秒级快路径优先，转录派生兜底：两端行为一致，且彻底堵上 web viewer 错过 fire-and-forget 事件的洞。
  return { events, title, supported, loading, pending: pending ?? derivedPending, busy, metrics };
}
