'use client';

import { useEffect, useState } from 'react';
import { QueryClient, QueryClientProvider, useQueryClient } from '@tanstack/react-query';
import { realtimeClient } from '@/lib/websocket';
import { useAuthStore } from '@/store/useAuthStore';

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
      if (event.type === 'issue.updated' || event.type === 'issue.created') {
        queryClient.invalidateQueries({ queryKey: ['issues'] });
        queryClient.invalidateQueries({ queryKey: ['board'] });
      }
      if (event.type === 'issue.comment_created' || event.type === 'comment.added') {
        queryClient.invalidateQueries({ queryKey: ['issue-comments'] });
      }
      if (event.type === 'sprint.updated' || event.type === 'sprint.started' || event.type === 'sprint.completed') {
        queryClient.invalidateQueries({ queryKey: ['issues'] });
      }
      if (event.type === 'automation.triggered') {
        queryClient.invalidateQueries({ queryKey: ['automation-rules'] });
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
        defaultOptions: {
          queries: {
            staleTime: 15_000,
            gcTime: 10 * 60_000,
            retry: 2,
            refetchOnWindowFocus: false
          }
        }
      })
  );

  return (
    <QueryClientProvider client={client}>
      {children}
      <RealtimeBridge />
    </QueryClientProvider>
  );
}
