'use client';

import { useState } from 'react';
import { useParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { IssueComments } from '@/components/issues/issue-comments';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { Issue, IssueComment } from '@/types';

export default function IssueDetailPage() {
  const params = useParams<{ id: string }>();
  const issueId = params.id;
  const queryClient = useQueryClient();

  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');

  const issueQuery = useQuery({
    queryKey: qk.issue(issueId),
    queryFn: () => apiRequest<Issue>(`/issues/${issueId}`)
  });

  const commentsQuery = useQuery({
    queryKey: qk.comments(issueId),
    queryFn: () => apiRequest<IssueComment[]>(`/issues/${issueId}/comments`)
  });

  const activityQuery = useQuery({
    queryKey: qk.activities(issueId),
    queryFn: () => apiRequest<Array<{ id: string; action: string; created_at: string }>>(`/issues/${issueId}/activity`)
  });

  const updateIssue = useMutation({
    mutationFn: async () =>
      apiRequest(`/issues/${issueId}`, {
        method: 'PATCH',
        body: JSON.stringify({ title: title || undefined, description: description || undefined })
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.issue(issueId) })
  });

  const timeMutation = useMutation({
    mutationFn: async (mode: 'start' | 'stop') => apiRequest(`/time/${mode}`, { method: 'POST', body: JSON.stringify({ issue_id: issueId }) })
  });

  const issue = issueQuery.data;

  return (
    <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
      <div className="space-y-4">
        <Card>
          <CardHeader><CardTitle>Issue details</CardTitle></CardHeader>
          <CardContent className="space-y-3">
            <Input defaultValue={issue?.title} onChange={(e) => setTitle(e.target.value)} placeholder="Title" />
            <Textarea defaultValue={issue?.description} onChange={(e) => setDescription(e.target.value)} placeholder="Description" />
            <div className="flex items-center gap-2">
              <Badge>{issue?.status ?? 'unknown'}</Badge>
              <Badge>{issue?.assignee_id ? `@${issue.assignee_id.slice(0, 8)}` : 'Unassigned'}</Badge>
            </div>
            <div className="flex gap-2">
              <Button onClick={() => updateIssue.mutate()} disabled={updateIssue.isPending}>Update issue</Button>
              <Button variant="outline" onClick={() => timeMutation.mutate('start')}>Start timer</Button>
              <Button variant="outline" onClick={() => timeMutation.mutate('stop')}>Stop timer</Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader><CardTitle>Comments (live)</CardTitle></CardHeader>
          <CardContent>
            <IssueComments issueId={issueId} comments={commentsQuery.data ?? []} />
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader><CardTitle>Activity timeline</CardTitle></CardHeader>
        <CardContent className="space-y-2 text-sm">
          {(activityQuery.data ?? []).map((entry) => (
            <div key={entry.id} className="rounded-md border p-2">
              <div>{entry.action}</div>
              <div className="text-xs text-muted-foreground">{new Date(entry.created_at).toLocaleString()}</div>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}
