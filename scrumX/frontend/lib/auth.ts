import { apiRequest } from '@/lib/api';
import { useAuthStore } from '@/store/useAuthStore';
import type { Tokens, User } from '@/types';

type AuthPayload = { user: User; tokens: Tokens };

export async function login(email: string, password: string) {
  const data = await apiRequest<AuthPayload>('/auth/login', {
    method: 'POST',
    skipAuth: true,
    body: JSON.stringify({ email, password })
  });
  useAuthStore.getState().setSession(data.user, data.tokens);
  return data;
}

export async function register(email: string, password: string) {
  const data = await apiRequest<AuthPayload>('/auth/register', {
    method: 'POST',
    skipAuth: true,
    body: JSON.stringify({ email, password })
  });
  useAuthStore.getState().setSession(data.user, data.tokens);
  return data;
}

export function logout() {
  useAuthStore.getState().clearSession();
}
