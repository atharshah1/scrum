'use client';

import Link from 'next/link';
import { useEffect } from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { describeTrustState } from '@/lib/trust';
import { useAppStore } from '@/store/useAppStore';

const toneClassName = {
  success: 'border-emerald-200 bg-emerald-50/80 text-emerald-900',
  info: 'border-blue-200 bg-blue-50/80 text-blue-900',
  warning: 'border-amber-200 bg-amber-50/80 text-amber-950'
};

export function TrustBar() {
  const online = useAppStore((state) => state.online);
  const pendingActions = useAppStore((state) => state.pendingActions);
  const conflictIssueIds = useAppStore((state) => state.conflictIssueIds);
  const setOnlineStatus = useAppStore((state) => state.setOnlineStatus);
  const trust = describeTrustState({ online, pendingActions, conflictIssueIds });
  const firstConflictIssueId = conflictIssueIds[0];

  useEffect(() => {
    const updateOnlineStatus = () => setOnlineStatus(window.navigator.onLine);
    updateOnlineStatus();
    window.addEventListener('online', updateOnlineStatus);
    window.addEventListener('offline', updateOnlineStatus);
    return () => {
      window.removeEventListener('online', updateOnlineStatus);
      window.removeEventListener('offline', updateOnlineStatus);
    };
  }, [setOnlineStatus]);

  return (
    <div className={`border-b px-4 py-2 text-sm ${toneClassName[trust.tone]}`}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <Badge className="border-white/60 bg-white/70 text-current">Trust path</Badge>
            <span className="font-medium">{trust.title}</span>
          </div>
          <p className="text-current/80">{trust.description}</p>
          <p className="text-xs font-medium uppercase tracking-wide text-current/70">Zero data loss • safe offline • safe sync • safe conflict resolution</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Link href="/projects">
            <Button size="sm" variant="outline">Issue workspace</Button>
          </Link>
          <Link href="/dashboard">
            <Button size="sm" variant="outline">Start instant demo</Button>
          </Link>
          {firstConflictIssueId ? (
            <Link href={`/issues/${firstConflictIssueId}`}>
              <Button size="sm">Resolve with recommended action</Button>
            </Link>
          ) : (
            <Link href="/projects?demo=instant">
              <Button size="sm">Open demo project</Button>
            </Link>
          )}
        </div>
      </div>
    </div>
  );
}
