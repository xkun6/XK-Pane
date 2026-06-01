import { useEffect, useRef, useState } from 'react';
import { TranscriptService, type TimelineEvent } from '@/services';

// 轮询间隔：转录是 append-only 文件，~1.5s 拉一次增量足够「准实时」，不压后端。
const POLL_MS = 1500;

export interface UseSessionTimelineArgs {
  sessionId: string;
  active: boolean; // 仅可见时轮询（隐藏即停，省资源）
}

export interface UseSessionTimelineResult {
  events: TimelineEvent[];
  title: string;
  supported: boolean | null; // null = 首拉前未知
  loading: boolean;
}

// useSessionTimeline：按游标增量轮询活跃会话的结构化时间线，累积事件。
// 会话切换重置；supported===false 时自停轮询（非 claude 引擎无结构化转录）。
export function useSessionTimeline({ sessionId, active }: UseSessionTimelineArgs): UseSessionTimelineResult {
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [title, setTitle] = useState('');
  const [supported, setSupported] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(true);
  const cursorRef = useRef(0);

  // 会话切换：清空累积、重置游标。
  useEffect(() => {
    cursorRef.current = 0;
    setEvents([]);
    setTitle('');
    setSupported(null);
    setLoading(true);
  }, [sessionId]);

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

  return { events, title, supported, loading };
}
