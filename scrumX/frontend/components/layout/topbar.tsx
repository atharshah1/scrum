'use client';

import { useEffect, useMemo, useState } from 'react';
import { Search, Moon, ShieldCheck, Sun, Wifi, WifiOff } from 'lucide-react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Badge } from '@/components/ui/badge';
import { useAuthStore } from '@/store/useAuthStore';
import { useAppStore } from '@/store/useAppStore';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { logout } from '@/lib/auth';
import { NotificationsPanel } from '@/components/layout/notifications-panel';
import { qk } from '@/lib/query-keys';
import { apiRequest } from '@/lib/api';
import type { IssueSearchSuggestions } from '@/types';
import { toast } from '@/components/ui/toast';

function scoreSuggestion(item: string, token: string): number | null {
  const normalizedToken = token.trim().toLowerCase();
  if (!normalizedToken) return 0;
  const value = item.toLowerCase();
  if (value.startsWith(normalizedToken)) return 0;
  const containsAt = value.indexOf(normalizedToken);
  if (containsAt >= 0) return 100 + containsAt;

  let cursor = 0;
  let score = 200;
  for (const ch of normalizedToken) {
    const idx = value.indexOf(ch, cursor);
    if (idx === -1) return null;
    score += idx - cursor;
    cursor = idx + 1;
  }
  score += value.length - normalizedToken.length;
  return score;
}

