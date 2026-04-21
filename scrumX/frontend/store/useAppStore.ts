import { create } from 'zustand';
import { persist } from 'zustand/middleware';

type AppState = {
  selectedProjectId: string | null;
  issueFilters: {
    status?: string;
    label?: string;
    assignee_id?: string;
  };
  commandPaletteOpen: boolean;
  jqlSearch: string;
  theme: 'light' | 'dark';
  online: boolean;
  pendingActions: number;
  conflictIssueIds: string[];
  setSelectedProject: (id: string | null) => void;
  setIssueFilter: (key: 'status' | 'label' | 'assignee_id', value: string) => void;
  resetIssueFilters: () => void;
  setCommandPaletteOpen: (open: boolean) => void;
  setJqlSearch: (query: string) => void;
  setTheme: (theme: 'light' | 'dark') => void;
  toggleTheme: () => void;
  setOnlineStatus: (online: boolean) => void;
  beginPendingAction: () => void;
  finishPendingAction: () => void;
  registerConflictIssue: (issueId: string) => void;
  clearConflictIssue: (issueId: string) => void;
};

export const useAppStore = create<AppState>()(
  persist(
    (set) => ({
      selectedProjectId: null,
      issueFilters: {},
      commandPaletteOpen: false,
      jqlSearch: '',
      theme: 'light',
      online: true,
      pendingActions: 0,
      conflictIssueIds: [],
      setSelectedProject: (id) => set({ selectedProjectId: id }),
      setIssueFilter: (key, value) =>
        set((state) => ({ issueFilters: { ...state.issueFilters, [key]: value || undefined } })),
      resetIssueFilters: () => set({ issueFilters: {} }),
      setCommandPaletteOpen: (open) => set({ commandPaletteOpen: open }),
      setJqlSearch: (query) => set({ jqlSearch: query }),
      setTheme: (theme) => set({ theme }),
      toggleTheme: () => set((state) => ({ theme: state.theme === 'dark' ? 'light' : 'dark' })),
      setOnlineStatus: (online) => set({ online }),
      beginPendingAction: () => set((state) => ({ pendingActions: state.pendingActions + 1 })),
      finishPendingAction: () => set((state) => ({ pendingActions: Math.max(0, state.pendingActions - 1) })),
      registerConflictIssue: (issueId) =>
        set((state) => ({
          conflictIssueIds: state.conflictIssueIds.includes(issueId)
            ? state.conflictIssueIds
            : [...state.conflictIssueIds, issueId]
        })),
      clearConflictIssue: (issueId) =>
        set((state) => ({ conflictIssueIds: state.conflictIssueIds.filter((value) => value !== issueId) }))
    }),
    {
      name: 'scrumx-app-state',
      partialize: (state) => ({
        selectedProjectId: state.selectedProjectId,
        issueFilters: state.issueFilters,
        jqlSearch: state.jqlSearch,
        theme: state.theme
      })
    }
  )
);
