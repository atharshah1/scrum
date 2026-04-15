'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAppStore } from '@/store/useAppStore';

const actions = [
  { id: 'create-issue', label: 'Create issue', href: '/projects', shortcut: 'G then P' },
  { id: 'jump-project', label: 'Jump to project', href: '/projects', shortcut: 'Ctrl/Cmd+K' },
  { id: 'start-sprint', label: 'Start sprint', href: '/sprint/default', shortcut: 'G then S' }
];

export function CommandPalette() {
  const router = useRouter();
  const isOpen = useAppStore((s) => s.commandPaletteOpen);
  const setOpen = useAppStore((s) => s.setCommandPaletteOpen);

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

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/30 pt-20" onClick={() => setOpen(false)}>
      <div className="w-[560px] rounded-lg border bg-white p-2 shadow-xl" onClick={(e) => e.stopPropagation()}>
        {actions.map((action) => (
          <button
            key={action.id}
            type="button"
            className="w-full rounded-md px-3 py-2 text-left text-sm hover:bg-accent"
            onClick={() => {
              router.push(action.href);
              setOpen(false);
            }}
          >
            <span>{action.label}</span>
            <kbd className="rounded border bg-muted px-1 py-0.5 text-[10px] text-muted-foreground">{action.shortcut}</kbd>
          </button>
        ))}
      </div>
    </div>
  );
}
