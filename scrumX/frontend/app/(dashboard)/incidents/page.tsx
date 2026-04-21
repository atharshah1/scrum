'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { ProductStatusNotice } from '@/components/ui/product-status-notice';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';

export default function IncidentsPage() {
  const incidents = useQuery({
    queryKey: qk.incidents,
    queryFn: () => apiRequest<Array<{ id: string; title: string; status: string }>>('/incidents')
  });

  return (
    <div className="space-y-4">
      <ProductStatusNotice
        title="Incidents are still a preview surface."
        description="Keep the validated workflow anchored on issues and conflict review until incident handling is proven through the same trust path."
        action={<Link href="/projects"><Button variant="outline">Open issue workspace</Button></Link>}
      />
      <Card>
        <CardHeader><CardTitle>Incident management</CardTitle></CardHeader>
        <CardContent className="space-y-2">
          {(incidents.data ?? []).map((incident) => (
            <div key={incident.id} className="rounded-md border p-3">
              <div className="font-medium">{incident.title}</div>
              <Badge className="mt-2">{incident.status}</Badge>
            </div>
          ))}
          {!incidents.data?.length ? (
            <EmptyState
              title="No incidents available yet"
              description="Capture incidents quickly to track impact and remediation."
              action={<Link href="/projects"><Button>Create linked issue</Button></Link>}
              hint="On-call tip: create a bug issue first, then attach incident details."
            />
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
