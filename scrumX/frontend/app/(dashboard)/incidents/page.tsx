'use client';

import { useQuery } from '@tanstack/react-query';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
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
        {!incidents.data?.length && <p className="text-sm text-muted-foreground">No incidents available yet.</p>}
      </CardContent>
    </Card>
  );
}
