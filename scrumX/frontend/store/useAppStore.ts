import { create } from 'zustand';

type AppState = {
  selectedProjectId: string | null;
  issueFilters: {
    status?: string;
    label?: string;
    assignee_id?: string;
  };
  commandPaletteOpen: boolean;
  setSelectedProject: (id: string | null) => void;
  setIssueFilter: (key: 'status' | 'label' | 'assignee_id', value: string) => void;
  resetIssueFilters: () => void;
  setCommandPaletteOpen: (open: boolean) => void;
};

export const useAppStore = create<AppState>((set) => ({
  selectedProjectId: null,
  issueFilters: {},
  commandPaletteOpen: false,
  setSelectedProject: (id) => set({ selectedProjectId: id }),
  setIssueFilter: (key, value) =>
    set((state) => ({ issueFilters: { ...state.issueFilters, [key]: value || undefined } })),
  resetIssueFilters: () => set({ issueFilters: {} }),
  setCommandPaletteOpen: (open) => set({ commandPaletteOpen: open })
}));
