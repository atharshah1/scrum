'use client';

import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Textarea } from '@/components/ui/textarea';
import { toast } from '@/components/ui/toast';
import { apiRequest } from '@/lib/api';
import { comingSoonContent, features, getAIIssueDraftMocks } from '@/lib/features';
import { qk } from '@/lib/query-keys';
import type { AIIssueDraft, Issue } from '@/types';

export default function ProjectDetailPage() {
  const params = useParams<{ id: string }>();
  const projectId = params.id;
  const [text, setText] = useState('');
  const [mockDrafts, setMockDrafts] = useState<AIIssueDraft[]>([]);

  useEffect(() => {
    setMockDrafts((current) => {
      if (current.length === 0) return current;
      return [];
    });
  }, [text]);

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
            placeholder="Describe the work, branch context, or desired outcome..."
          />
          <Button
            onClick={() => {
              if (!features.AI) {
                const mocks = getAIIssueDraftMocks(text);
                setMockDrafts(mocks);
                toast({
                  title: comingSoonContent.AI.title,
                  description: `${comingSoonContent.AI.description} ${comingSoonContent.AI.hint}`
                });
                return;
              }
              toast({
                title: 'Smart drafting',
                description: 'Feature flag is enabled, but the final drafting workflow is still being finalized.'
              });
            }}
            disabled={!text.trim()}
          >
            Draft issue breakdown
          </Button>
          <p className="text-xs text-muted-foreground">
            Start with deterministic issue breakdowns first. Optional AI can layer on later without changing the core workflow.
          </p>
          {!features.AI && mockDrafts.length ? (
            <div className="space-y-2 rounded-md border p-3 text-sm">
              <p className="text-xs font-medium text-muted-foreground">Draft preview</p>
              {mockDrafts.map((draft, index) => (
                <div key={`${draft.title}-${index}`} className="rounded-md border p-2">
                  <p className="font-medium">{draft.title}</p>
                  <p className="text-xs text-muted-foreground">{draft.description}</p>
                </div>
              ))}
            </div>
          ) : null}
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
