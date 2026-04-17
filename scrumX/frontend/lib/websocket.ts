import type { ScrumEvent } from '@/types';

type EventHandler = (event: ScrumEvent) => void;

class RealtimeClient {
  private ws: WebSocket | null = null;
  private handlers = new Set<EventHandler>();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private shouldReconnect = false;
  private targetEndpoint: string | null = null;

  connect(accessToken: string, projectId?: string) {
    if (typeof window === 'undefined' || !accessToken) return;
    document.cookie = `ws_access_token=${encodeURIComponent(accessToken)}; Path=/; SameSite=Lax`;
    this.shouldReconnect = true;
    const fallback = (() => {
      const rawApi = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080/api/v1';
      try {
        const url = new URL(rawApi);
        url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
        url.pathname = '/ws';
        url.search = '';
        url.hash = '';
        return url.toString();
      } catch {
        return 'ws://localhost:8080/ws';
      }
    })();
    const endpoint = new URL(process.env.NEXT_PUBLIC_WS_URL ?? fallback);
    if (projectId) {
      endpoint.searchParams.set('project_id', projectId);
    } else {
      endpoint.searchParams.delete('project_id');
    }
    const nextEndpoint = endpoint.toString();
    if (this.ws && this.targetEndpoint === nextEndpoint) return;
    this.targetEndpoint = nextEndpoint;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.close();
      this.ws = null;
    }
    this.connectToTarget();
  }

  private connectToTarget() {
    if (!this.targetEndpoint || typeof window === 'undefined') return;
    this.ws = new WebSocket(this.targetEndpoint);
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
        this.reconnectTimer = setTimeout(() => this.connectToTarget(), 1500);
      }
    };
  }

  disconnect() {
    this.shouldReconnect = false;
    if (typeof window !== 'undefined') {
      document.cookie = 'ws_access_token=; Max-Age=0; Path=/; SameSite=Lax';
    }
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.targetEndpoint = null;
  }

  subscribe(handler: EventHandler) {
    this.handlers.add(handler);
    return () => this.handlers.delete(handler);
  }
}

export const realtimeClient = new RealtimeClient();
