import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { Tokens, User } from '@/types';

type AuthState = {
  user: User | null;
  tokens: Tokens | null;
  orgId: string | null;
  hydrated: boolean;
  setSession: (user: User, tokens: Tokens) => void;
  setTokens: (tokens: Tokens) => void;
  setUser: (user: User | null) => void;
  clearSession: () => void;
  markHydrated: () => void;
};

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      tokens: null,
      orgId: null,
      hydrated: false,
      setSession: (user, tokens) => set({ user, tokens, orgId: user.org_id }),
      setTokens: (tokens) => set((state) => ({ tokens, orgId: state.orgId ?? state.user?.org_id ?? null })),
      setUser: (user) => set((state) => ({ user, orgId: user?.org_id ?? state.orgId ?? null })),
      clearSession: () => set({ user: null, tokens: null, orgId: null }),
      markHydrated: () => set({ hydrated: true })
    }),
    {
      name: 'scrumx-auth-session',
      partialize: (state) => ({ user: state.user, tokens: state.tokens, orgId: state.orgId }),
      onRehydrateStorage: () => (state) => {
        state?.markHydrated();
      }
    }
  )
);