export function Topbar() {
  const queryClient = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const openPalette = useAppStore((s) => s.setCommandPaletteOpen);
  const jqlSearch = useAppStore((s) => s.jqlSearch);
  const setJqlSearch = useAppStore((s) => s.setJqlSearch);
  const toggleTheme = useAppStore((s) => s.toggleTheme);
  const theme = useAppStore((s) => s.theme);
  const [highlighted, setHighlighted] = useState(0);
  const [showMenu, setShowMenu] = useState(false);
  const [online, setOnline] = useState(true);
  const [shortcutLabel, setShortcutLabel] = useState('⌘K');

  useEffect(() => {
    const updateOnline = () => setOnline(window.navigator.onLine);
    updateOnline();
    setShortcutLabel(/mac|iphone|ipad|ipod/i.test(window.navigator.platform) ? '⌘K' : 'Ctrl+K');
    window.addEventListener('online', updateOnline);
    window.addEventListener('offline', updateOnline);
    return () => {
      window.removeEventListener('online', updateOnline);
      window.removeEventListener('offline', updateOnline);
    };
  }, []);

  const suggestionsQuery = useQuery({
    queryKey: qk.issueSearchSuggestions,
    queryFn: () => apiRequest<IssueSearchSuggestions>('/issues/search/suggestions')
  });
  const saveQueryMutation = useMutation({
    mutationFn: async ({ name, query }: { name: string; query: string }) =>
      apiRequest('/issues/queries/saved', { method: 'POST', body: JSON.stringify({ name, query }) }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: qk.issueSearchSuggestions, exact: true });
      queryClient.invalidateQueries({ queryKey: qk.issueSavedQueries, exact: true });
      toast({ title: 'Saved filter created', variant: 'success' });
    },
    onError: (error) => {
      toast({
        title: 'Unable to save filter',
        description: error instanceof Error ? error.message : 'Try again with a valid query.',
        variant: 'error'
      });
    }
  });
  const suggestions = useMemo(() => {
    const dynamic = suggestionsQuery.data;
    const query = jqlSearch.trim();
    const queryLower = query.toLowerCase();
    const lastToken = queryLower.split(/\s+/).pop() ?? '';
    if (queryLower.endsWith('status=')) return dynamic?.statuses ?? ['todo', 'in_progress', 'done'];
    if (queryLower.endsWith('assignee=')) {
      const users = dynamic?.assignees.map((a) => a.id);
      return ['me', 'mine', ...(users ?? [])];
    }
    if (queryLower.endsWith('priority=')) return dynamic?.priorities ?? ['low', 'medium', 'high', 'critical'];
    if (queryLower.endsWith('type=')) return dynamic?.types ?? ['task', 'story', 'bug', 'epic'];
    if (queryLower.endsWith('label=')) return dynamic?.labels ?? [];
    const baseSuggestions = [
      ...(dynamic?.fields ?? ['status', 'assignee', 'priority', 'label', 'sprint', 'type', 'title', 'project']).map((f) => `${f}=`),
      'title~"payment bug"',
      'status=done AND assignee=me'
    ];
    if (!lastToken) {
      return baseSuggestions;
    }
    const ranked = baseSuggestions
      .map((item) => ({ item, score: scoreSuggestion(item, lastToken) }))
      .filter((item): item is { item: string; score: number } => item.score !== null)
      .sort((a, b) => a.score - b.score || a.item.localeCompare(b.item))
      .map((item) => item.item);
    return ranked.length ? ranked : baseSuggestions;
  }, [jqlSearch, suggestionsQuery.data]);
  const savedQueries = suggestionsQuery.data?.saved ?? [];
  const recentQueries = suggestionsQuery.data?.recent ?? [];

  return (
    <header className="sticky top-0 z-10 flex h-14 items-center justify-between border-b bg-background px-4">
      <div className="relative flex w-[42rem] items-center gap-2 rounded-md border px-2">
        <Search className="h-4 w-4 text-muted-foreground" />
        <Input
          className="border-0 p-0 shadow-none focus-visible:ring-0"
          value={jqlSearch}
          onFocus={() => setShowMenu(true)}
          onBlur={() => setTimeout(() => setShowMenu(false), 120)}
          onKeyDown={(event) => {
            if (!showMenu || suggestions.length === 0) return;
            if (event.key === 'ArrowDown') {
              event.preventDefault();
              setHighlighted((v) => (v + 1) % suggestions.length);
            }
            if (event.key === 'ArrowUp') {
              event.preventDefault();
              setHighlighted((v) => (v - 1 + suggestions.length) % suggestions.length);
            }
            if (event.key === 'Enter' && suggestions[highlighted]) {
              event.preventDefault();
              setJqlSearch(suggestions[highlighted]);
              setShowMenu(false);
            }
          }}
          onChange={(event) => setJqlSearch(event.target.value)}
          placeholder="Task filter (e.g. status=done AND assignee=me)"
        />
        <select
          className="max-w-40 rounded border bg-transparent px-1 py-1 text-xs"
          value=""
          onChange={(event) => {
            if (event.target.value) setJqlSearch(event.target.value);
          }}
        >
          <option value="">Saved</option>
          {savedQueries.map((saved) => (
            <option key={saved.id} value={saved.query}>
              {saved.name}
            </option>
          ))}
        </select>
        <select
          className="max-w-40 rounded border bg-transparent px-1 py-1 text-xs"
          value=""
          onChange={(event) => {
            if (event.target.value) setJqlSearch(event.target.value);
          }}
        >
          <option value="">Recent</option>
          {recentQueries.map((recent) => (
            <option key={recent.query} value={recent.query}>
              {recent.query}
            </option>
          ))}
        </select>
        <Button type="button" variant="outline" size="sm" onClick={() => openPalette(true)}>
          {shortcutLabel}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={saveQueryMutation.isPending || !jqlSearch.trim()}
          onClick={() => {
            const query = jqlSearch.trim();
            if (!query) return;
            const defaultName = `Filter ${new Date().toISOString().slice(0, 16).replace('T', ' ')}`;
            const name = window.prompt('Saved filter name', defaultName)?.trim();
            if (!name) return;
            saveQueryMutation.mutate({ name, query });
          }}
        >
          Save
        </Button>
        {showMenu && suggestions.length > 0 ? (
          <div className="absolute left-2 right-2 top-12 z-20 max-h-56 overflow-auto rounded-md border bg-background shadow-lg">
            {suggestions.slice(0, 20).map((item, index) => (
              <button
                key={item}
                type="button"
                className={`block w-full px-3 py-2 text-left text-xs ${index === highlighted ? 'bg-accent' : ''}`}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => {
                  setJqlSearch(item);
                  setShowMenu(false);
                }}
              >
                {item}
              </button>
            ))}
          </div>
        ) : null}
      </div>
      <div className="flex items-center gap-2 text-sm">
        <Badge className={online ? 'border-emerald-200 text-emerald-700' : 'border-amber-200 text-amber-700'}>
          {online ? <Wifi className="mr-1 h-3.5 w-3.5" /> : <WifiOff className="mr-1 h-3.5 w-3.5" />}
          {online ? 'Synced web workspace' : 'Offline-safe browser session'}
        </Badge>
        <Badge className="border-blue-200 text-blue-700">
          <ShieldCheck className="mr-1 h-3.5 w-3.5" />
          Conflict-safe edits
        </Badge>
        <Button variant="outline" size="sm" onClick={toggleTheme} aria-label="Toggle theme">
          {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
        </Button>
        <NotificationsPanel />
        <span className="text-muted-foreground">{user?.email ?? 'Guest'}</span>
        {user?.role ? <Badge>{user.role}</Badge> : null}
        <Button variant="outline" size="sm" onClick={logout}>Sign out</Button>
      </div>
    </header>
  );
}
