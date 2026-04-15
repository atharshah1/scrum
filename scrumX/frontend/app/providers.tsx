'use client';

import { useEffect, useState } from 'react';
import { MutationCache, QueryCache, QueryClient, QueryClientProvider, useQueryClient } from '@tanstack/react-query';
import { AuthBootstrap } from '@/components/auth/auth-bootstrap';
import { ToastList, toast } from '@/components/ui/toast';
import { realtimeClient } from '@/lib/websocket';
import { qk } from '@/lib/query-keys';
import { useAuthStore } from '@/store/useAuthStore';
import type { ScrumEvent } from '@/types';

function extractIssuePayload(event: ScrumEvent): { issueId?: string; projectId?: string } {
  const payload = event.payload as Record<string, unknown>;
  const issue = payload.issue as Record<string, unknown> | undefined;
  return {
    issueId: typeof issue?.id === 'string' ? issue.id : typeof payload.issue_id === 'string' ? payload.issue_id : undefined,
    projectId:
      typeof issue?.project_id === 'string'
        ? issue.project_id
        : typeof payload.project_id === 'string'
          ? payload.project_id
          : undefined
  };
}

function RealtimeBridge() {
  const queryClient = useQueryClient();
  const accessToken = useAuthStore((s) => s.tokens?.access_token);

  useEffect(() => {
    if (!accessToken) {
      realtimeClient.disconnect();
      return;
    }

    realtimeClient.connect();
    const unsub = realtimeClient.subscribe((event) => {
      const { issueId, projectId } = extractIssuePayload(event);

      if (event.type === 'issue.updated' || event.type === 'issue.created') {
        if (issueId) {
          queryClient.invalidateQueries({ queryKey: qk.issue(issueId) });
        }
        queryClient.invalidateQueries({
          predicate: (query) => {
            if (query.queryKey[0] !== 'issues') return false;
            if (!projectId) return true;
            return String(query.queryKey[1] ?? '').includes(projectId);
          }
        });
        queryClient.invalidateQueries({ queryKey: ['board'] });
      }
      if (event.type === 'issue.comment_created' || event.type === 'comment.added') {
        if (issueId) {
          queryClient.invalidateQueries({ queryKey: qk.comments(issueId) });
          queryClient.invalidateQueries({ queryKey: qk.issue(issueId) });
        } else {
          queryClient.invalidateQueries({ queryKey: ['issue-comments'] });
        }
      }
      if (event.type === 'sprint.updated' || event.type === 'sprint.started' || event.type === 'sprint.completed') {
        queryClient.invalidateQueries({ queryKey: ['issues'] });
      }
      if (event.type === 'automation.triggered') {
        queryClient.invalidateQueries({ queryKey: qk.automationRules });
      }
      if (event.type.startsWith('issue.') || event.type.startsWith('sprint.')) {
        queryClient.invalidateQueries({ queryKey: qk.notifications() });
      }
    });

    return () => {
      unsub();
      realtimeClient.disconnect();
    };
  }, [accessToken, queryClient]);

  return null;
}

export function Providers({ children }: { children: React.ReactNode }) {
  const [client] = useState(
    () =>
      new QueryClient({
        queryCache: new QueryCache({
          onError: (error) => {
            toast({ title: 'Request failed', description: (error as Error).message, variant: 'error' });
          }
        }),
        mutationCache: new MutationCache({
          onError: (error) => {
            toast({ title: 'Action failed', description: (error as Error).message, variant: 'error' });
          }
        }),
        defaultOptions: {
          queries: {
            staleTime: 15_000,
            gcTime: 600_000,
            retry: 2,
            refetchOnWindowFocus: false
          }
        }
      })
  );

  return (
    <QueryClientProvider client={client}>
      {children}
      <AuthBootstrap />
      <RealtimeBridge />
      <ToastList />
    </QueryClientProvider>
  );
}
