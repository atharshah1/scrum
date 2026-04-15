'use client';

import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { IssueComment } from '@/types';

export function IssueComments({ issueId, comments }: { issueId: string; comments: IssueComment[] }) {
  const [body, setBody] = useState('');
  const queryClient = useQueryClient();

  const createComment = useMutation({
    mutationFn: async () => apiRequest(`/issues/${issueId}/comments`, { method: 'POST', body: JSON.stringify({ body }) }),
    onSuccess: () => {
      setBody('');
      queryClient.invalidateQueries({ queryKey: qk.comments(issueId) });
    }
  });

  return (
    <div className="space-y-3">
      <div className="space-y-2 rounded-md border p-3">
        <Textarea placeholder="Write a comment..." value={body} onChange={(e) => setBody(e.target.value)} />
        <Button onClick={() => createComment.mutate()} disabled={!body.trim() || createComment.isPending}>Add comment</Button>
      </div>
      {comments.map((comment) => (
        <div key={comment.id} className="rounded-md border p-3 text-sm">
          <div className="mb-1 text-xs text-muted-foreground">{new Date(comment.created_at).toLocaleString()}</div>
          {comment.body}
        </div>
      ))}
    </div>
  );
}
