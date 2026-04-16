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
  setSelectedProject: (id: string | null) => void;
  setIssueFilter: (key: 'status' | 'label' | 'assignee_id', value: string) => void;
  resetIssueFilters: () => void;
  setCommandPaletteOpen: (open: boolean) => void;
  setJqlSearch: (query: string) => void;
  setTheme: (theme: 'light' | 'dark') => void;
  toggleTheme: () => void;
};

export const useAppStore = create<AppState>()(
  persist(
    (set) => ({
      selectedProjectId: null,
      issueFilters: {},
      commandPaletteOpen: false,
      jqlSearch: '',
      theme: 'light',
      setSelectedProject: (id) => set({ selectedProjectId: id }),
      setIssueFilter: (key, value) =>
        set((state) => ({ issueFilters: { ...state.issueFilters, [key]: value || undefined } })),
      resetIssueFilters: () => set({ issueFilters: {} }),
      setCommandPaletteOpen: (open) => set({ commandPaletteOpen: open }),
      setJqlSearch: (query) => set({ jqlSearch: query }),
      setTheme: (theme) => set({ theme }),
      toggleTheme: () => set((state) => ({ theme: state.theme === 'dark' ? 'light' : 'dark' }))
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
