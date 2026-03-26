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
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function loadSecret() {
      if (!namespaceId || !name) {
        setError('Missing namespace or secret name');
        setLoading(false);
        return;
      }

      setLoading(true);
      setError(null);
      
      try {
        // Fetch secret with Git version for comparison
        const secret = await api.getSecret(namespaceId, name, true);
        setSecret(secret);
      } catch (err: any) {
        setError(err.message || 'Failed to load secret');
      } finally {
        setLoading(false);
      }
    }

    loadSecret();
  }, [namespaceId, name]);

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 p-8">
        <div className="max-w-6xl mx-auto text-center">
          <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900"></div>
          <p className="mt-4 text-gray-600">Loading secret...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-gray-50 p-8">
        <div className="max-w-6xl mx-auto">
          <div className="p-4 bg-red-50 border border-red-200 rounded-md text-red-800">
            <strong>Error:</strong> {error}
          </div>
          <button
            onClick={() => window.history.back()}
            className="mt-4 px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
          >
            ← Go Back
          </button>
        </div>
      </div>
    );
  }

  if (!secret) {
    return (
      <div className="min-h-screen bg-gray-50 p-8">
        <div className="max-w-6xl mx-auto text-center text-gray-600">
          Secret not found
        </div>
      </div>
    );
  }

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
