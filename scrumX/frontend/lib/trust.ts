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
      description: 'Open the issue detail page to compare local and remote changes, then keep local, keep remote, or merge.'
    };
  }
  if (pendingActions > 0) {
    return {
      tone: 'info' as const,
      title: `⟳ Applying ${pendingActions} web change${pendingActions === 1 ? '' : 's'}`,
      description: 'Stay on the trust path while scrumX confirms the latest server state.'
    };
  }
  if (!online) {
    return {
      tone: 'warning' as const,
      title: 'Offline network detected',
      description: 'The web app is in safe mode. Reconnect before editing, or use the scrumX CLI for offline issue capture.'
    };
  }
  return {
    tone: 'success' as const,
    title: '✔ Synced and conflict-safe',
    description: 'Use the issue workspace as the fastest path for create, review, sync, and conflict resolution.'
  };
}
