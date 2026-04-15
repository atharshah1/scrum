'use client';

import { useState } from 'react';
import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Textarea } from '@/components/ui/textarea';
import { toast } from '@/components/ui/toast';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { Issue } from '@/types';

export default function ProjectDetailPage() {
  const params = useParams<{ id: string }>();
  const projectId = params.id;
  const [text, setText] = useState('');

  const issuesQuery = useQuery({
    queryKey: qk.issues(`project-${projectId}`),
    queryFn: () => apiRequest<Issue[]>(`/issues?project_id=${projectId}`)
  });

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
          <Button
            onClick={() =>
              toast({
                title: 'AI Issue Generation',
                description: 'Coming soon 🚧. This button is intentionally non-blocking for current workflows.'
              })
            }
            disabled={!text.trim()}
          >
            ✨ Generate Issues
          </Button>
        </CardContent>
      </Card>

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
