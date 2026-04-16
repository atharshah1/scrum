'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { useFormFields, required } from '@/components/forms/use-form';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { IssueComment } from '@/types';

export function IssueComments({ issueId, comments }: { issueId: string; comments: IssueComment[] }) {
  const form = useFormFields(
    { body: '' },
    {
      body: [required('Comment')]
    }
  );
  const queryClient = useQueryClient();

  const createComment = useMutation({
    mutationFn: async () => apiRequest<IssueComment>(`/issues/${issueId}/comments`, { method: 'POST', body: JSON.stringify({ body: form.values.body }) }),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: qk.comments(issueId) });
      const previous = queryClient.getQueryData<IssueComment[]>(qk.comments(issueId));
      const optimistic: IssueComment = {
        id: `tmp-${Date.now()}`,
        issue_id: issueId,
        author_name: 'You',
        body: form.values.body,
        created_at: new Date().toISOString()
      };
      queryClient.setQueryData<IssueComment[]>(qk.comments(issueId), [optimistic, ...(previous ?? [])]);
      return { previous };
    },
    onError: (_error, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(qk.comments(issueId), context.previous);
      }
    },
    onSuccess: () => {
      form.reset();
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: qk.comments(issueId), exact: true });
    }
  });

  return (
    <div className="space-y-3">
      <div className="space-y-2 rounded-md border p-3">
        <Textarea placeholder="Write a comment..." value={form.values.body} onChange={(e) => form.setField('body', e.target.value)} />
        {form.errors.body ? <p className="text-xs text-red-600">{form.errors.body}</p> : null}
        <Button
          onClick={() => {
            if (!form.validate()) return;
            createComment.mutate();
          }}
          disabled={createComment.isPending || !form.isValid}
        >
          Add comment
        </Button>
      </div>
      {comments.map((comment) => (
        <div key={comment.id} className="rounded-md border p-3 text-sm">
          <div className="mb-1 text-xs text-muted-foreground">
            {(comment.author_name || comment.author_email || 'Unknown user')} • {new Date(comment.created_at).toLocaleString()}
          </div>
          {comment.body}
        </div>
      ))}
    </div>
  );
}
