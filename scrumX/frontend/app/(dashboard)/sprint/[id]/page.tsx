'use client';

import { useParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { Issue } from '@/types';

export default function SprintPage() {
  const params = useParams<{ id: string }>();
  const sprintId = params.id;
  const queryClient = useQueryClient();

  const issuesQuery = useQuery({
    queryKey: qk.issues(`sprint-${sprintId}`),
    queryFn: () => apiRequest<Issue[]>(`/issues?sprint_id=${sprintId}`)
  });

  const start = useMutation({
    mutationFn: async () => apiRequest(`/sprints/${sprintId}/start`, { method: 'POST' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.issues(`sprint-${sprintId}`) })
  });

  const end = useMutation({
    mutationFn: async () => apiRequest(`/sprints/${sprintId}/end`, { method: 'POST' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.issues(`sprint-${sprintId}`) })
  });

  const issues = issuesQuery.data ?? [];
  const done = issues.filter((i) => i.status === 'done').length;
  const progress = issues.length ? (done / issues.length) * 100 : 0;

  return (
    <Card>
      <CardHeader><CardTitle>Active sprint {sprintId}</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <Progress value={progress} />
        <p className="text-sm text-muted-foreground">{done}/{issues.length} issues completed</p>
        <div className="flex gap-2">
          <Button onClick={() => start.mutate()} disabled={start.isPending}>Start sprint</Button>
          <Button variant="outline" onClick={() => end.mutate()} disabled={end.isPending}>End sprint</Button>
        </div>
      </CardContent>
    </Card>
  );
}
