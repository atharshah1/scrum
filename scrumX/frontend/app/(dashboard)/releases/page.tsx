'use client';

import { useQuery } from '@tanstack/react-query';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';

const envs = ['dev', 'staging', 'prod'];

export default function ReleasesPage() {
  const releases = useQuery({
    queryKey: qk.releases,
    queryFn: () => apiRequest<Array<{ id: string; name: string; issues?: number }>>('/releases')
  });

  return (
    <Card>
      <CardHeader><CardTitle>Release management</CardTitle></CardHeader>
      <CardContent className="space-y-2">
        {(releases.data ?? []).map((release) => (
          <div key={release.id} className="rounded-md border p-3">
            <div className="font-medium">{release.name}</div>
            <div className="mt-2 flex gap-2">
              {envs.map((env) => <Badge key={env}>{env}</Badge>)}
            </div>
          </div>
        ))}
        {!releases.data?.length && <p className="text-sm text-muted-foreground">No releases available yet.</p>}
      </CardContent>
    </Card>
  );
}
