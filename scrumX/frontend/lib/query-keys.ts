export const qk = {
  projects: ['projects'] as const,
  issues: (filters?: string) => ['issues', filters ?? 'all'] as const,
  issue: (id: string) => ['issue', id] as const,
  comments: (id: string) => ['issue-comments', id] as const,
  activities: (id: string) => ['issue-activities', id] as const,
  issueSummary: (id: string) => ['issue-summary', id] as const,
  issueSuggestion: (id: string) => ['issue-suggestion', id] as const,
  insightsStuck: (scope = 'all') => ['insights-stuck', scope] as const,
  insightsBottleneck: (scope = 'all') => ['insights-bottleneck', scope] as const,
  insightsVelocity: (scope = 'all') => ['insights-velocity', scope] as const,
  insightsCycleTime: (scope = 'all') => ['insights-cycle-time', scope] as const,
  board: (id: string) => ['board', id] as const,
  workflowTransitions: (projectId: string) => ['workflow-transitions', projectId] as const,
  automationRules: ['automation-rules'] as const,
  releases: ['releases'] as const,
  incidents: ['incidents'] as const,
  notifications: (scope = 'all') => ['notifications', scope] as const
};
