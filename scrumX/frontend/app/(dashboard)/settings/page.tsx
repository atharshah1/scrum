'use client';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Input } from '@/components/ui/input';
import { canManageOrgSettings } from '@/lib/permissions';
import { useAuthStore } from '@/store/useAuthStore';

const migrationSteps = [
  '1. Connect Jira source',
  '2. Preview issues, epics, and users',
  '3. Map statuses and users',
  '4. Import with progress tracking',
  '5. Validate unmatched users and failed records'
];

export default function SettingsPage() {
  const user = useAuthStore((s) => s.user);
  const canManage = canManageOrgSettings(user?.role);

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl font-semibold">Migration & Settings</h1>
        <p className="text-sm text-muted-foreground">Use Jira as a migration source, not as the product frame.</p>
      </div>

      {!canManage ? (
        <EmptyState
          title="Limited settings access"
          description="Only admins can edit organization settings, migration credentials, and import mappings."
          hint="You can still review the migration flow and ask an admin to run it."
        />
      ) : null}

      <Card>
        <CardHeader><CardTitle>Jira migration flow</CardTitle></CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-2">
          <div className="space-y-2 text-sm text-muted-foreground">
            {migrationSteps.map((step) => (
              <div key={step} className="rounded-md border p-3">{step}</div>
            ))}
          </div>
          <div className="grid gap-2">
            <Input placeholder="Jira base URL" readOnly={!canManage} />
            <Input placeholder="Jira email" readOnly={!canManage} />
            <Input placeholder="Jira API token" readOnly={!canManage} />
            <Input placeholder="Status mapping profile" readOnly={!canManage} />
            <Input placeholder="User mapping profile" readOnly={!canManage} />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Import validation</CardTitle></CardHeader>
        <CardContent className="grid gap-2 md:grid-cols-2">
          <Input placeholder="Last import run ID" readOnly={!canManage} />
          <Input placeholder="Unmatched users report" readOnly={!canManage} />
          <Input placeholder="Failed records report" readOnly={!canManage} />
          <Input placeholder="Post-import verification" readOnly={!canManage} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Organization defaults</CardTitle></CardHeader>
        <CardContent className="grid gap-2 md:grid-cols-2">
          <Input placeholder="Organization name" readOnly={!canManage} />
          <Input placeholder="Workspace slug" readOnly={!canManage} />
          <Input placeholder="Default role" readOnly={!canManage} />
          <Input placeholder="Webhook URL" readOnly={!canManage} />
        </CardContent>
      </Card>
    </div>
  );
}
