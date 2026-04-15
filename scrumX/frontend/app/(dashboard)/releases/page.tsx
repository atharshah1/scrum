'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
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
        {!releases.data?.length ? (
          <EmptyState
            title="No releases available yet"
            description="Link issues to a release to start tracking deployment readiness."
            action={<Link href="/projects"><Button>Prepare release scope</Button></Link>}
            hint="Tip: use labels like release:candidate to group scope quickly."
          />
        ) : null}
      </CardContent>
    </Card>
  );
}
