'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { api, type Secret, type Namespace } from '@/lib/api';

interface SecretFormProps {
  namespaceId: string;
  secret?: Secret;
  mode: 'create' | 'edit';
}

export function SecretForm({ namespaceId, secret, mode }: SecretFormProps) {
  const router = useRouter();
  const [name, setName] = useState(secret?.secret_name || '');
  const [selectedNamespace, setSelectedNamespace] = useState(namespaceId || secret?.namespace_id || '');
  const [namespaces, setNamespaces] = useState<Namespace[]>([]);
  const [keyValues, setKeyValues] = useState<Array<{ key: string; value: string }>>(
    secret ? Object.entries(secret.data).map(([key, value]) => ({ key, value })) : [{ key: '', value: '' }]
  );
  const [loading, setLoading] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    loadNamespaces();
  }, []);

  useEffect(() => {
    // Auto-select first namespace if none selected and namespaces loaded
    if (!selectedNamespace && namespaces.length > 0) {
      setSelectedNamespace(namespaces[0].id);
    }
  }, [namespaces, selectedNamespace]);

  async function loadNamespaces() {
    try {
      const data = await api.getNamespaces();
      setNamespaces(data);
    } catch (err) {
      console.error('Failed to load namespaces:', err);
    }
  }

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
    setErrors({});

    // Validation
    const newErrors: Record<string, string> = {};
    
    if (!name.trim()) {
      newErrors.name = 'Secret name is required';
    }

    if (!selectedNamespace) {
      newErrors.namespace = 'Namespace is required';
    }

    const data = keyValues.reduce((acc, kv) => {
      if (kv.key.trim()) {
        acc[kv.key] = kv.value;
      }
      return acc;
    }, {} as Record<string, string>);

    if (Object.keys(data).length === 0) {
      newErrors.keyValues = 'At least one key-value pair required';
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors);
      return;
    }

    try {
      setLoading(true);
      if (mode === 'create') {
        await api.createSecret(selectedNamespace, { name, data });
        router.push('/secrets?message=Secret draft created');
      } else {
        await api.updateSecret(selectedNamespace, name, data);
        router.push('/secrets?message=Secret updated');
      }
    } catch (err) {
      setErrors({ submit: err instanceof Error ? err.message : 'Failed to save secret' });
    } finally {
      setLoading(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="max-w-3xl mx-auto">
      <div className="mb-6">
        <label htmlFor="secret-name" className="block text-sm font-medium mb-2">Secret Name</label>
        <input
          id="secret-name"
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
        {errors.name && <p className="text-sm text-red-600 mt-1">{errors.name}</p>}
      </div>

      <div className="mb-6">
        <label htmlFor="namespace" className="block text-sm font-medium mb-2">Namespace</label>
        <select
          id="namespace"
          value={selectedNamespace}
          onChange={(e) => setSelectedNamespace(e.target.value)}
          disabled={mode === 'edit'}
          className="w-full px-3 py-2 border rounded disabled:bg-gray-100"
        >
          {!selectedNamespace && <option value="">Select namespace...</option>}
          {namespaces.map((ns) => (
            <option key={ns.id} value={ns.id}>
              {ns.name} ({ns.cluster})
            </option>
          ))}
        </select>
        {errors.namespace && <p className="text-sm text-red-600 mt-1">{errors.namespace}</p>}
      </div>

      <div className="mb-6">
        <label className="block text-sm font-medium mb-2">Key-Value Pairs</label>
        {keyValues.map((kv, index) => (
          <div key={index} className="flex gap-2 mb-2">
            <input
              id={`key-${index}`}
              type="text"
              value={kv.key}
              onChange={(e) => updateKey(index, e.target.value)}
              className="flex-1 px-3 py-2 border rounded"
              placeholder="key"
              aria-label="Key"
            />
            <input
              id={`value-${index}`}
              type="text"
              value={kv.value}
              onChange={(e) => updateValue(index, e.target.value)}
              className="flex-1 px-3 py-2 border rounded"
              placeholder="value"
              aria-label="Value"
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
          Add Key
        </button>
        {errors.keyValues && <p className="text-sm text-red-600 mt-1">{errors.keyValues}</p>}
      </div>

      {errors.submit && <div className="mb-4 p-3 bg-red-50 text-red-600 rounded">{errors.submit}</div>}

      <div className="flex gap-4">
        <button
          type="submit"
          disabled={loading}
          className="px-6 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
        >
          {loading ? 'Saving...' : mode === 'create' ? 'Create Draft' : 'Save Draft'}
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
