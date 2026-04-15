'use client';

import Link from 'next/link';
import { useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { useFormFields, required } from '@/components/forms/use-form';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import { useAppStore } from '@/store/useAppStore';
import type { Issue } from '@/types';

export default function ProjectsPage() {
  const selectedProjectId = useAppStore((s) => s.selectedProjectId);
  const setSelectedProject = useAppStore((s) => s.setSelectedProject);
  const filters = useAppStore((s) => s.issueFilters);
  const setIssueFilter = useAppStore((s) => s.setIssueFilter);
  const queryClient = useQueryClient();
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

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Projects & Issues</h1>
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
            disabled={!selectedProjectId || createIssue.isPending || !createIssueForm.isValid}
            onClick={() => {
              if (!createIssueForm.validate()) return;
              createIssue.mutate();
            }}
          >
            Create
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
              <Link key={issue.id} href={`/issues/${issue.id}`} className="block rounded-md border p-3 hover:bg-accent">
                <div className="font-medium">{issue.title}</div>
                <div className="text-xs text-muted-foreground">{issue.status}</div>
              </Link>
            ))
          )}
          {!issuesQuery.isPending && !issues.length ? <p className="text-sm text-muted-foreground">No issues found.</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}
