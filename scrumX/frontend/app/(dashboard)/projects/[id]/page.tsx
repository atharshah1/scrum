'use client';

import { useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { AIIssueDraft, Issue } from '@/types';

export default function ProjectDetailPage() {
  const params = useParams<{ id: string }>();
  const projectId = params.id;
  const queryClient = useQueryClient();
  const [text, setText] = useState('');
  const [drafts, setDrafts] = useState<AIIssueDraft[]>([]);

  const issuesQuery = useQuery({
    queryKey: qk.issues(`project-${projectId}`),
    queryFn: () => apiRequest<Issue[]>(`/issues?project_id=${projectId}`)
  });

  const generateIssueDrafts = async (sourceText: string) =>
    apiRequest<AIIssueDraft[]>('/ai/issues/from-text', {
      method: 'POST',
      body: JSON.stringify({ text: sourceText, project_id: projectId })
    });

  const generateMutation = useMutation({
    mutationFn: generateIssueDrafts,
    onSuccess: (items) => setDrafts(items)
  });

  const bulkCreateMutation = useMutation({
    mutationFn: async () => {
      const created = await Promise.all(
        drafts.map((draft) =>
          apiRequest<Issue>('/issues/', {
            method: 'POST',
            body: JSON.stringify({
              project_id: projectId,
              title: draft.title,
              description: draft.description ?? '',
              issue_type: draft.type,
              priority: draft.priority
            })
          })
        )
      );
      return created;
    },
    onSuccess: () => {
      setDrafts([]);
      queryClient.invalidateQueries({ queryKey: qk.issues(`project-${projectId}`), exact: true });
    }
  });

  const canConfirm = useMemo(
    () => drafts.length > 0 && drafts.every((draft) => draft.title.trim().length > 0),
    [drafts]
  );

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader><CardTitle>Project {projectId}</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <Textarea
            value={text}
            onChange={(event) => setText(event.target.value)}
            placeholder="Describe what needs to be built..."
          />
          <Button onClick={() => generateMutation.mutate(text)} disabled={!text.trim() || generateMutation.isPending}>
            ✨ Generate Issues
          </Button>
        </CardContent>
      </Card>

      {drafts.length > 0 ? (
        <Card>
          <CardHeader><CardTitle>Generated issues</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {drafts.map((draft, index) => (
              <div key={index} className="grid gap-2 rounded-md border p-3 md:grid-cols-[2fr_2fr_1fr_1fr_auto]">
                <Input
                  value={draft.title}
                  onChange={(event) =>
                    setDrafts((items) => items.map((item, itemIndex) => (itemIndex === index ? { ...item, title: event.target.value } : item)))
                  }
                  placeholder="Title"
                />
                <Input
                  value={draft.description ?? ''}
                  onChange={(event) =>
                    setDrafts((items) => items.map((item, itemIndex) => (itemIndex === index ? { ...item, description: event.target.value } : item)))
                  }
                  placeholder="Description"
                />
                <Select
                  value={draft.type}
                  onChange={(event) =>
                    setDrafts((items) =>
                      items.map((item, itemIndex) =>
                        itemIndex === index ? { ...item, type: event.target.value as AIIssueDraft['type'] } : item
                      )
                    )
                  }
                >
                  <option value="epic">epic</option>
                  <option value="story">story</option>
                  <option value="task">task</option>
                  <option value="bug">bug</option>
                </Select>
                <Select
                  value={draft.priority}
                  onChange={(event) =>
                    setDrafts((items) =>
                      items.map((item, itemIndex) =>
                        itemIndex === index ? { ...item, priority: event.target.value as AIIssueDraft['priority'] } : item
                      )
                    )
                  }
                >
                  <option value="low">low</option>
                  <option value="medium">medium</option>
                  <option value="high">high</option>
                </Select>
                <Button variant="outline" onClick={() => setDrafts((items) => items.filter((_, itemIndex) => itemIndex !== index))}>
                  Delete
                </Button>
              </div>
            ))}
            <Button onClick={() => bulkCreateMutation.mutate()} disabled={!canConfirm || bulkCreateMutation.isPending}>
              Confirm & bulk create
            </Button>
          </CardContent>
        </Card>
      ) : null}

      <Card>
        <CardHeader><CardTitle>Project issues</CardTitle></CardHeader>
        <CardContent className="space-y-2">
          {(issuesQuery.data ?? []).map((issue) => (
            <div key={issue.id} className="rounded-md border p-3 text-sm">{issue.title}</div>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}
