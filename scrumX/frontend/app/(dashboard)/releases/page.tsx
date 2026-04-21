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

const envs = ['dev', 'staging', 'prod'];

export default function ReleasesPage() {
  const releases = useQuery({
    queryKey: qk.releases,
    queryFn: () => apiRequest<Array<{ id: string; name: string; issues?: number }>>('/releases')
  });

  return (
    <div className="space-y-4">
      <ProductStatusNotice
        title="Release tracking is still a preview workflow."
        description="This screen is intentionally demoted until the core issue create → sync → conflict → resolve path is proven end to end."
        action={<Link href="/projects"><Button variant="outline">Work from the issue slice</Button></Link>}
      />
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
    </div>
  );
}
