'use client';

import { ReactNode } from 'react';
import { Badge } from '@/components/ui/badge';

export function ProductStatusNotice({
  badge = 'Preview surface',
  title,
  description,
  action
}: {
  badge?: string;
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="rounded-md border border-amber-200 bg-amber-50/70 p-4 text-sm text-amber-950">
      <div className="mb-2">
        <Badge className="border-amber-300 bg-amber-100 text-amber-900">{badge}</Badge>
      </div>
      <p className="font-medium">{title}</p>
      <p className="mt-1 text-amber-900/80">{description}</p>
      {action ? <div className="mt-3">{action}</div> : null}
    </div>
  );
}
