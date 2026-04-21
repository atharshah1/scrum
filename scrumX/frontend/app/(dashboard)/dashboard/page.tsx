'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Skeleton } from '@/components/ui/skeleton';
import { apiRequest } from '@/lib/api';
import { comingSoonContent, features } from '@/lib/features';
import { qk } from '@/lib/query-keys';
import type { BottleneckInsight, Issue, VelocityInsight } from '@/types';

export default function DashboardPage() {
  const issuesQuery = useQuery({
    queryKey: qk.issues('dashboard'),
    queryFn: () => apiRequest<Issue[]>('/issues?limit=20')
  });
  const velocityQuery = useQuery({
    queryKey: qk.insightsVelocity('dashboard'),
    enabled: features.INSIGHTS,
    queryFn: () => apiRequest<VelocityInsight>('/insights/velocity')
  });
  const bottleneckQuery = useQuery({
    queryKey: qk.insightsBottleneck('dashboard'),
    enabled: features.INSIGHTS,
    queryFn: () => apiRequest<BottleneckInsight>('/insights/bottlenecks')
  });

  const issues = issuesQuery.data ?? [];
  const active = issues.filter((i) => i.status !== 'done').length;
  const done = issues.filter((i) => i.status === 'done').length;

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl font-semibold">Workspace</h1>
        <p className="text-sm text-muted-foreground">Speed and safety first: issue flow, sync trust, and conflict awareness.</p>
      </div>

      <div className="grid gap-4 md:grid-cols-4">
        {issuesQuery.isPending ? (
          <>
            <Skeleton className="h-24" />
            <Skeleton className="h-24" />
            <Skeleton className="h-24" />
            <Skeleton className="h-24" />
          </>
        ) : (
          <>
            <Summary title="Open work" value={active} helper="Fast triage first" />
            <Summary title="Completed" value={done} helper="Closed safely" />
            <Summary title="Recent issues" value={issues.length} helper="Visible in one place" />
            <Summary title="Assigned" value={issues.filter((i) => !!i.assignee_id).length} helper="Developer context" />
          </>
        )}
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader><CardTitle>Sync trust</CardTitle></CardHeader>
          <CardContent className="space-y-2 text-sm text-muted-foreground">
            <p>✔ Optimistic updates keep the UI fast.</p>
            <p>✔ Conflict checks protect concurrent edits.</p>
            <p>✔ Issue detail exposes a dedicated conflict review section when edits collide.</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>Migration flow</CardTitle></CardHeader>
          <CardContent className="space-y-2 text-sm text-muted-foreground">
            <p>Bring work in from Jira with connect, preview, mapping, progress, and validation.</p>
            <Link href="/settings"><Button variant="outline">Open migration workspace</Button></Link>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>Developer context</CardTitle></CardHeader>
          <CardContent className="space-y-2 text-sm text-muted-foreground">
            <p>Use the CLI for repo-aware issue creation and the web for fast issue review.</p>
            <Link href="/projects"><Button variant="outline">Go to issue list</Button></Link>
          </CardContent>
        </Card>
      </div>

      {features.INSIGHTS ? (
        <div className="grid gap-4 md:grid-cols-2">
          <Card>
            <CardHeader><CardTitle>Delivery velocity</CardTitle></CardHeader>
            <CardContent className="text-sm">
              <p className="text-2xl font-semibold">{velocityQuery.data?.current ?? 0}</p>
              <p className="text-muted-foreground">Trend: {(velocityQuery.data?.trend ?? []).join(' → ') || 'No sprint data yet'}</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Queue bottleneck</CardTitle></CardHeader>
            <CardContent className="text-sm">
              <p className="text-2xl font-semibold">{bottleneckQuery.data?.status || 'none'}</p>
              <p className="text-muted-foreground">Avg {Number(bottleneckQuery.data?.avg_days ?? 0).toFixed(1)} days</p>
            </CardContent>
          </Card>
        </div>
      ) : (
        <Card>
          <CardHeader><CardTitle>{comingSoonContent.INSIGHTS.title}</CardTitle></CardHeader>
          <CardContent className="text-sm text-muted-foreground">
            {comingSoonContent.INSIGHTS.description} {comingSoonContent.INSIGHTS.hint}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader><CardTitle>Recent issue activity</CardTitle></CardHeader>
        <CardContent className="space-y-2 text-sm">
          {issuesQuery.isPending ? (
            <>
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
            </>
          ) : (
            issues.slice(0, 8).map((issue) => (
              <Link key={issue.id} className="block rounded-md border p-2 hover:bg-accent" href={`/issues/${issue.id}`}>
                {issue.title}
              </Link>
            ))
          )}
          {!issuesQuery.isPending && !issues.length ? (
            <EmptyState
              title="No issue activity yet"
              description="Create an issue to start tracking work in the fastest path through the product."
              action={<Link href="/projects"><Button>Create issue</Button></Link>}
              hint="Use the issue list as the default workspace for day-to-day work."
            />
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}

function Summary({ title, value, helper }: { title: string; value: number; helper: string }) {
  return (
    <Card>
      <CardHeader><CardTitle>{title}</CardTitle></CardHeader>
      <CardContent>
        <div className="text-2xl font-semibold">{value}</div>
        <div className="text-xs text-muted-foreground">{helper}</div>
      </CardContent>
    </Card>
  );
}
