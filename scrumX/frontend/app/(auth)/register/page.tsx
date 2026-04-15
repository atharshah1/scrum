'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useMutation } from '@tanstack/react-query';
import { register } from '@/lib/auth';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';

export default function RegisterPage() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const signUp = useMutation({
    mutationFn: async () => register(email, password),
    onSuccess: () => router.push('/dashboard')
  });

  return (
    <main className="flex min-h-screen items-center justify-center bg-muted/30 p-4">
      <Card className="w-full max-w-md">
        <CardHeader><CardTitle>Create your scrumX workspace</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <Input placeholder="Email" value={email} onChange={(e) => setEmail(e.target.value)} />
          <Input placeholder="Password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
          <Button onClick={() => signUp.mutate()} disabled={signUp.isPending || !email || !password} className="w-full">Register</Button>
          {signUp.error && <p className="text-sm text-red-600">{(signUp.error as Error).message}</p>}
        </CardContent>
      </Card>
    </main>
  );
}
