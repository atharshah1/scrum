'use client';

import { useQuery } from '@tanstack/react-query';
import { AutomationBuilder } from '@/components/automation/automation-builder';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { AutomationRule } from '@/types';

export default function AutomationPage() {
  const rulesQuery = useQuery({
    queryKey: qk.automationRules,
    queryFn: () => apiRequest<AutomationRule[]>('/automation/rules')
  });

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Automation</h1>
      <AutomationBuilder />
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
          {!rulesQuery.isPending && !rulesQuery.data?.length ? <p className="text-sm text-muted-foreground">No automation rules yet.</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}
