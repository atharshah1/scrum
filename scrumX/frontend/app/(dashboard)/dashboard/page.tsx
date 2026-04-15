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
      <h1 className="text-xl font-semibold">Dashboard</h1>
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
            <Summary title="Total issues" value={issues.length} />
            <Summary title="Active" value={active} />
            <Summary title="Done" value={done} />
            <Summary title="Assigned to me" value={issues.filter((i) => !!i.assignee_id).length} />
          </>
        )}
      </div>
      {features.INSIGHTS ? (
        <div className="grid gap-4 md:grid-cols-2">
          <Card>
            <CardHeader><CardTitle>Velocity</CardTitle></CardHeader>
            <CardContent className="text-sm">
              <p className="text-2xl font-semibold">{velocityQuery.data?.current ?? 0}</p>
              <p className="text-muted-foreground">📈 Trend: {(velocityQuery.data?.trend ?? []).join(' → ') || 'No sprint data'}</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Bottleneck</CardTitle></CardHeader>
            <CardContent className="text-sm">
              <p className="text-2xl font-semibold">{bottleneckQuery.data?.status || 'none'}</p>
              <p className="text-muted-foreground">⚠️ Avg {Number(bottleneckQuery.data?.avg_days ?? 0).toFixed(1)} days</p>
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
        <CardHeader><CardTitle>Activity feed</CardTitle></CardHeader>
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
              title="No activity yet"
              description="Create an issue to start tracking work on your board."
              action={<Link href="/projects"><Button>Create issue</Button></Link>}
              hint="Once created, updates and comments appear here in real time."
            />
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}

function Summary({ title, value }: { title: string; value: number }) {
  return (
    <Card>
      <CardHeader><CardTitle>{title}</CardTitle></CardHeader>
      <CardContent><div className="text-2xl font-semibold">{value}</div></CardContent>
    </Card>
  );
}
