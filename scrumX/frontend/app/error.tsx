'use client';

import { Button } from '@/components/ui/button';

export default function GlobalError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <html lang="en">
      <body>
        <main className="flex min-h-screen items-center justify-center p-6">
          <div className="w-full max-w-lg rounded-lg border bg-white p-6 shadow-sm">
            <h2 className="text-lg font-semibold">Something went wrong</h2>
            <p className="mt-2 text-sm text-muted-foreground">{error.message || 'Unexpected application error.'}</p>
            <Button className="mt-4" onClick={reset}>Try again</Button>
          </div>
        </main>
      </body>
    </html>
  );
}
