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
  const conflictIssueIds = useAppStore((s) => s.conflictIssueIds);
  const pendingActions = useAppStore((s) => s.pendingActions);
  const [query, setQuery] = useState('');
  const [selectedIndex, setSelectedIndex] = useState(0);
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
      const target = event.target as HTMLElement | null;
      const isTypingTarget =
        target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.isContentEditable;

      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setOpen(!isOpen);
        return;
      }

      if (event.key === 'Escape') {
        setOpen(false);
        pendingGo = false;
        return;
      }

      if (isTypingTarget || event.ctrlKey || event.metaKey || event.altKey) {
        pendingGo = false;
        return;
      }

      if (event.key === '/') {
        event.preventDefault();
        document.querySelector<HTMLInputElement>('[data-topbar-search="true"]')?.focus();
        pendingGo = false;
        return;
      }

      if (event.key.toLowerCase() === 'c') {
        event.preventDefault();
        router.push('/projects?focus=create');
        setOpen(false);
        pendingGo = false;
        return;
      }

      if (event.key.toLowerCase() === 't') {
        event.preventDefault();
        router.push(conflictIssueIds[0] ? `/issues/${conflictIssueIds[0]}` : '/dashboard');
        setOpen(false);
        pendingGo = false;
        return;
      }

      if (event.key.toLowerCase() === 'g') {
        pendingGo = true;
        return;
      }

      if (pendingGo && event.key.toLowerCase() === 'i') {
        event.preventDefault();
        router.push('/projects');
        setOpen(false);
        pendingGo = false;
        return;
      }

      if (pendingGo && event.key.toLowerCase() === 'w') {
        event.preventDefault();
        router.push('/dashboard');
        setOpen(false);
        pendingGo = false;
        return;
      }

      if (pendingGo && event.key.toLowerCase() === 'b') {
        event.preventDefault();
        router.push(selectedProjectId ? `/board/${selectedProjectId}` : '/board/default');
        setOpen(false);
        pendingGo = false;
        return;
      }

      if (pendingGo && event.key.toLowerCase() === 's') {
        event.preventDefault();
        router.push('/settings');
        setOpen(false);
      }

      pendingGo = false;
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [conflictIssueIds, isOpen, router, selectedProjectId, setOpen]);

  useEffect(() => {
    if (!isOpen) {
      setQuery('');
    }
    setSelectedIndex(0);
  }, [isOpen, query]);

  const actions = useMemo(() => {
    const staticActions = [
      { id: 'create-issue', label: 'Create issue (C)', action: () => router.push('/projects?focus=create') },
      { id: 'open-issues', label: 'Open issue list (G I)', action: () => router.push('/projects') },
      { id: 'open-workspace', label: 'Open workspace (G W)', action: () => router.push('/dashboard') },
      { id: 'open-board', label: 'Open board (G B)', action: () => router.push(selectedProjectId ? `/board/${selectedProjectId}` : '/board/default') },
      { id: 'open-migration', label: 'Open migration flow (G S)', action: () => router.push('/settings') },
      { id: 'focus-search', label: 'Focus issue filter (/)', action: () => document.querySelector<HTMLInputElement>('[data-topbar-search="true"]')?.focus() }
    ];
    const trustActions = [
      ...(pendingActions > 0
        ? [{
            id: 'trust-pending',
            label: `Review ${pendingActions} safe pending change${pendingActions === 1 ? '' : 's'}`,
            action: () => router.push('/dashboard')
          }]
        : []),
      ...(conflictIssueIds.length > 0
        ? [{
            id: 'trust-first-conflict',
            label: 'Resolve first conflict safely (T)',
            action: () => router.push(`/issues/${conflictIssueIds[0]}`)
          }]
        : []),
      ...conflictIssueIds.map((issueId) => ({
        id: `conflict-${issueId}`,
        label: `Resolve conflict on issue ${issueId}`,
        action: () => router.push(`/issues/${issueId}`)
      }))
    ];
    const dynamicSaved = (savedQueries.data ?? []).slice(0, 8).map((item) => ({
      id: `saved-${item.id}`,
      label: `Saved filter: ${item.name}`,
      action: () => {
        setJqlSearch(item.query);
        router.push('/projects');
      }
    }));
    const dynamicRecent = (recentQueries.data ?? []).slice(0, 8).map((item, i) => ({
      id: `recent-${i}`,
      label: `Recent filter: ${item.query}`,
      action: () => {
        setJqlSearch(item.query);
        router.push('/projects');
      }
    }));
    const searchAction = query.trim()
      ? [{
          id: 'search-issues',
          label: `Filter issues: ${query.trim()}`,
          action: () => {
            setJqlSearch(query.trim());
            router.push('/projects');
          }
        }]
      : [];
    return [...trustActions, ...searchAction, ...dynamicSaved, ...dynamicRecent, ...staticActions];
  }, [conflictIssueIds, pendingActions, query, recentQueries.data, router, savedQueries.data, selectedProjectId, setJqlSearch]);

  const filteredActions = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return actions;
    return actions.filter((item) => item.label.toLowerCase().includes(q));
  }, [actions, query]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/30 pt-20" onClick={() => setOpen(false)}>
      <div className="w-[560px] rounded-lg border bg-background p-2 shadow-xl" onClick={(e) => e.stopPropagation()}>
        <Input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            if (!filteredActions.length) return;
            if (event.key === 'ArrowDown') {
              event.preventDefault();
              setSelectedIndex((index) => (index + 1) % filteredActions.length);
            }
            if (event.key === 'ArrowUp') {
              event.preventDefault();
              setSelectedIndex((index) => (index - 1 + filteredActions.length) % filteredActions.length);
            }
            if (event.key === 'Enter') {
              event.preventDefault();
              filteredActions[selectedIndex]?.action();
              setOpen(false);
            }
          }}
          className="mb-2"
          placeholder="Find commands, filters, and navigation..."
          autoFocus
        />
        {filteredActions.map((action, index) => (
          <button
            key={action.id}
            type="button"
            className={`w-full rounded-md px-3 py-2 text-left text-sm hover:bg-accent ${index === selectedIndex ? 'bg-accent' : ''}`}
            onClick={() => {
              action.action();
              setOpen(false);
            }}
            onMouseEnter={() => setSelectedIndex(index)}
          >
            <span>{action.label}</span>
          </button>
        ))}
        <div className="px-3 py-2 text-xs text-muted-foreground">
          <div>⌘/Ctrl+K open | G I/W/B/S navigate | C create</div>
          <div>/ focus filter | T open trust action | ↑/↓ then Enter to run</div>
        </div>
      </div>
    </div>
  );
}
