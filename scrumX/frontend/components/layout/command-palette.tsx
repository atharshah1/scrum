'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAppStore } from '@/store/useAppStore';

const actions = [
  { id: 'create-issue', label: 'Create issue', href: '/projects' },
  { id: 'jump-project', label: 'Jump to project', href: '/projects' },
  { id: 'start-sprint', label: 'Start sprint', href: '/sprint/default' }
];

export function CommandPalette() {
  const router = useRouter();
  const isOpen = useAppStore((s) => s.commandPaletteOpen);
  const setOpen = useAppStore((s) => s.setCommandPaletteOpen);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setOpen(!isOpen);
      }
      if (event.key === 'Escape') {
        setOpen(false);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [isOpen, setOpen]);

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
            {action.label}
          </button>
        ))}
      </div>
    </div>
  );
}
