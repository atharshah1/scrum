import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';

export default function BoardLoading() {
  return (
    <Card>
      <CardHeader><CardTitle>Board</CardTitle></CardHeader>
      <CardContent className="space-y-3">
        <Skeleton className="h-6 w-32" />
        <div className="grid gap-4 md:grid-cols-3">
          <Skeleton className="h-40" />
          <Skeleton className="h-40" />
          <Skeleton className="h-40" />
        </div>
      </CardContent>
    </Card>
  );
}
