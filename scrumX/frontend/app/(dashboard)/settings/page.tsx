'use client';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Input } from '@/components/ui/input';
import { canManageOrgSettings } from '@/lib/permissions';
import { useAuthStore } from '@/store/useAuthStore';

export default function SettingsPage() {
  const user = useAuthStore((s) => s.user);
  const canManage = canManageOrgSettings(user?.role);

  return (
    <div className="space-y-4">
      {!canManage ? (
        <EmptyState
          title="Limited settings access"
          description="Only admins can edit organization and integration settings."
          hint="You can still view current settings and ask an admin for changes."
        />
      ) : null}
      <Card>
        <CardHeader><CardTitle>Organization management</CardTitle></CardHeader>
        <CardContent className="grid gap-2 md:grid-cols-2">
          <Input placeholder="Organization name" readOnly={!canManage} />
          <Input placeholder="Workspace slug" readOnly={!canManage} />
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>User roles & Integrations</CardTitle></CardHeader>
        <CardContent className="grid gap-2 md:grid-cols-2">
          <Input placeholder="GitHub integration token" readOnly={!canManage} />
          <Input placeholder="Jira integration token" readOnly={!canManage} />
          <Input placeholder="Webhook URL" readOnly={!canManage} />
          <Input placeholder="Default role" readOnly={!canManage} />
        </CardContent>
      </Card>
    </div>
  );
}
