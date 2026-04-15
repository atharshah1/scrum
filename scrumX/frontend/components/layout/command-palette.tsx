'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Input } from '@/components/ui/input';
import { useAppStore } from '@/store/useAppStore';

export function CommandPalette() {
  const router = useRouter();
  const isOpen = useAppStore((s) => s.commandPaletteOpen);
  const setOpen = useAppStore((s) => s.setCommandPaletteOpen);
  const selectedProjectId = useAppStore((s) => s.selectedProjectId);
  const [query, setQuery] = useState('');

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
    () => [
      { id: 'create-issue', label: 'Create Issue', action: () => router.push('/projects') },
      { id: 'move-issue', label: 'Move Issue', action: () => router.push('/projects') },
      { id: 'assign-issue', label: 'Assign Issue', action: () => router.push('/projects') },
      { id: 'open-project', label: 'Open Project', action: () => router.push(selectedProjectId ? `/projects/${selectedProjectId}` : '/projects') },
      { id: 'open-board', label: 'Open Board', action: () => router.push(selectedProjectId ? `/board/${selectedProjectId}` : '/board/default') },
      { id: 'open-dashboard', label: 'Open Dashboard', action: () => router.push('/dashboard') }
    ],
    [router, selectedProjectId]
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
