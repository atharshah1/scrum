export const qk = {
  projects: ['projects'] as const,
  issues: (filters?: string) => ['issues', filters ?? 'all'] as const,
  issue: (id: string) => ['issue', id] as const,
  comments: (id: string) => ['issue-comments', id] as const,
  activities: (id: string) => ['issue-activities', id] as const,
  board: (id: string) => ['board', id] as const,
  automationRules: ['automation-rules'] as const,
  releases: ['releases'] as const,
  incidents: ['incidents'] as const
};
