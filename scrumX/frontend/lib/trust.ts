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
      description: 'Your work is still protected. Compare local and remote changes, then resolve with the recommended action.'
    };
  }
  if (pendingActions > 0) {
    return {
      tone: 'info' as const,
      title: `⟳ Applying ${pendingActions} web change${pendingActions === 1 ? '' : 's'}`,
      description: 'scrumX is holding your latest change safely while sync finishes.'
    };
  }
  if (!online) {
    return {
      tone: 'warning' as const,
      title: 'Offline safe mode',
      description: 'Keep working offline. Sync picks up safely when the network returns.'
    };
  }
  return {
    tone: 'success' as const,
    title: '✔ Synced and protected',
    description: 'Conflict, pending, and offline state stay visible when they matter.'
  };
}
