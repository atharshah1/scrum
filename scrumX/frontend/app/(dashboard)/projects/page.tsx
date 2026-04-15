'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from '@/components/ui/toast';
import { useFormFields, required } from '@/components/forms/use-form';
import { apiRequest } from '@/lib/api';
import { isAdminRole } from '@/lib/permissions';
import { qk } from '@/lib/query-keys';
import { useAppStore } from '@/store/useAppStore';
import { useAuthStore } from '@/store/useAuthStore';
import type { Issue } from '@/types';

export default function ProjectsPage() {
  const router = useRouter();
  const selectedProjectId = useAppStore((s) => s.selectedProjectId);
  const setSelectedProject = useAppStore((s) => s.setSelectedProject);
  const filters = useAppStore((s) => s.issueFilters);
  const setIssueFilter = useAppStore((s) => s.setIssueFilter);
  const queryClient = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const isAdmin = isAdminRole(user?.role);
  const [activeIndex, setActiveIndex] = useState(0);
  const createIssueForm = useFormFields(
    { title: '', description: '' },
    {
      title: [required('Issue title')]
    }
  );

  const filterKey = JSON.stringify({ selectedProjectId, filters });
  const issuesQuery = useQuery({
    queryKey: qk.issues(filterKey),
    queryFn: () => {
      const search = new URLSearchParams();
      if (selectedProjectId) search.set('project_id', selectedProjectId);
      if (filters.status) search.set('status', filters.status);
      if (filters.label) search.set('label', filters.label);
      if (filters.assignee_id) search.set('assignee_id', filters.assignee_id);
      return apiRequest<Issue[]>(`/issues?${search.toString()}`);
    }
  });

  const createIssue = useMutation({
    mutationFn: async () =>
      apiRequest<Issue>('/issues/', {
        method: 'POST',
        body: JSON.stringify({
          project_id: selectedProjectId,
          title: createIssueForm.values.title,
          description: createIssueForm.values.description
        })
      }),
    onSuccess: (createdIssue) => {
      createIssueForm.reset();
      queryClient.setQueryData<Issue[]>(qk.issues(filterKey), (prev) => [createdIssue, ...(prev ?? [])]);
      queryClient.invalidateQueries({ queryKey: qk.issues(filterKey), exact: true });
      if (createdIssue.id) {
        queryClient.setQueryData(qk.issue(createdIssue.id), createdIssue);
      }
    }
  });

  const issues = useMemo(() => issuesQuery.data ?? [], [issuesQuery.data]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      const isTypingTarget =
        target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.isContentEditable;
      if (isTypingTarget || event.ctrlKey || event.metaKey || event.altKey) return;
      if (!issues.length) return;
      if (event.key.toLowerCase() === 'j') {
        event.preventDefault();
        setActiveIndex((index) => Math.min(index + 1, issues.length - 1));
      }
      if (event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setActiveIndex((index) => Math.max(index - 1, 0));
      }
      if (event.key === 'Enter' && issues[activeIndex]) {
        event.preventDefault();
        router.push(`/issues/${issues[activeIndex].id}`);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [activeIndex, issues, router]);

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Projects & Issues</h1>
      <p className="text-xs text-muted-foreground">Keyboard: J/K to move, Enter to open selected issue.</p>
      <Card>
        <CardHeader><CardTitle>Filters</CardTitle></CardHeader>
        <CardContent className="grid gap-2 md:grid-cols-4">
          <Input placeholder="Project ID" value={selectedProjectId ?? ''} onChange={(e) => setSelectedProject(e.target.value || null)} />
          <Select value={filters.status ?? ''} onChange={(e) => setIssueFilter('status', e.target.value)}>
            <option value="">All statuses</option>
            <option value="todo">To do</option>
            <option value="in_progress">In progress</option>
            <option value="done">Done</option>
          </Select>
          <Input placeholder="Label" value={filters.label ?? ''} onChange={(e) => setIssueFilter('label', e.target.value)} />
          <Input placeholder="Assignee ID" value={filters.assignee_id ?? ''} onChange={(e) => setIssueFilter('assignee_id', e.target.value)} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Create issue</CardTitle></CardHeader>
        <CardContent className="grid gap-2 md:grid-cols-[1fr_2fr_auto]">
          <div>
            <Input placeholder="Issue title" value={createIssueForm.values.title} onChange={(e) => createIssueForm.setField('title', e.target.value)} />
            {createIssueForm.errors.title ? <p className="mt-1 text-xs text-red-600">{createIssueForm.errors.title}</p> : null}
          </div>
          <Input placeholder="Description" value={createIssueForm.values.description} onChange={(e) => createIssueForm.setField('description', e.target.value)} />
          <Button
            disabled={!selectedProjectId || createIssue.isPending || !createIssueForm.isValid || !isAdmin}
            onClick={() => {
              if (!createIssueForm.validate()) return;
              createIssue.mutate();
            }}
          >
            {isAdmin ? 'Create' : 'Admin only'}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Issues</CardTitle></CardHeader>
        <CardContent className="space-y-2">
          {issuesQuery.isPending ? (
            <>
              <Skeleton className="h-14" />
              <Skeleton className="h-14" />
              <Skeleton className="h-14" />
            </>
          ) : (
            issues.map((issue) => (
              <IssueRow
                key={issue.id}
                issue={issue}
                active={issues[activeIndex]?.id === issue.id}
                filterKey={filterKey}
              />
            ))
          )}
          {!issuesQuery.isPending && !issues.length ? (
            <EmptyState
              title="No issues found"
              description="Start by selecting a project and creating your first issue."
              action={<Button onClick={() => setSelectedProject('default-project')}>Use sample project</Button>}
              hint="Tip: after creating an issue, use J/K and Enter for keyboard triage."
            />
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}

function IssueRow({ issue, active, filterKey }: { issue: Issue; active: boolean; filterKey: string }) {
  const queryClient = useQueryClient();
  const [assignee, setAssignee] = useState('');
  const [label, setLabel] = useState('');
  const [status, setStatus] = useState(issue.status);

  const quickUpdate = useMutation({
    mutationFn: async (payload: Record<string, unknown>) =>
      apiRequest(`/issues/${issue.id}`, { method: 'PATCH', body: JSON.stringify(payload) }),
    onMutate: async (payload) => {
      await queryClient.cancelQueries({ queryKey: qk.issues(filterKey), exact: true });
      const previous = queryClient.getQueryData<Issue[]>(qk.issues(filterKey));
      if (previous) {
        queryClient.setQueryData<Issue[]>(
          qk.issues(filterKey),
          previous.map((item) => (item.id === issue.id ? { ...item, ...payload } : item))
        );
      }
      return { previous };
    },
    onError: (_error, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(qk.issues(filterKey), context.previous);
      toast({ title: 'Quick action failed', description: 'Unable to apply quick issue update.', variant: 'error' });
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: qk.issues(filterKey), exact: true });
      queryClient.invalidateQueries({ queryKey: qk.issue(issue.id), exact: true });
      queryClient.invalidateQueries({ predicate: (query) => query.queryKey[0] === 'board' });
    }
  });

  const addLabel = useMutation({
    mutationFn: async (nextLabel: string) =>
      apiRequest(`/issues/${issue.id}/labels`, { method: 'POST', body: JSON.stringify({ label: nextLabel }) }),
    onSuccess: () => {
      setLabel('');
      queryClient.invalidateQueries({ queryKey: qk.issues(filterKey), exact: true });
      queryClient.invalidateQueries({ queryKey: qk.issue(issue.id), exact: true });
    }
  });

  return (
    <div className={`group rounded-md border p-3 ${active ? 'ring-2 ring-blue-200' : ''}`}>
      <Link href={`/issues/${issue.id}`} className="block hover:underline">
        <div className="font-medium">{issue.title}</div>
      </Link>
      <div className="text-xs text-muted-foreground">{issue.status}</div>
      <div className="mt-2 hidden grid-cols-1 gap-2 group-hover:grid md:grid-cols-3">
        <Select
          value={status}
          onChange={(event) => {
            setStatus(event.target.value);
            quickUpdate.mutate({ status: event.target.value });
          }}
        >
          <option value="todo">🔁 todo</option>
          <option value="in_progress">🔁 in_progress</option>
          <option value="done">🔁 done</option>
        </Select>
        <div className="flex gap-1">
          <Input value={assignee} onChange={(event) => setAssignee(event.target.value)} placeholder="👤 assignee UUID" />
          <Button size="sm" variant="outline" onClick={() => quickUpdate.mutate({ assignee_id: assignee || null })}>Save</Button>
        </div>
        <div className="flex gap-1">
          <Input value={label} onChange={(event) => setLabel(event.target.value)} placeholder="🏷 label" />
          <Button size="sm" variant="outline" onClick={() => label && addLabel.mutate(label)}>Add</Button>
        </div>
      </div>
    </div>
  );
}
