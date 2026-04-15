'use client';

import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { Issue } from '@/types';

export default function ProjectDetailPage() {
  const params = useParams<{ id: string }>();
  const projectId = params.id;

  const issuesQuery = useQuery({
    queryKey: qk.issues(`project-${projectId}`),
    queryFn: () => apiRequest<Issue[]>(`/issues?project_id=${projectId}`)
  });

  return (
    <Card>
      <CardHeader><CardTitle>Project {projectId}</CardTitle></CardHeader>
      <CardContent className="space-y-2">
        {(issuesQuery.data ?? []).map((issue) => (
          <div key={issue.id} className="rounded-md border p-3 text-sm">{issue.title}</div>
        ))}
      </CardContent>
    </Card>
  );
}
