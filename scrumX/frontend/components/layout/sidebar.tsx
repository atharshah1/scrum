'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/utils';

const items = [
  { href: '/dashboard', label: 'Dashboard' },
  { href: '/projects', label: 'Projects' },
  { href: '/board/default', label: 'Board' },
  { href: '/automation', label: 'Automation' },
  { href: '/releases', label: 'Releases' },
  { href: '/incidents', label: 'Incidents' },
  { href: '/settings', label: 'Settings' }
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="w-64 border-r bg-white p-3">
      <div className="mb-4 px-2 text-lg font-semibold">scrumX</div>
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
