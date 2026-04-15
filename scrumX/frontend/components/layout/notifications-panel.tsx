'use client';

import { Bell } from 'lucide-react';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { Notification } from '@/types';

export function NotificationsPanel() {
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();

  const notificationsQuery = useQuery({
    queryKey: qk.notifications('topbar'),
    queryFn: () => apiRequest<Notification[]>('/notifications?limit=20')
  });

  const markRead = useMutation({
    mutationFn: async (id: string) => apiRequest(`/notifications/${id}/read`, { method: 'PATCH' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: qk.notifications('topbar') });
    }
  });

  const markAllRead = useMutation({
    mutationFn: async () => apiRequest('/notifications/read-all', { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: qk.notifications('topbar') });
    }
  });

  const notifications = notificationsQuery.data ?? [];
  const unreadCount = notifications.filter((item) => !item.is_read).length;

  return (
    <div className="relative">
      <Button variant="outline" size="sm" onClick={() => setOpen((v) => !v)} className="relative">
        <Bell className="h-4 w-4" />
        {unreadCount > 0 ? <Badge className="absolute -right-2 -top-2 px-1 py-0 text-[10px]">{unreadCount}</Badge> : null}
      </Button>
      {open ? (
        <div className="absolute right-0 z-40 mt-2 w-[360px] rounded-md border bg-white shadow-lg">
          <div className="flex items-center justify-between border-b p-3">
            <div className="text-sm font-semibold">Notifications</div>
            <button
              type="button"
              className="text-xs text-muted-foreground hover:text-foreground"
              onClick={() => markAllRead.mutate()}
              disabled={markAllRead.isPending || unreadCount === 0}
            >
              Mark all read
            </button>
          </div>
          <div className="max-h-[360px] overflow-auto p-2">
            {notifications.map((item) => (
              <button
                key={item.id}
                type="button"
                className="mb-1 w-full rounded-md border p-2 text-left text-xs hover:bg-accent"
                onClick={() => {
                  if (!item.is_read) {
                    markRead.mutate(item.id);
                  }
                }}
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="font-medium">{item.title}</div>
                  {!item.is_read ? <Badge className="bg-blue-100 text-blue-900">New</Badge> : null}
                </div>
                <div className="mt-1 text-muted-foreground">{item.message}</div>
              </button>
            ))}
            {!notifications.length ? <div className="p-2 text-xs text-muted-foreground">No notifications yet.</div> : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}
