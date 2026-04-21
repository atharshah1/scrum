export function describeTrustState({
  online,
  pendingActions,
  conflictIssueIds
}: {
  online: boolean;
  pendingActions: number;
  conflictIssueIds: string[];
}) {
  if (conflictIssueIds.length > 0) {
    return {
      tone: 'warning' as const,
      title: `⚠ Conflict detected on ${conflictIssueIds.length} issue${conflictIssueIds.length === 1 ? '' : 's'}`,
      description: 'Zero data loss is still intact. Compare local and remote changes, then resolve safely in one click.'
    };
  }
  if (pendingActions > 0) {
    return {
      tone: 'info' as const,
      title: `⟳ Applying ${pendingActions} web change${pendingActions === 1 ? '' : 's'}`,
      description: 'Zero data loss stays on. scrumX is syncing your latest change safely.'
    };
  }
  if (!online) {
    return {
      tone: 'warning' as const,
      title: 'Offline safe mode',
      description: 'Zero data loss stays on while offline. Keep working, then sync safely when the network returns.'
    };
  }
  return {
    tone: 'success' as const,
    title: '✔ Synced with zero data loss',
    description: 'Safe offline, safe sync, and safe conflict resolution stay visible everywhere in scrumX.'
  };
}
