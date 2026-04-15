'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';

export default function IncidentsPage() {
  const incidents = useQuery({
    queryKey: qk.incidents,
    queryFn: () => apiRequest<Array<{ id: string; title: string; status: string }>>('/incidents')
  });

  return (
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
  );
}
