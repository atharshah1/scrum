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
import { comingSoonContent, features } from '@/lib/features';
import { qk } from '@/lib/query-keys';
import type { Board, CycleTimeInsight, Issue, IssueComment, WorkflowTransition } from '@/types';

const defaultTransitions: WorkflowTransition[] = [
  { id: 'todo-in-progress', from_status: 'todo', to_status: 'in_progress', conditions: {}, validators: {}, post_functions: {} },
  { id: 'todo-done', from_status: 'todo', to_status: 'done', conditions: {}, validators: {}, post_functions: {} },
  { id: 'in-progress-todo', from_status: 'in_progress', to_status: 'todo', conditions: {}, validators: {}, post_functions: {} },
  { id: 'in-progress-done', from_status: 'in_progress', to_status: 'done', conditions: {}, validators: {}, post_functions: {} },
  { id: 'done-todo', from_status: 'done', to_status: 'todo', conditions: {}, validators: {}, post_functions: {} }
];

type ConflictPreview = {
  kind: 'update' | 'transition';
  payload: Record<string, unknown>;
  local: Issue;
  changedFields: string[];
};

export default function IssueDetailPage() {
  const params = useParams<{ id: string }>();
  const issueId = params.id;
  const queryClient = useQueryClient();
  const [nextStatus, setNextStatus] = useState('');
  const [conflictPreview, setConflictPreview] = useState<ConflictPreview | null>(null);

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
    enabled: features.INSIGHTS,
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
        body: JSON.stringify({ ...payload, updated_at: issue?.updated_at })
      }),
    onMutate: async (payload) => {
      await queryClient.cancelQueries({ queryKey: qk.issue(issueId), exact: true });
      const previousIssue = queryClient.getQueryData<Issue>(qk.issue(issueId));
      if (previousIssue) {
        queryClient.setQueryData<Issue>(qk.issue(issueId), { ...previousIssue, ...payload });
      }
      return { previousIssue };
    },
    onSuccess: () => {
      setConflictPreview(null);
    },
    onError: (error, payload, context) => {
      if (context?.previousIssue) {
        queryClient.setQueryData(qk.issue(issueId), context.previousIssue);
      }
      const message = error instanceof Error ? error.message : 'Changes were reverted.';
      const isConflict = /409|conflict|optimistic|updated_at/i.test(message);
      if (isConflict && context?.previousIssue) {
        setConflictPreview(buildConflictPreview(context.previousIssue, payload, 'update'));
      }
      toast({
        title: isConflict ? 'Update conflict detected' : 'Update failed',
        description: isConflict ? 'Review local vs remote values below, then keep local, keep remote, or merge.' : message,
        variant: 'error'
      });
      if (isConflict) {
        queryClient.invalidateQueries({ queryKey: qk.issue(issueId), exact: true });
      }
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
        body: JSON.stringify({ status, updated_at: issue?.updated_at })
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
      setConflictPreview(null);
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
    onError: (error, status, context) => {
      if (context?.previousIssue) {
        queryClient.setQueryData(qk.issue(issueId), context.previousIssue);
      }
      const message = error instanceof Error ? error.message : 'Issue transition was reverted.';
      const isConflict = /409|conflict|optimistic|updated_at/i.test(message);
      if (isConflict && context?.previousIssue) {
        setConflictPreview(buildConflictPreview(context.previousIssue, { status }, 'transition'));
      }
      toast({
        title: isConflict ? 'Transition conflict detected' : 'Transition failed',
        description: isConflict ? 'Review local vs remote status below, then keep local, keep remote, or merge.' : message,
        variant: 'error'
      });
      if (isConflict) {
        queryClient.invalidateQueries({ queryKey: qk.issue(issueId), exact: true });
      }
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

  const resolveConflict = (resolution: 'local' | 'remote' | 'merge') => {
    if (!conflictPreview || !issue) return;
    if (resolution === 'remote') {
      setConflictPreview(null);
      queryClient.invalidateQueries({ queryKey: qk.issue(issueId), exact: true });
      return;
    }
    if (resolution === 'local') {
      if (conflictPreview.kind === 'transition') {
        transitionIssue.mutate(String(conflictPreview.payload.status ?? conflictPreview.local.status));
        return;
      }
      updateIssue.mutate(conflictPreview.payload);
      return;
    }
    updateIssue.mutate(mergeConflictPayload(conflictPreview, issue));
  };

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
              <Badge className="border-emerald-200 text-emerald-700">Conflict-safe editing</Badge>
              {features.INSIGHTS ? <Badge>⏱ Cycle time: {(cycleTimeQuery.data?.avg_days ?? 0).toFixed(1)} days</Badge> : null}
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
              <Button
                variant="outline"
                onClick={() => toast({ title: comingSoonContent.AI.title, description: `${comingSoonContent.AI.description} ${comingSoonContent.AI.hint}` })}
              >
                Draft summary
              </Button>
              <Button
                variant="outline"
                onClick={() => toast({ title: comingSoonContent.AI.title, description: `${comingSoonContent.AI.description} ${comingSoonContent.AI.hint}` })}
              >
                Suggest labels
              </Button>
              <Button variant="outline" onClick={() => timeMutation.mutate('start')}>Start timer</Button>
              <Button variant="outline" onClick={() => timeMutation.mutate('stop')}>Stop timer</Button>
            </div>
            {!features.AI ? <p className="text-xs text-muted-foreground">{comingSoonContent.AI.description} {comingSoonContent.AI.hint}</p> : null}
          </CardContent>
        </Card>

        {conflictPreview && issue ? (
          <Card>
            <CardHeader><CardTitle>Conflict review</CardTitle></CardHeader>
            <CardContent className="space-y-4 text-sm">
              <p className="text-muted-foreground">Compare your local change with the latest remote value, then keep local, keep remote, or merge.</p>
              <div className="grid gap-3 md:grid-cols-2">
                <div className="rounded-md border p-3">
                  <div className="mb-2 text-xs font-medium text-muted-foreground">LOCAL CHANGE</div>
                  {conflictPreview.changedFields.map((field) => (
                    <div key={`local-${field}`} className="mb-2">
                      <div className="text-xs uppercase text-muted-foreground">{field}</div>
                      <div>{formatConflictValue(conflictPreview.local[field as keyof Issue])}</div>
                    </div>
                  ))}
                </div>
                <div className="rounded-md border p-3">
                  <div className="mb-2 text-xs font-medium text-muted-foreground">REMOTE CHANGE</div>
                  {conflictPreview.changedFields.map((field) => (
                    <div key={`remote-${field}`} className="mb-2">
                      <div className="text-xs uppercase text-muted-foreground">{field}</div>
                      <div>{formatConflictValue(issue[field as keyof Issue])}</div>
                    </div>
                  ))}
                </div>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button variant="outline" onClick={() => resolveConflict('local')}>Keep local</Button>
                <Button variant="outline" onClick={() => resolveConflict('remote')}>Keep remote</Button>
                <Button onClick={() => resolveConflict('merge')}>Merge</Button>
              </div>
            </CardContent>
          </Card>
        ) : null}

        <Card>
          <CardHeader><CardTitle>Comments (live)</CardTitle></CardHeader>
          <CardContent>
            <IssueComments issueId={issueId} comments={commentsQuery.data ?? []} />
          </CardContent>
        </Card>
      </div>

      <div className="space-y-4">
        <Card>
          <CardHeader><CardTitle>Sync & safety</CardTitle></CardHeader>
          <CardContent className="space-y-2 text-sm text-muted-foreground">
            <p>✔ Inline edits stay fast with optimistic updates.</p>
            <p>✔ Conflicts are surfaced instead of silently overwriting work.</p>
            <p>✔ Merge actions let you preserve both local intent and latest remote state.</p>
          </CardContent>
        </Card>

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
    </div>
  );
}

