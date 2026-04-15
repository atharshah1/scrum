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
    if (mockDrafts.length > 0) {
      setMockDrafts([]);
    }
  }, [mockDrafts.length, text]);

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
                title: 'AI Issue Generation',
                description: 'Feature flag is enabled, but API workflow is still being finalized.'
              });
            }}
            disabled={!text.trim()}
          >
            ✨ Generate Issues
          </Button>
          {!features.AI ? (
            <p className="text-xs text-muted-foreground">
              {comingSoonContent.AI.description} {comingSoonContent.AI.hint}
            </p>
          ) : null}
          {!features.AI && mockDrafts.length ? (
            <div className="space-y-2 rounded-md border p-3 text-sm">
              <p className="text-xs font-medium text-muted-foreground">Dev mock preview</p>
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
