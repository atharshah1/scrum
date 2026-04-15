import { useAuthStore } from '@/store/useAuthStore';
import type { ApiResponse, Tokens } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080/api/v1';

type RequestOptions = RequestInit & { skipAuth?: boolean; retry?: number };

async function parseResponse<T>(res: Response): Promise<T> {
  if (res.status === 204) {
    return undefined as T;
  }
  const json = (await res.json()) as ApiResponse<T> | T;
  if (typeof json === 'object' && json !== null && 'success' in json) {
    const wrapped = json as ApiResponse<T>;
    if (!wrapped.success) {
      throw new Error(wrapped.error?.message ?? `API error (${res.status})`);
    }
    return wrapped.data;
  }
  return json as T;
}

async function refreshTokens(): Promise<Tokens | null> {
  const store = useAuthStore.getState();
  if (!store.tokens?.refresh_token) return null;

  const res = await fetch(`${API_BASE}/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: store.tokens.refresh_token })
  });

  if (!res.ok) {
    store.clearSession();
    return null;
  }

  const tokens = await parseResponse<Tokens>(res);
  store.setTokens(tokens);
  return tokens;
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { skipAuth, retry = 2, headers, ...rest } = options;
  let attempt = 0;

  while (attempt <= retry) {
    const state = useAuthStore.getState();
    const accessToken = state.tokens?.access_token;
    const orgId = state.orgId;

    const requestHeaders = new Headers(headers ?? {});
    if (!requestHeaders.has('Content-Type') && rest.body) {
      requestHeaders.set('Content-Type', 'application/json');
    }
    if (!skipAuth && accessToken) {
      requestHeaders.set('Authorization', `Bearer ${accessToken}`);
    }
    if (!skipAuth && orgId) {
      requestHeaders.set('X-Org-ID', orgId);
    }

    try {
      const res = await fetch(`${API_BASE}${path}`, { ...rest, headers: requestHeaders, cache: 'no-store' });

      if (res.status === 401 && !skipAuth) {
        const tokens = await refreshTokens();
        if (tokens && attempt < retry) {
          attempt += 1;
          continue;
        }
      }

      if (!res.ok) {
        const message = await res.text();
        throw new Error(message || `Request failed with status ${res.status}`);
      }

      return await parseResponse<T>(res);
    } catch (error) {
      if (attempt >= retry) {
        throw error;
      }
      await new Promise((resolve) => setTimeout(resolve, 150 * 2 ** attempt));
      attempt += 1;
    }
  }

  throw new Error('Request failed');
}
