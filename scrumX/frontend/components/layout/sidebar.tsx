'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/utils';
import { useAppStore } from '@/store/useAppStore';

const items = [
  { href: '/projects', label: 'Issues' },
  { href: '/dashboard', label: 'Workspace' },
  { href: '/board/default', label: 'Board' },
  { href: '/settings', label: 'Migration & Settings' },
  { href: '/automation', label: 'Automation (preview)' },
  { href: '/releases', label: 'Releases (preview)' },
  { href: '/incidents', label: 'Incidents (preview)' }
];

export function Sidebar() {
  const pathname = usePathname();
  const conflictIssueIds = useAppStore((state) => state.conflictIssueIds);
  const pendingActions = useAppStore((state) => state.pendingActions);

  return (
    <aside className="w-64 border-r bg-background p-3">
      <div className="mb-1 px-2 text-lg font-semibold">scrumX</div>
      <p className="mb-2 px-2 text-xs text-muted-foreground">Zero data loss with safe offline work, safe sync, and safe conflict resolution.</p>
      <div className="mb-4 space-y-1 px-2 text-[11px] text-muted-foreground">
        <div>Trust state is always visible.</div>
        <div>{conflictIssueIds.length > 0 ? `⚠ ${conflictIssueIds.length} conflict${conflictIssueIds.length === 1 ? '' : 's'} need review` : '✔ No open conflicts'}</div>
        <div>{pendingActions > 0 ? `⟳ ${pendingActions} pending web change${pendingActions === 1 ? '' : 's'}` : '✔ No pending web changes'}</div>
      </div>
      <nav className="space-y-1">
        {items.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            className={cn(
              'block rounded-md px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-foreground',
              pathname === item.href && 'bg-accent text-foreground'
            )}
          >
            <span className="flex items-center justify-between gap-2">
              <span>{item.label}</span>
              {item.href === '/projects' && conflictIssueIds.length > 0 ? (
                <span className="rounded-full border border-amber-200 bg-amber-100 px-2 py-0.5 text-[10px] text-amber-900">
                  {conflictIssueIds.length}
                </span>
              ) : null}
            </span>
          </Link>
        ))}
      </nav>
    </aside>
  );
}
