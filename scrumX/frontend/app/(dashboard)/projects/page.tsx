'use client';

import Link from 'next/link';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Badge } from '@/components/ui/badge';
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
import type { Issue, Project } from '@/types';

const demoIssueTemplates = [
  {
    title: 'Demo: create or edit work fast',
    description: 'Use this issue first to prove the create → edit loop in the workspace.'
  },
  {
    title: 'Demo: offline-safe draft',
    description: 'This issue is here to show the pending/offline-safe story before you sync.'
  },
  {
    title: 'Demo: resolve the preloaded conflict',
    description: 'Open this issue to review the recommended conflict resolution path.',
    labels: ['demo-conflict']
  }
] satisfies Array<{ title: string; description: string; labels?: string[] }>;

export default function ProjectsPage() {
  const router = useRouter();
  const selectedProjectId = useAppStore((s) => s.selectedProjectId);
  const setSelectedProject = useAppStore((s) => s.setSelectedProject);
  const filters = useAppStore((s) => s.issueFilters);
  const setIssueFilter = useAppStore((s) => s.setIssueFilter);
  const jqlSearch = useAppStore((s) => s.jqlSearch);
  const conflictIssueIds = useAppStore((s) => s.conflictIssueIds);
  const beginPendingAction = useAppStore((s) => s.beginPendingAction);
  const finishPendingAction = useAppStore((s) => s.finishPendingAction);
  const registerConflictIssue = useAppStore((s) => s.registerConflictIssue);
  const clearConflictIssue = useAppStore((s) => s.clearConflictIssue);
  const queryClient = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const isAdmin = isAdminRole(user?.role);
  const [activeIndex, setActiveIndex] = useState(0);
  const titleInputRef = useRef<HTMLInputElement | null>(null);
  const autoDemoStartedRef = useRef(false);
  const createIssueForm = useFormFields(
    { title: '', description: '' },
    {
      title: [required('Issue title')]
    }
  );

  const projectsQuery = useQuery({
    queryKey: qk.projects,
    queryFn: () => apiRequest<Project[]>('/projects/')
  });

  const filterKey = JSON.stringify({ selectedProjectId, filters, jqlSearch });
  const issuesQuery = useQuery({
    queryKey: qk.issues(filterKey),
    queryFn: () => {
      if (jqlSearch.trim()) {
        const search = new URLSearchParams();
        search.set('q', jqlSearch);
        return apiRequest<Issue[]>(`/issues/search?${search.toString()}`);
      }
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
      clearConflictIssue(createdIssue.id);
      queryClient.setQueryData<Issue[]>(qk.issues(filterKey), (prev) => [createdIssue, ...(prev ?? [])]);
      queryClient.invalidateQueries({ queryKey: qk.issues(filterKey), exact: true });
      if (createdIssue.id) {
        queryClient.setQueryData(qk.issue(createdIssue.id), createdIssue);
      }
    },
    onMutate: async () => {
      beginPendingAction();
    },
    onError: (error) => {
      toast({
        title: 'Create issue failed',
        description: error instanceof Error ? error.message : 'Unable to create issue.',
        variant: 'error'
      });
    },
    onSettled: () => {
      finishPendingAction();
    }
  });

  const instantDemo = useMutation({
    mutationFn: async () => {
      const keySuffix = Date.now().toString().slice(-4);
      const project = await apiRequest<Project>('/projects/', {
        method: 'POST',
        body: JSON.stringify({
          name: 'Instant demo project',
          key: `DEMO${keySuffix}`
        })
      });
      const createdIssues = [];
      for (const template of demoIssueTemplates) {
        const createdIssue = await apiRequest<Issue>('/issues/', {
          method: 'POST',
          body: JSON.stringify({
            project_id: project.id,
            title: template.title,
            description: template.description,
            labels: template.labels ?? []
          })
        });
        createdIssues.push(createdIssue);
      }
      return { project, createdIssues };
    },
    onMutate: async () => {
      beginPendingAction();
    },
    onSuccess: ({ project, createdIssues }) => {
      const conflictIssue = createdIssues.find((issue) => issue.labels?.includes('demo-conflict'));
      queryClient.setQueryData<Project[]>(qk.projects, (prev) => [project, ...(prev ?? [])]);
      setSelectedProject(project.id);
      if (conflictIssue) {
        registerConflictIssue(conflictIssue.id);
      }
      queryClient.invalidateQueries({ queryKey: qk.projects, exact: true });
      queryClient.invalidateQueries({ predicate: (query) => query.queryKey[0] === 'issues' });
      toast({
        title: 'Instant demo ready',
        description: 'Your demo project now includes a starter issue, an offline-safe draft, and a preloaded conflict.'
      });
    },
    onError: (error) => {
      toast({
        title: 'Instant demo setup failed',
        description: error instanceof Error ? error.message : 'Unable to prepare the demo project.',
        variant: 'error'
      });
    },
    onSettled: () => {
      finishPendingAction();
    }
  });

  const issues = useMemo(() => issuesQuery.data ?? [], [issuesQuery.data]);
  const projects = useMemo(() => projectsQuery.data ?? [], [projectsQuery.data]);
  const selectedProject = useMemo(
    () => projects.find((project) => project.id === selectedProjectId) ?? null,
    [projects, selectedProjectId]
  );

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get('focus') === 'create') {
      titleInputRef.current?.focus();
    }
  }, []);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      const isTypingTarget =
        target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.isContentEditable;
      if ((isTypingTarget && event.key !== 'Escape') || event.ctrlKey || event.metaKey || event.altKey) return;
      if (event.key.toLowerCase() === 'c') {
        event.preventDefault();
        titleInputRef.current?.focus();
        return;
      }
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

  useEffect(() => {
    if (projectsQuery.isPending || selectedProjectId) return;
    if (projects.length > 0) {
      setSelectedProject(projects[0].id);
      return;
    }
    if (!isAdmin || autoDemoStartedRef.current) return;
    autoDemoStartedRef.current = true;
    instantDemo.mutate();
  }, [instantDemo, isAdmin, projects, projectsQuery.isPending, selectedProjectId, setSelectedProject]);

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Issue workspace</h1>
      <p className="text-xs text-muted-foreground">Keyboard: C create, / filter from top bar, J/K move, Enter open, T jump to trust action.</p>
      {conflictIssueIds.length > 0 ? (
        <Card className="border-amber-200 bg-amber-50/60">
          <CardContent className="flex flex-wrap items-center justify-between gap-3 p-4 text-sm text-amber-950">
            <div>
              <div className="font-medium">⚠ Zero data loss protected your work</div>
              <p className="text-amber-900/80">A conflict was caught before overwrite. Open the issue detail page for instant local vs remote resolution.</p>
            </div>
            <Link href={`/issues/${conflictIssueIds[0]}`}>
              <Button size="sm">Resolve safely now</Button>
            </Link>
          </CardContent>
        </Card>
      ) : null}
      <Card className="border-blue-200 bg-blue-50/40">
        <CardContent className="flex flex-wrap items-center justify-between gap-3 p-4 text-sm text-blue-950">
          <div>
            <div className="font-medium">Instant demo project</div>
            <p className="text-blue-900/80">Start with one ready project, one starter issue, one offline-safe draft, and one conflict that is already waiting for review.</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button size="sm" onClick={() => (projects.length > 0 ? setSelectedProject(projects[0].id) : instantDemo.mutate())} disabled={instantDemo.isPending || (!isAdmin && projects.length === 0)}>
              {projects.length > 0 ? 'Start instant demo' : instantDemo.isPending ? 'Preparing demo…' : 'Create instant demo'}
            </Button>
            <Link href="/dashboard"><Button size="sm" variant="outline">Open guided steps</Button></Link>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>Project & filters</CardTitle></CardHeader>
        <CardContent className="grid gap-2 md:grid-cols-4">
          <Select value={selectedProjectId ?? ''} onChange={(e) => setSelectedProject(e.target.value || null)}>
            <option value="">{projectsQuery.isPending ? 'Loading projects…' : 'Select project'}</option>
            {projects.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}{project.key ? ` (${project.key})` : ''}
              </option>
            ))}
          </Select>
          <Select value={filters.status ?? ''} onChange={(e) => setIssueFilter('status', e.target.value)}>
            <option value="">All statuses</option>
            <option value="todo">To do</option>
            <option value="in_progress">In progress</option>
            <option value="done">Done</option>
          </Select>
          <Input placeholder="Label" value={filters.label ?? ''} onChange={(e) => setIssueFilter('label', e.target.value)} />
          <Input placeholder="Assignee ID" value={filters.assignee_id ?? ''} onChange={(e) => setIssueFilter('assignee_id', e.target.value)} />
        </CardContent>
        {!projectsQuery.isPending && selectedProject ? (
          <p className="px-6 pb-4 text-xs text-muted-foreground">
            Working in <span className="font-medium">{selectedProject.name}</span>. The instant demo keeps the conflict path one click away.
          </p>
        ) : null}
        {jqlSearch.trim() ? (
          <p className="px-6 pb-4 text-xs text-muted-foreground">
            Task filter is active from the top bar: <span className="font-medium">{jqlSearch}</span>
          </p>
        ) : null}
      </Card>

      <Card>
        <CardHeader><CardTitle>Create issue</CardTitle></CardHeader>
        <CardContent>
          <form
            className="grid gap-2 md:grid-cols-[1fr_2fr_auto]"
            onSubmit={(event) => {
              event.preventDefault();
              if (!createIssueForm.validate()) return;
              createIssue.mutate();
            }}
          >
            <div>
              <Input ref={titleInputRef} placeholder="Issue title" value={createIssueForm.values.title} onChange={(e) => createIssueForm.setField('title', e.target.value)} />
              {createIssueForm.errors.title ? <p className="mt-1 text-xs text-red-600">{createIssueForm.errors.title}</p> : null}
            </div>
            <Input placeholder="Description" value={createIssueForm.values.description} onChange={(e) => createIssueForm.setField('description', e.target.value)} />
            <Button
              type="submit"
              disabled={!selectedProjectId || createIssue.isPending || !createIssueForm.isValid || !isAdmin}
            >
              {isAdmin ? 'Create' : 'Admin only'}
            </Button>
          </form>
          <p className="mt-2 text-xs text-muted-foreground">Press C to jump here. Press Enter to create once the title is ready.</p>
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
              description="Start with the instant demo project so you can create, review, and resolve without entering a project ID."
              action={
                <Button onClick={() => (projects.length > 0 ? setSelectedProject(projects[0].id) : instantDemo.mutate())} disabled={instantDemo.isPending || (!isAdmin && projects.length === 0)}>
                  {projects.length > 0 ? 'Open instant demo' : 'Create instant demo'}
                </Button>
              }
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
  const conflictIssueIds = useAppStore((s) => s.conflictIssueIds);
  const beginPendingAction = useAppStore((s) => s.beginPendingAction);
  const finishPendingAction = useAppStore((s) => s.finishPendingAction);
  const registerConflictIssue = useAppStore((s) => s.registerConflictIssue);
  const clearConflictIssue = useAppStore((s) => s.clearConflictIssue);
  const [assignee, setAssignee] = useState('');
  const [label, setLabel] = useState('');
  const [status, setStatus] = useState(issue.status);
  const hasConflict = conflictIssueIds.includes(issue.id);

  const quickUpdate = useMutation({
    mutationFn: async (payload: Record<string, unknown>) =>
      apiRequest(`/issues/${issue.id}`, { method: 'PATCH', body: JSON.stringify({ ...payload, updated_at: issue.updated_at }) }),
    onMutate: async (payload) => {
      beginPendingAction();
      await queryClient.cancelQueries({ queryKey: qk.issues(filterKey), exact: true });
      const previous = queryClient.getQueryData<Issue[]>(qk.issues(filterKey));
      const previousIssue = previous?.find((item) => item.id === issue.id);
      if (previous) {
        queryClient.setQueryData<Issue[]>(
          qk.issues(filterKey),
          previous.map((item) => (item.id === issue.id ? { ...item, ...payload } : item))
        );
      }
      return { previous, previousIssue };
    },
    onSuccess: () => {
      clearConflictIssue(issue.id);
    },
    onError: (error, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(qk.issues(filterKey), context.previous);
      if (context?.previousIssue?.status) setStatus(context.previousIssue.status);
      const message = error instanceof Error ? error.message : 'Unable to apply quick issue update.';
      const isConflict = /409|conflict|optimistic|updated_at/i.test(message);
      if (isConflict) {
        registerConflictIssue(issue.id);
      }
      toast({
        title: isConflict ? 'Conflict detected' : 'Quick action failed',
        description: isConflict ? 'Open the issue detail page for guided conflict resolution.' : message,
        variant: 'error'
      });
    },
    onSettled: () => {
      finishPendingAction();
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
    <div className={`group rounded-md border p-3 ${active ? 'ring-2 ring-blue-200' : ''} ${hasConflict ? 'border-amber-300 bg-amber-50/50 ring-2 ring-amber-200' : ''}`}>
      <Link href={`/issues/${issue.id}`} className="block hover:underline">
        <div className="flex items-center gap-2 font-medium">
          <span>{issue.title}</span>
          {hasConflict ? <Badge className="border-amber-200 bg-amber-100 text-amber-900">⚠ zero-data-loss conflict</Badge> : null}
        </div>
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
          <Input aria-label="Assignee UUID" value={assignee} onChange={(event) => setAssignee(event.target.value)} placeholder="Assignee UUID" />
          <Button size="sm" variant="outline" onClick={() => quickUpdate.mutate({ assignee_id: assignee || null })}>Save</Button>
        </div>
        <div className="flex gap-1">
          <Input aria-label="Label" value={label} onChange={(event) => setLabel(event.target.value)} placeholder="Label" />
          <Button size="sm" variant="outline" onClick={() => label && addLabel.mutate(label)}>Add</Button>
        </div>
      </div>
    </div>
  );
}
