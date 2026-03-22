'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { isAuthenticated } from '@/lib/api';

export default function Home() {
  const router = useRouter();

  useEffect(() => {
    // Redirect authenticated users to dashboard
    if (isAuthenticated()) {
      router.push('/dashboard');
    }
  }, [router]);

  return (
    <main className="flex min-h-screen flex-col items-center justify-center p-24">
      <div className="text-center">
        <h1 className="text-4xl font-bold mb-4">Secret Manager</h1>
        <p className="text-muted-foreground mb-8">
          GitOps-based secret management with SOPS encryption
        </p>
        <a 
          href="/auth/login"
          className="inline-block px-6 py-3 bg-primary text-primary-foreground rounded-lg hover:opacity-90 transition-opacity"
        >
          Get Started
        </a>
      </div>
    </main>
  );
}