function buildConflictPreview(previousIssue: Issue, payload: Record<string, unknown>, kind: ConflictPreview['kind']): ConflictPreview {
  return {
    kind,
    payload,
    local: { ...previousIssue, ...payload },
    changedFields: Object.keys(payload)
  };
}

function mergeConflictPayload(preview: ConflictPreview, remoteIssue: Issue): Record<string, unknown> {
  const merged: Record<string, unknown> = {};
  for (const field of preview.changedFields) {
    const localValue = preview.local[field as keyof Issue];
    const remoteValue = remoteIssue[field as keyof Issue];
    switch (field) {
      case 'labels':
        merged[field] = Array.from(new Set([...(Array.isArray(remoteValue) ? remoteValue : []), ...(Array.isArray(localValue) ? localValue : [])]));
        break;
      case 'description':
      case 'title':
        if (typeof localValue === 'string' && typeof remoteValue === 'string' && localValue !== remoteValue) {
          merged[field] = [remoteValue, localValue].filter(Boolean).join(field === 'description' ? '\n\n' : ' / ');
          break;
        }
        merged[field] = localValue;
        break;
      case 'status':
      case 'priority':
      case 'assignee_id':
        merged[field] = localValue ?? remoteValue;
        break;
      default:
        merged[field] = localValue;
        break;
    }
  }
  return merged;
}

function formatConflictValue(value: unknown) {
  if (Array.isArray(value)) return value.join(', ') || '<empty>';
  if (value === null || value === undefined || value === '') return '<empty>';
  return String(value);
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
