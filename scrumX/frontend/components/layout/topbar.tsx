'use client';

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

  return (
    <header className="sticky top-0 z-10 flex h-14 items-center justify-between border-b bg-white px-4">
      <button
        type="button"
        onClick={() => openPalette(true)}
        className="flex w-80 items-center gap-2 rounded-md border px-2"
      >
        <Search className="h-4 w-4 text-muted-foreground" />
        <Input className="border-0 p-0 shadow-none focus-visible:ring-0" readOnly value="Search or jump... (Ctrl+K)" />
      </button>
      <div className="flex items-center gap-2 text-sm">
        <NotificationsPanel />
        <span className="text-muted-foreground">{user?.email ?? 'Guest'}</span>
        {user?.role ? <Badge>{user.role}</Badge> : null}
        <Button variant="outline" size="sm" onClick={logout}>Sign out</Button>
      </div>
    </header>
  );
}
