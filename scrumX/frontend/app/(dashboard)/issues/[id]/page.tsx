'use client';

import { useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { InlineEdit } from '@/components/forms/inline-edit';
import { IssueComments } from '@/components/issues/issue-comments';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Select } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { toast } from '@/components/ui/toast';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { Board, CycleTimeInsight, Issue, IssueComment, WorkflowTransition } from '@/types';

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
  const [nextStatus, setNextStatus] = useState('');

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

  const cycleTimeQuery = useQuery({
    queryKey: qk.insightsCycleTime(issueId),
    queryFn: () => apiRequest<CycleTimeInsight>(`/insights/cycle-time?issue_id=${issueId}`)
  });

  const transitions = transitionsQuery.data?.length ? transitionsQuery.data : defaultTransitions;
  const issue = issueQuery.data;
  const allowedTransitions = useMemo(
    () => transitions.filter((transition) => transition.from_status === issue?.status).map((transition) => transition.to_status),
    [issue?.status, transitions]
  );

  const updateIssue = useMutation({
    mutationFn: async (payload: Record<string, unknown>) =>
      apiRequest<Issue>(`/issues/${issueId}`, {
        method: 'PATCH',
        body: JSON.stringify(payload)
      }),
    onMutate: async (payload) => {
      await queryClient.cancelQueries({ queryKey: qk.issue(issueId), exact: true });
      const previousIssue = queryClient.getQueryData<Issue>(qk.issue(issueId));
      if (previousIssue) {
        queryClient.setQueryData<Issue>(qk.issue(issueId), { ...previousIssue, ...payload });
      }
      return { previousIssue };
    },
    onError: (_error, _vars, context) => {
      if (context?.previousIssue) {
        queryClient.setQueryData(qk.issue(issueId), context.previousIssue);
      }
      toast({ title: 'Update failed', description: 'Changes were reverted.', variant: 'error' });
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: qk.issue(issueId), exact: true });
      queryClient.invalidateQueries({ predicate: (query) => query.queryKey[0] === 'issues' });
      queryClient.invalidateQueries({ predicate: (query) => query.queryKey[0] === 'board' });
    }
  });

  const transitionIssue = useMutation({
    mutationFn: async (status: string) =>
      apiRequest<Issue>(`/issues/${issueId}`, {
        method: 'PATCH',
        body: JSON.stringify({ status })
      }),
    onMutate: async (status) => {
      await queryClient.cancelQueries({ queryKey: qk.issue(issueId), exact: true });
      const previousIssue = queryClient.getQueryData<Issue>(qk.issue(issueId));
      if (previousIssue) {
        queryClient.setQueryData<Issue>(qk.issue(issueId), { ...previousIssue, status });
      }
      return { previousIssue };
    },
    onSuccess: (_updatedIssue, status) => {
      setNextStatus('');
      queryClient.setQueryData<Board | undefined>(qk.board(issue?.project_id ?? ''), (board) => {
        if (!board) return board;
        return {
          ...board,
          columns: board.columns.map((column) => ({
            ...column,
            issues: column.issues.map((item) => (item.id === issueId ? { ...item, status } : item))
          }))
        };
      });
    },
    onError: (_error, _variables, context) => {
      if (context?.previousIssue) {
        queryClient.setQueryData(qk.issue(issueId), context.previousIssue);
      }
      toast({ title: 'Transition failed', description: 'Issue transition was reverted.', variant: 'error' });
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: qk.issue(issueId), exact: true });
      queryClient.invalidateQueries({ predicate: (query) => query.queryKey[0] === 'issues' });
      queryClient.invalidateQueries({ predicate: (query) => query.queryKey[0] === 'board' });
    }
  });

  const timeMutation = useMutation({
    mutationFn: async (mode: 'start' | 'stop') =>
      apiRequest(`/time/${mode}`, { method: 'POST', body: JSON.stringify({ issue_id: issueId }) })
  });

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
          <CardContent className="space-y-4">
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">Title</p>
              <InlineEdit
                value={issue?.title ?? ''}
                onSave={(value) => updateIssue.mutate({ title: value })}
                className="font-medium"
                placeholder="Issue title"
              />
            </div>

            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">Description (click to edit)</p>
              <InlineTextarea
                value={issue?.description ?? ''}
                onSave={(value) => updateIssue.mutate({ description: value })}
              />
            </div>

            <div className="grid gap-3 md:grid-cols-2">
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">Priority</p>
                <Select value={issue?.priority ?? 'medium'} onChange={(event) => updateIssue.mutate({ priority: event.target.value })}>
                  <option value="low">low</option>
                  <option value="medium">medium</option>
                  <option value="high">high</option>
                </Select>
              </div>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">Assignee (UUID)</p>
                <InlineEdit
                  value={issue?.assignee_id ?? ''}
                  onSave={(value) => updateIssue.mutate({ assignee_id: value || null })}
                  placeholder="Click to assign"
                />
              </div>
            </div>

            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">Labels (comma separated)</p>
              <InlineEdit
                value={(issue?.labels ?? []).join(', ')}
                onSave={(value) =>
                  updateIssue.mutate({
                    labels: value.split(',').map((item) => item.trim()).filter(Boolean)
                  })
                }
                placeholder="Click to add labels"
              />
            </div>

            <div className="flex items-center gap-2">
              <Badge>{issue?.status ?? 'unknown'}</Badge>
              <Badge>⏱ Cycle time: {(cycleTimeQuery.data?.avg_days ?? 0).toFixed(1)} days</Badge>
            </div>

            <div className="grid gap-2 md:grid-cols-[1fr_auto]">
              <Select value={nextStatus} onChange={(event) => setNextStatus(event.target.value)}>
                <option value="">Select workflow transition</option>
                {allowedTransitions.map((status) => (
                  <option key={status} value={status}>{status}</option>
                ))}
              </Select>
              <Button variant="outline" onClick={() => nextStatus && transitionIssue.mutate(nextStatus)} disabled={!nextStatus || transitionIssue.isPending}>
                Apply transition
              </Button>
            </div>

            <div className="flex gap-2">
              <Button variant="outline" onClick={() => toast({ title: 'AI Summary', description: 'Coming soon 🚧' })}>✨ Summarize</Button>
              <Button variant="outline" onClick={() => toast({ title: 'AI Suggestions', description: 'Coming soon 🚧' })}>✨ Suggest Fields</Button>
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

function InlineTextarea({ value, onSave }: { value: string; onSave: (value: string) => void }) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);

  if (editing) {
    return (
      <Textarea
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={() => {
          setEditing(false);
          if (draft !== value) onSave(draft);
        }}
        onKeyDown={(event) => {
          if (event.key === 'Escape') {
            setDraft(value);
            setEditing(false);
          }
          if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
            setEditing(false);
            if (draft !== value) onSave(draft);
          }
        }}
      />
    );
  }

  return (
    <button type="button" className="w-full rounded-md border p-2 text-left text-sm hover:bg-accent" onClick={() => setEditing(true)}>
      {value || 'Click to add description'}
    </button>
  );
}
