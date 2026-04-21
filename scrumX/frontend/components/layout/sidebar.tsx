'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/utils';

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

  return (
    <aside className="w-64 border-r bg-background p-3">
      <div className="mb-1 px-2 text-lg font-semibold">scrumX</div>
      <p className="mb-4 px-2 text-xs text-muted-foreground">Fast task work with offline safety and conflict protection.</p>
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
            {item.label}
          </Link>
        ))}
      </nav>
    </aside>
  );
}
