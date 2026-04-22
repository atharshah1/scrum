'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/button';
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
  const [activeStep, setActiveStep] = useState(0);
  const [importComplete, setImportComplete] = useState(false);

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
        <CardContent className="grid gap-4 md:grid-cols-2">
          <div className="space-y-3">
            <div className="space-y-2 text-sm text-muted-foreground">
              {migrationSteps.map((step, index) => {
                const state = importComplete ? 'done' : index < activeStep ? 'done' : index === activeStep ? 'active' : 'upcoming';
                return (
                  <div
                    key={step}
                    className={`rounded-md border p-3 ${
                      state === 'active'
                        ? 'border-blue-200 bg-blue-50 text-blue-950'
                        : state === 'done'
                          ? 'border-emerald-200 bg-emerald-50 text-emerald-950'
                          : ''
                    }`}
                  >
                    <div className="font-medium">{step}</div>
                    <div className="mt-1 text-xs uppercase tracking-wide">
                      {state === 'active' ? 'In progress' : state === 'done' ? 'Complete' : 'Waiting'}
                    </div>
                  </div>
                );
              })}
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                variant="outline"
                onClick={() => {
                  setImportComplete(false);
                  setActiveStep((step) => Math.max(0, step - 1));
                }}
                disabled={activeStep === 0 && !importComplete}
              >
                Previous step
              </Button>
              <Button
                onClick={() => {
                  if (activeStep >= migrationSteps.length - 1) {
                    setImportComplete(true);
                    return;
                  }
                  setActiveStep((step) => Math.min(migrationSteps.length - 1, step + 1));
                }}
              >
                {activeStep >= migrationSteps.length - 1 ? 'Mark import complete' : 'Advance step'}
              </Button>
            </div>
            {importComplete ? (
              <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-950">
                <div className="font-medium">Import complete</div>
                <p className="text-emerald-900/80">The migration checklist is complete. Review unmatched users and failed records before cutting over.</p>
              </div>
            ) : (
              <div className="rounded-md border border-blue-200 bg-blue-50 p-3 text-sm text-blue-950">
                <div className="font-medium">Current step</div>
                <p className="text-blue-900/80">{migrationSteps[activeStep]}</p>
              </div>
            )}
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
