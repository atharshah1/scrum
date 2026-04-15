'use client';

import Link from 'next/link';
import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import { useAppStore } from '@/store/useAppStore';
import type { Issue } from '@/types';

export default function ProjectsPage() {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const selectedProjectId = useAppStore((s) => s.selectedProjectId);
  const setSelectedProject = useAppStore((s) => s.setSelectedProject);
  const filters = useAppStore((s) => s.issueFilters);
  const setIssueFilter = useAppStore((s) => s.setIssueFilter);
  const queryClient = useQueryClient();

  const issuesQuery = useQuery({
    queryKey: qk.issues(JSON.stringify({ selectedProjectId, filters })),
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
      apiRequest('/issues/', {
        method: 'POST',
        body: JSON.stringify({ project_id: selectedProjectId, title, description })
      }),
    onSuccess: () => {
      setTitle('');
      setDescription('');
      queryClient.invalidateQueries({ queryKey: qk.issues() });
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
          <Input placeholder="Issue title" value={title} onChange={(e) => setTitle(e.target.value)} />
          <Input placeholder="Description" value={description} onChange={(e) => setDescription(e.target.value)} />
          <Button disabled={!selectedProjectId || !title || createIssue.isPending} onClick={() => createIssue.mutate()}>Create</Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Issues</CardTitle></CardHeader>
        <CardContent className="space-y-2">
          {issues.map((issue) => (
            <Link key={issue.id} href={`/issues/${issue.id}`} className="block rounded-md border p-3 hover:bg-accent">
              <div className="font-medium">{issue.title}</div>
              <div className="text-xs text-muted-foreground">{issue.status}</div>
            </Link>
          ))}
          {!issues.length && <p className="text-sm text-muted-foreground">No issues found.</p>}
        </CardContent>
      </Card>
    </div>
  );
}
