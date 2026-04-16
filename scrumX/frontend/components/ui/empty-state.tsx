import { ReactNode } from 'react';

export function EmptyState({
  title,
  description,
  action,
  hint
}: {
  title: string;
  description: string;
  action?: ReactNode;
  hint?: string;
}) {
  return (
    <div className="rounded-md border border-dashed bg-muted/20 p-6 text-center">
      <p className="text-base font-medium">{title}</p>
      <p className="mt-1 text-sm text-muted-foreground">{description}</p>
      {action ? <div className="mt-3 flex justify-center">{action}</div> : null}
      {hint ? <p className="mt-2 text-xs text-muted-foreground">{hint}</p> : null}
    </div>
  );
}
