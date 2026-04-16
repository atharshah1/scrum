'use client';

import { useEffect } from 'react';
import { useAuthStore } from '@/store/useAuthStore';
import { apiRequest } from '@/lib/api';
import type { User } from '@/types';

export function AuthBootstrap() {
  const tokens = useAuthStore((s) => s.tokens);
  const hydrated = useAuthStore((s) => s.hydrated);

  useEffect(() => {
    if (!hydrated || !tokens?.access_token) return;

    const store = useAuthStore.getState();
    if (store.user) return;

    apiRequest<User>('/auth/me')
      .then((user) => {
        useAuthStore.getState().setUser(user);
      })
      .catch(() => {
        useAuthStore.getState().clearSession();
      });
  }, [hydrated, tokens?.access_token]);

  return null;
}
