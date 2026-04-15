'use client';

import { useMemo } from 'react';
import { Search } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { useAuthStore } from '@/store/useAuthStore';
import { useAppStore } from '@/store/useAppStore';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { logout } from '@/lib/auth';
import { NotificationsPanel } from '@/components/layout/notifications-panel';

export function Topbar() {
  const user = useAuthStore((s) => s.user);
  const openPalette = useAppStore((s) => s.setCommandPaletteOpen);
  const jqlSearch = useAppStore((s) => s.jqlSearch);
  const setJqlSearch = useAppStore((s) => s.setJqlSearch);
  const suggestions = useMemo(() => {
    const query = jqlSearch.trim().toLowerCase();
    if (query.endsWith('status=')) return ['todo', 'in_progress', 'done'];
    if (query.endsWith('assignee=')) return ['me', 'mine', 'user1', 'user2'];
    if (query.endsWith('priority=')) return ['low', 'medium', 'high', 'critical'];
    if (query.endsWith('type=')) return ['task', 'story', 'bug', 'epic'];
    if (query.endsWith('label=')) return ['payments', 'backend', 'frontend', 'infra'];
    return ['status=', 'assignee=', 'priority=', 'label=', 'sprint=', 'type=', 'status=done AND assignee=me'];
  }, [jqlSearch]);

  return (
    <header className="sticky top-0 z-10 flex h-14 items-center justify-between border-b bg-white px-4">
      <div className="flex w-[34rem] items-center gap-2 rounded-md border px-2">
        <Search className="h-4 w-4 text-muted-foreground" />
        <Input
          className="border-0 p-0 shadow-none focus-visible:ring-0"
          value={jqlSearch}
          list="jql-suggestions"
          onChange={(event) => setJqlSearch(event.target.value)}
          placeholder='JQL search (e.g. status=done AND assignee=me)'
        />
        <datalist id="jql-suggestions">
          {suggestions.map((item) => (
            <option key={item} value={item} />
          ))}
        </datalist>
        <Button type="button" variant="outline" size="sm" onClick={() => openPalette(true)}>
          Ctrl+K
        </Button>
      </div>
      <div className="flex items-center gap-2 text-sm">
        <NotificationsPanel />
        <span className="text-muted-foreground">{user?.email ?? 'Guest'}</span>
        {user?.role ? <Badge>{user.role}</Badge> : null}
        <Button variant="outline" size="sm" onClick={logout}>Sign out</Button>
      </div>
    </header>
  );
}
