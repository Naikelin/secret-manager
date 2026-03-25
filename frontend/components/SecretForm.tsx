'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { api, type Secret } from '@/lib/api';

interface SecretFormProps {
  namespaceId: string;
  secret?: Secret;
  mode: 'create' | 'edit';
}

export function SecretForm({ namespaceId, secret, mode }: SecretFormProps) {
  const router = useRouter();
  const [name, setName] = useState(secret?.secret_name || '');
  const [keyValues, setKeyValues] = useState<Array<{ key: string; value: string }>>(
    secret ? Object.entries(secret.data).map(([key, value]) => ({ key, value })) : [{ key: '', value: '' }]
  );
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  function addKeyValue() {
    setKeyValues([...keyValues, { key: '', value: '' }]);
  }

  function removeKeyValue(index: number) {
    setKeyValues(keyValues.filter((_, i) => i !== index));
  }

  function updateKey(index: number, key: string) {
    const updated = [...keyValues];
    updated[index].key = key;
    setKeyValues(updated);
  }

  function updateValue(index: number, value: string) {
    const updated = [...keyValues];
    updated[index].value = value;
    setKeyValues(updated);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');

    // Validation
    if (!name.trim()) {
      setError('Secret name is required');
      return;
    }

    const data = keyValues.reduce((acc, kv) => {
      if (kv.key.trim()) {
        acc[kv.key] = kv.value;
      }
      return acc;
    }, {} as Record<string, string>);

    if (Object.keys(data).length === 0) {
      setError('At least one key-value pair is required');
      return;
    }

    try {
      setLoading(true);
      if (mode === 'create') {
        await api.createSecret(namespaceId, { name, data });
      } else {
        await api.updateSecret(namespaceId, name, data);
      }
      router.push('/secrets');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save secret');
    } finally {
      setLoading(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="max-w-3xl mx-auto">
      <div className="mb-6">
        <label className="block text-sm font-medium mb-2">Secret Name</label>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={mode === 'edit'}
          className="w-full px-3 py-2 border rounded disabled:bg-gray-100"
          placeholder="my-secret"
        />
        <p className="text-xs text-gray-500 mt-1">
          DNS-1123 compliant (lowercase, numbers, hyphens)
        </p>
      </div>

      <div className="mb-6">
        <label className="block text-sm font-medium mb-2">Key-Value Pairs</label>
        {keyValues.map((kv, index) => (
          <div key={index} className="flex gap-2 mb-2">
            <input
              type="text"
              value={kv.key}
              onChange={(e) => updateKey(index, e.target.value)}
              className="flex-1 px-3 py-2 border rounded"
              placeholder="key"
            />
            <input
              type="text"
              value={kv.value}
              onChange={(e) => updateValue(index, e.target.value)}
              className="flex-1 px-3 py-2 border rounded"
              placeholder="value"
            />
            {keyValues.length > 1 && (
              <button
                type="button"
                onClick={() => removeKeyValue(index)}
                className="px-3 py-2 text-red-600 hover:bg-red-50 rounded"
              >
                Remove
              </button>
            )}
          </div>
        ))}
        <button
          type="button"
          onClick={addKeyValue}
          className="mt-2 px-4 py-2 text-sm text-blue-600 hover:bg-blue-50 rounded"
        >
          + Add Key
        </button>
      </div>

      {error && <div className="mb-4 p-3 bg-red-50 text-red-600 rounded">{error}</div>}

      <div className="flex gap-4">
        <button
          type="submit"
          disabled={loading}
          className="px-6 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
        >
          {loading ? 'Saving...' : mode === 'create' ? 'Create Draft' : 'Update Draft'}
        </button>
        <button
          type="button"
          onClick={() => router.push('/secrets')}
          className="px-6 py-2 bg-gray-200 rounded hover:bg-gray-300"
        >
          Cancel
        </button>
      </div>
    </form>
  );
}
