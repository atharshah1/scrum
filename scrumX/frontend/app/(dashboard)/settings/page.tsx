'use client';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';

export default function SettingsPage() {
  return (
    <div className="space-y-4">
      <Card>
        <CardHeader><CardTitle>Organization management</CardTitle></CardHeader>
        <CardContent className="grid gap-2 md:grid-cols-2">
          <Input placeholder="Organization name" />
          <Input placeholder="Workspace slug" />
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>User roles & Integrations</CardTitle></CardHeader>
        <CardContent className="grid gap-2 md:grid-cols-2">
          <Input placeholder="GitHub integration token" />
          <Input placeholder="Jira integration token" />
          <Input placeholder="Webhook URL" />
          <Input placeholder="Default role" />
        </CardContent>
      </Card>
    </div>
  );
}
