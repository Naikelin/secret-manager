'use client';

import { Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import { SecretForm } from '@/components/SecretForm';

function NewSecretContent() {
  const searchParams = useSearchParams();
  const namespaceId = searchParams.get('namespace') || '';

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <h1 className="text-3xl font-bold mb-8">Create Secret</h1>
      <SecretForm namespaceId={namespaceId} mode="create" />
    </div>
  );
}

export default function NewSecretPage() {
  return (
    <Suspense fallback={<div className="p-8">Loading...</div>}>
      <NewSecretContent />
    </Suspense>
  );
}
