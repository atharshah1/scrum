'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { Input } from '@/components/ui/input';
import { useAppStore } from '@/store/useAppStore';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { SavedIssueQuery, RecentIssueQuery } from '@/types';

export function CommandPalette() {
  const router = useRouter();
  const isOpen = useAppStore((s) => s.commandPaletteOpen);
  const setOpen = useAppStore((s) => s.setCommandPaletteOpen);
  const selectedProjectId = useAppStore((s) => s.selectedProjectId);
  const setJqlSearch = useAppStore((s) => s.setJqlSearch);
  const [query, setQuery] = useState('');
  const savedQueries = useQuery({
    queryKey: qk.issueSavedQueries,
    queryFn: () => apiRequest<SavedIssueQuery[]>('/issues/queries/saved')
  });
  const recentQueries = useQuery({
    queryKey: qk.issueRecentQueries,
    queryFn: () => apiRequest<RecentIssueQuery[]>('/issues/queries/recent?limit=10')
  });

  useEffect(() => {
    let pendingGo = false;
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setOpen(!isOpen);
      }
      if (!event.ctrlKey && !event.metaKey && event.key.toLowerCase() === 'g') {
        pendingGo = true;
        return;
      }
      if (pendingGo && !event.ctrlKey && !event.metaKey && event.key.toLowerCase() === 'p') {
        event.preventDefault();
        router.push('/projects');
        setOpen(false);
      }
      if (pendingGo && !event.ctrlKey && !event.metaKey && event.key.toLowerCase() === 's') {
        event.preventDefault();
        router.push('/sprint/default');
        setOpen(false);
      }
      pendingGo = false;
      if (event.key === 'Escape') {
        setOpen(false);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [isOpen, router, setOpen]);

  const actions = useMemo(
    () => {
      const staticActions = [
        { id: 'create-issue', label: 'Create Issue', action: () => router.push('/projects') },
        { id: 'move-issue', label: 'Move Issue', action: () => router.push('/projects') },
        { id: 'assign-issue', label: 'Assign Issue', action: () => router.push('/projects') },
        { id: 'open-project', label: 'Open Project', action: () => router.push(selectedProjectId ? `/projects/${selectedProjectId}` : '/projects') },
        { id: 'open-board', label: 'Open Board', action: () => router.push(selectedProjectId ? `/board/${selectedProjectId}` : '/board/default') },
        { id: 'open-dashboard', label: 'Open Dashboard', action: () => router.push('/dashboard') }
      ];
      const dynamicSaved = (savedQueries.data ?? []).slice(0, 8).map((item) => ({
        id: `saved-${item.id}`,
        label: `Saved: ${item.name}`,
        action: () => {
          setJqlSearch(item.query);
          router.push('/projects');
        }
      }));
      const dynamicRecent = (recentQueries.data ?? []).slice(0, 8).map((item, i) => ({
        id: `recent-${i}`,
        label: `Recent: ${item.query}`,
        action: () => {
          setJqlSearch(item.query);
          router.push('/projects');
        }
      }));
      const searchAction = query.trim()
        ? [{
            id: 'search-issues',
            label: `Search issues: ${query.trim()}`,
            action: () => {
              setJqlSearch(query.trim());
              router.push('/projects');
            }
          }]
        : [];
      return [...searchAction, ...dynamicSaved, ...dynamicRecent, ...staticActions];
    },
    [query, recentQueries.data, router, savedQueries.data, selectedProjectId, setJqlSearch]
  );

  const filteredActions = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return actions;
    return actions.filter((item) => item.label.toLowerCase().includes(q));
  }, [actions, query]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/30 pt-20" onClick={() => setOpen(false)}>
      <div className="w-[560px] rounded-lg border bg-white p-2 shadow-xl" onClick={(e) => e.stopPropagation()}>
        <Input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          className="mb-2"
          placeholder="Search commands..."
          autoFocus
        />
        {filteredActions.map((action) => (
          <button
            key={action.id}
            type="button"
            className="w-full rounded-md px-3 py-2 text-left text-sm hover:bg-accent"
            onClick={() => {
              action.action();
              setOpen(false);
            }}
          >
            <span>{action.label}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
