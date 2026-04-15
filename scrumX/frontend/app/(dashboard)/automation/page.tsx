'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { AutomationBuilder } from '@/components/automation/automation-builder';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Skeleton } from '@/components/ui/skeleton';
import { apiRequest } from '@/lib/api';
import { canManageAutomation } from '@/lib/permissions';
import { qk } from '@/lib/query-keys';
import { useAuthStore } from '@/store/useAuthStore';
import type { AutomationRule } from '@/types';

export default function AutomationPage() {
  const user = useAuthStore((s) => s.user);
  const canEdit = canManageAutomation(user?.role);

  const rulesQuery = useQuery({
    queryKey: qk.automationRules,
    queryFn: () => apiRequest<AutomationRule[]>('/automation/rules')
  });
  const emptyStateAction = canEdit
    ? <Button onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}>Create first rule</Button>
    : <Link className="text-sm underline" href="/settings">Request admin access</Link>;

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Automation</h1>
      {!canEdit ? (
        <EmptyState
          title="Read-only automation"
          description="Only admins can create or edit rules."
          hint="You can still inspect existing rules below."
        />
      ) : null}
      <AutomationBuilder canEdit={canEdit} />
      <Card>
        <CardHeader><CardTitle>Saved rules</CardTitle></CardHeader>
        <CardContent className="space-y-2">
          {rulesQuery.isPending ? (
            <>
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
            </>
          ) : (
            (rulesQuery.data ?? []).map((rule) => (
              <div key={rule.id ?? rule.name} className="rounded-md border p-2 text-sm">{rule.name} • {rule.trigger}</div>
            ))
          )}
          {!rulesQuery.isPending && !rulesQuery.data?.length ? (
            <EmptyState
              title="No automation rules yet"
              description="Start from a template and automate repetitive workflow updates."
              action={emptyStateAction}
              hint="Good first rule: auto-assign QA when status changes to done."
            />
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
