'use client';

import { Suspense } from 'react';
import { useEffect, useState } from 'react';
import { useParams, useSearchParams } from 'next/navigation';
import { SecretForm } from '@/components/SecretForm';
import { api, type Secret } from '@/lib/api';

function EditSecretContent() {
  const params = useParams();
  const searchParams = useSearchParams();
  const name = params.name as string;
  const namespaceId = searchParams.get('namespace') || '';
  const [secret, setSecret] = useState<Secret | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (namespaceId && name) {
      // Fetch secret with Git version for comparison
      api.getSecret(namespaceId, name, true)
        .then(setSecret)
        .catch(() => alert('Failed to load secret'))
        .finally(() => setLoading(false));
    }
  }, [namespaceId, name]);

  if (loading) return <div className="p-8">Loading...</div>;
  if (!secret) return <div className="p-8">Secret not found</div>;

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <h1 className="text-3xl font-bold mb-8">Edit Secret</h1>
      <SecretForm namespaceId={namespaceId} secret={secret} mode="edit" />
    </div>
  );
}

export default function EditSecretPage() {
  return (
    <Suspense fallback={<div className="p-8">Loading...</div>}>
      <EditSecretContent />
    </Suspense>
  );
}
