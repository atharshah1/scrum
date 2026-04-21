'use client';

import { useEffect } from 'react';
import { Sidebar } from '@/components/layout/sidebar';
import { Topbar } from '@/components/layout/topbar';
import { CommandPalette } from '@/components/layout/command-palette';
import { TrustBar } from '@/components/layout/trust-bar';
import { useAppStore } from '@/store/useAppStore';

export function AppShell({ children }: { children: React.ReactNode }) {
  const theme = useAppStore((s) => s.theme);
  useEffect(() => {
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }, [theme]);

  return (
    <div className="flex min-h-screen bg-muted/40">
      <Sidebar />
      <div className="flex min-h-screen flex-1 flex-col">
        <Topbar />
        <TrustBar />
        <main className="flex-1 p-4">{children}</main>
      </div>
      <CommandPalette />
    </div>
  );
}
