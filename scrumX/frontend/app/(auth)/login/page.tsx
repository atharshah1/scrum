'use client';

import { useRouter } from 'next/navigation';
import { useMutation } from '@tanstack/react-query';
import { login } from '@/lib/auth';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { toast } from '@/components/ui/toast';
import { email, minLength, required, useFormFields } from '@/components/forms/use-form';

export default function LoginPage() {
  const router = useRouter();
  const form = useFormFields(
    { email: '', password: '' },
    {
      email: [required('Email'), email('Email')],
      password: [required('Password'), minLength('Password', 8)]
    }
  );

  const signIn = useMutation({
    mutationFn: async () => login(form.values.email, form.values.password),
    onSuccess: () => {
      toast({ title: 'Welcome back', variant: 'success' });
      router.push('/dashboard');
    }
  });

  const submit = () => {
    if (!form.validate()) return;
    signIn.mutate();
  };

  return (
    <main className="flex min-h-screen items-center justify-center bg-muted/30 p-4">
      <Card className="w-full max-w-md">
        <CardHeader><CardTitle>Login to scrumX</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <div>
            <Input placeholder="Email" value={form.values.email} onChange={(e) => form.setField('email', e.target.value)} />
            {form.errors.email ? <p className="mt-1 text-xs text-red-600">{form.errors.email}</p> : null}
          </div>
          <div>
            <Input placeholder="Password" type="password" value={form.values.password} onChange={(e) => form.setField('password', e.target.value)} />
            {form.errors.password ? <p className="mt-1 text-xs text-red-600">{form.errors.password}</p> : null}
          </div>
          <Button onClick={submit} disabled={signIn.isPending || !form.isValid} className="w-full">Sign in</Button>
          {signIn.error ? <p className="text-sm text-red-600">{(signIn.error as Error).message}</p> : null}
        </CardContent>
      </Card>
    </main>
  );
}
