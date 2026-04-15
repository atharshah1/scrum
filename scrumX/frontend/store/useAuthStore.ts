import { create } from 'zustand';
import type { Tokens, User } from '@/types';

type AuthState = {
  user: User | null;
  tokens: Tokens | null;
  orgId: string | null;
  setSession: (user: User, tokens: Tokens) => void;
  setTokens: (tokens: Tokens) => void;
  clearSession: () => void;
};

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  tokens: null,
  orgId: null,
  setSession: (user, tokens) => set({ user, tokens, orgId: user.org_id }),
  setTokens: (tokens) => set((state) => ({ tokens, orgId: state.orgId ?? state.user?.org_id ?? null })),
  clearSession: () => set({ user: null, tokens: null, orgId: null })
}));
