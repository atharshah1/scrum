'use client';

import { useEffect, useState } from 'react';
import { MutationCache, QueryCache, QueryClient, QueryClientProvider, useQueryClient } from '@tanstack/react-query';
import { AuthBootstrap } from '@/components/auth/auth-bootstrap';
import { ToastList, toast } from '@/components/ui/toast';
import { realtimeClient } from '@/lib/websocket';
import { qk } from '@/lib/query-keys';
import { useAppStore } from '@/store/useAppStore';
import { useAuthStore } from '@/store/useAuthStore';
import type { Board, ScrumEvent } from '@/types';

function extractIssuePayload(event: ScrumEvent): { issueId?: string; projectId?: string } {
  const payload = event.payload as Record<string, unknown>;
  const scope = event.scope as Record<string, unknown> | undefined;
  const issue = payload.issue as Record<string, unknown> | undefined;
  return {
    issueId: typeof issue?.id === 'string' ? issue.id : typeof payload.issue_id === 'string' ? payload.issue_id : undefined,
    projectId:
      typeof scope?.project_id === 'string'
        ? scope.project_id
        : typeof issue?.project_id === 'string'
        ? issue.project_id
        : typeof payload.project_id === 'string'
          ? payload.project_id
          : undefined
  };
}

function RealtimeBridge() {
  const queryClient = useQueryClient();
  const accessToken = useAuthStore((s) => s.tokens?.access_token);
  const orgId = useAuthStore((s) => s.orgId);
  const selectedProjectId = useAppStore((s) => s.selectedProjectId);

  useEffect(() => {
    if (!accessToken) {
      realtimeClient.disconnect();
      return;
    }

    realtimeClient.connect(accessToken, orgId ?? undefined, selectedProjectId ?? undefined);
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
        queryClient.invalidateQueries({
          predicate: (query) => {
            if (query.queryKey[0] !== 'board') return false;
            if (!projectId) return false;
            const board = query.state.data as Board | undefined;
            return board?.project_id === projectId;
          }
        });
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
  }, [accessToken, orgId, selectedProjectId, queryClient]);

  return null;
}

export function Providers({ children }: { children: React.ReactNode }) {
  const [client] = useState(
    () => {
      const queryClient = new QueryClient({
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
            // Baseline defaults for general CRUD screens: responsive enough for collaboration,
            // while avoiding excessive refetch churn for large workspaces.
            staleTime: 30_000,
            gcTime: 600_000,
            retry: 2,
            refetchOnWindowFocus: false
          }
        }
      });

      // Issue lists/boards change frequently while users triage work, so keep these fairly fresh.
      queryClient.setQueryDefaults(['issues'], { staleTime: 20_000, gcTime: 900_000 });
      queryClient.setQueryDefaults(['board'], { staleTime: 45_000, gcTime: 900_000 });
      // Automation rules change less often and are typically admin-driven configuration.
      queryClient.setQueryDefaults(['automation-rules'], { staleTime: 120_000, gcTime: 1_200_000 });
      // Notifications should refresh more aggressively to keep unread indicators responsive.
      queryClient.setQueryDefaults(['notifications'], { staleTime: 8_000, gcTime: 300_000 });
      return queryClient;
    }
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
