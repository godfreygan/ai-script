import { useEffect, useRef, useState } from 'react';
import { useAuthStore } from '@/stores/auth';

export interface ProgressEvent {
  topic: string;
  type: 'progress' | 'done' | 'error' | 'log';
  percent?: number;
  message?: string;
  data?: Record<string, unknown>;
  time?: number;
}

// 后端是 /ws/progress?token=...&topic=...
function buildWsURL(topic: string, token: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${window.location.host}/ws/progress?token=${encodeURIComponent(token)}&topic=${encodeURIComponent(topic)}`;
}

/**
 * 订阅 topic 上的进度事件。topic 为 null/空时连接不建立。
 * 返回事件历史与连接状态;done 或 error 后保留最后一条事件,直到 topic 改变。
 */
export function useProgressWS(topic: string | null | undefined) {
  const [events, setEvents] = useState<ProgressEvent[]>([]);
  const [connected, setConnected] = useState(false);
  const [last, setLast] = useState<ProgressEvent | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    if (!topic) {
      setEvents([]);
      setLast(null);
      return;
    }
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    const ws = new WebSocket(buildWsURL(topic, token));
    wsRef.current = ws;

    ws.onopen = () => setConnected(true);
    ws.onclose = () => setConnected(false);
    ws.onerror = () => setConnected(false);
    ws.onmessage = (e) => {
      try {
        const ev: ProgressEvent = JSON.parse(e.data);
        setLast(ev);
        setEvents((prev) => [...prev.slice(-50), ev]);
      } catch (err) {
        // eslint-disable-next-line no-console
        console.error('WebSocket message parse error:', err);
      }
    };

    return () => {
      try {
        ws.close();
      } catch {
        /* ignore */
      }
      wsRef.current = null;
      setConnected(false);
    };
  }, [topic]);

  return { events, last, connected };
}
