'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { Issue } from '@/types';

export default function DashboardPage() {
  const issuesQuery = useQuery({
    queryKey: qk.issues('dashboard'),
    queryFn: () => apiRequest<Issue[]>('/issues?limit=20')
  });

  const issues = issuesQuery.data ?? [];
  const active = issues.filter((i) => i.status !== 'done').length;
  const done = issues.filter((i) => i.status === 'done').length;

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Dashboard</h1>
      <div className="grid gap-4 md:grid-cols-4">
        <Summary title="Total issues" value={issues.length} />
        <Summary title="Active" value={active} />
        <Summary title="Done" value={done} />
        <Summary title="Assigned to me" value={issues.filter((i) => !!i.assignee_id).length} />
      </div>
      <Card>
        <CardHeader><CardTitle>Activity feed</CardTitle></CardHeader>
        <CardContent className="space-y-2 text-sm">
          {issues.slice(0, 8).map((issue) => (
            <Link key={issue.id} className="block rounded-md border p-2 hover:bg-accent" href={`/issues/${issue.id}`}>
              {issue.title}
            </Link>
          ))}
          {!issues.length && <p className="text-muted-foreground">No activity yet.</p>}
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
