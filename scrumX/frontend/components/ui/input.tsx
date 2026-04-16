import * as React from 'react';
import { cn } from '@/lib/utils';

export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(({ className, ...props }, ref) => (
  <input
    ref={ref}
    className={cn('flex h-9 w-full rounded-md border bg-background px-3 py-1 text-sm outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring', className)}
    {...props}
  />
));
Input.displayName = 'Input';
