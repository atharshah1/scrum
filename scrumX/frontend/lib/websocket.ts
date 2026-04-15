import type { ScrumEvent } from '@/types';

type EventHandler = (event: ScrumEvent) => void;

class RealtimeClient {
  private ws: WebSocket | null = null;
  private handlers = new Set<EventHandler>();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private shouldReconnect = false;

  connect() {
    if (typeof window === 'undefined' || this.ws) return;
    this.shouldReconnect = true;
    const fallback = (process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080/api/v1').replace('/api/v1', '/ws').replace('http', 'ws');
    const endpoint = process.env.NEXT_PUBLIC_WS_URL ?? fallback;

    this.ws = new WebSocket(endpoint);
    this.ws.onmessage = (message) => {
      try {
        const event = JSON.parse(message.data) as ScrumEvent;
        this.handlers.forEach((handler) => handler(event));
      } catch {
        // noop
      }
    };
    this.ws.onclose = () => {
      this.ws = null;
      if (this.shouldReconnect) {
        this.reconnectTimer = setTimeout(() => this.connect(), 1500);
      }
    };
  }

  disconnect() {
    this.shouldReconnect = false;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  subscribe(handler: EventHandler) {
    this.handlers.add(handler);
    return () => this.handlers.delete(handler);
  }
}

export const realtimeClient = new RealtimeClient();
