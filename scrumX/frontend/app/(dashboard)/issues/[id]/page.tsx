'use client';

import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { IssueComments } from '@/components/issues/issue-comments';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { apiRequest } from '@/lib/api';
import { formatAssignee } from '@/lib/format';
import { qk } from '@/lib/query-keys';
import type { Board, Issue, IssueComment, WorkflowTransition } from '@/types';

const defaultTransitions: WorkflowTransition[] = [
  { id: 'todo-in-progress', from_status: 'todo', to_status: 'in_progress', conditions: {}, validators: {}, post_functions: {} },
  { id: 'todo-done', from_status: 'todo', to_status: 'done', conditions: {}, validators: {}, post_functions: {} },
  { id: 'in-progress-todo', from_status: 'in_progress', to_status: 'todo', conditions: {}, validators: {}, post_functions: {} },
  { id: 'in-progress-done', from_status: 'in_progress', to_status: 'done', conditions: {}, validators: {}, post_functions: {} },
  { id: 'done-todo', from_status: 'done', to_status: 'todo', conditions: {}, validators: {}, post_functions: {} }
];

export default function IssueDetailPage() {
  const params = useParams<{ id: string }>();
  const issueId = params.id;
  const queryClient = useQueryClient();

  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [nextStatus, setNextStatus] = useState('');
  const [editing, setEditing] = useState(false);

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

  const transitionsQuery = useQuery({
    queryKey: qk.workflowTransitions(issueQuery.data?.project_id ?? ''),
    enabled: !!issueQuery.data?.project_id,
    queryFn: () => apiRequest<WorkflowTransition[]>(`/workflows/${issueQuery.data?.project_id}/transitions`)
  });

  const transitions = transitionsQuery.data?.length ? transitionsQuery.data : defaultTransitions;

  const updateIssue = useMutation({
    mutationFn: async () =>
      apiRequest<Issue>(`/issues/${issueId}`, {
        method: 'PATCH',
        body: JSON.stringify({ title: title || undefined, description: description || undefined })
      }),
    onSuccess: (updatedIssue) => {
      queryClient.setQueryData(qk.issue(issueId), updatedIssue);
      queryClient.invalidateQueries({ queryKey: qk.issue(issueId), exact: true });
    }
  });

  const transitionIssue = useMutation({
    mutationFn: async (status: string) =>
      apiRequest<Issue>(`/issues/${issueId}`, {
        method: 'PATCH',
        body: JSON.stringify({ status })
      }),
    onSuccess: (updatedIssue) => {
      setNextStatus('');
      queryClient.setQueryData(qk.issue(issueId), updatedIssue);
      queryClient.invalidateQueries({
        predicate: (query) =>
          query.queryKey[0] === 'issues' && String(query.queryKey[1] ?? '').includes(updatedIssue.project_id)
      });
      queryClient.invalidateQueries({
        predicate: (query) => {
          if (query.queryKey[0] !== 'board') return false;
          const board = query.state.data as Board | undefined;
          return board?.project_id === updatedIssue.project_id;
        }
      });
    }
  });

  const timeMutation = useMutation({
    mutationFn: async (mode: 'start' | 'stop') => apiRequest(`/time/${mode}`, { method: 'POST', body: JSON.stringify({ issue_id: issueId }) })
  });

  const issue = issueQuery.data;

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
        event.preventDefault();
        if (editing) {
          updateIssue.mutate();
        }
      }
      if (event.key.toLowerCase() === 'e' && !event.metaKey && !event.ctrlKey) {
        setEditing((value) => !value);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [editing, updateIssue]);
  const allowedTransitions = useMemo(
    () => transitions.filter((transition) => transition.from_status === issue?.status).map((transition) => transition.to_status),
    [issue?.status, transitions]
  );

  if (issueQuery.isPending) {
    return (
      <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
        <Skeleton className="h-72" />
        <Skeleton className="h-72" />
      </div>
    );
  }

  return (
    <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
      <div className="space-y-4">
        <Card>
          <CardHeader><CardTitle>Issue details</CardTitle></CardHeader>
          <CardContent className="space-y-3">
            <Input defaultValue={issue?.title} onChange={(e) => setTitle(e.target.value)} placeholder="Title" readOnly={!editing} />
            <Textarea defaultValue={issue?.description} onChange={(e) => setDescription(e.target.value)} placeholder="Description" readOnly={!editing} />
            <div className="flex items-center gap-2">
              <Badge>{issue?.status ?? 'unknown'}</Badge>
              <Badge>{formatAssignee(issue?.assignee_id)}</Badge>
            </div>
            <div className="grid gap-2 md:grid-cols-[1fr_auto]">
              <Select value={nextStatus} onChange={(e) => setNextStatus(e.target.value)}>
                <option value="">Select workflow transition</option>
                {allowedTransitions.map((status) => (
                  <option key={status} value={status}>{status}</option>
                ))}
              </Select>
              <Button
                variant="outline"
                onClick={() => nextStatus && transitionIssue.mutate(nextStatus)}
                disabled={!nextStatus || transitionIssue.isPending}
              >
                Apply transition
              </Button>
            </div>
            <div className="flex gap-2">
              <Button onClick={() => updateIssue.mutate()} disabled={updateIssue.isPending || !editing}>Update issue</Button>
              <Button variant="outline" onClick={() => setEditing((v) => !v)}>{editing ? 'View mode' : 'Quick edit (E)'}</Button>
              <Button variant="outline" onClick={() => timeMutation.mutate('start')}>Start timer</Button>
              <Button variant="outline" onClick={() => timeMutation.mutate('stop')}>Stop timer</Button>
            </div>
            {editing ? <p className="text-xs text-muted-foreground">Tip: press Ctrl/Cmd+S to save quickly.</p> : null}
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
