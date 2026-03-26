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
    secret ? Object.entries(secret.data).map(([key, value]) => ({ key, value: value as string })) : [{ key: '', value: '' }]
  );
  const [loading, setLoading] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [showValues, setShowValues] = useState(false);

  const gitData = secret?.git_data;

  useEffect(() => {
    loadNamespaces();
  }, []);

  useEffect(() => {
    // Auto-select namespace ONLY if namespaceId prop was provided (from URL or edit mode)
    if (namespaceId && !selectedNamespace && namespaces.length > 0) {
      setSelectedNamespace(namespaceId);
    }
  }, [namespaces, selectedNamespace, namespaceId]);

  // Decode base64 values (Kubernetes Secret format)
  function decodeValue(base64Value: string): string {
    try {
      return atob(base64Value);
    } catch (e) {
      return base64Value; // If not base64, return as-is
    }
  }

  // Check if a key-value pair differs from Git
  function isDrifted(key: string, currentValue: string): boolean {
    if (!gitData) return false;
    const gitValue = gitData[key];
    if (!gitValue) return false; // Key doesn't exist in Git
    return decodeValue(currentValue) !== decodeValue(gitValue);
  }

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
    // Encode the value to base64 when updating
    updated[index].value = btoa(value);
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
    <form onSubmit={handleSubmit} className="max-w-6xl mx-auto">
      {/* Show/Hide Values Toggle (top right) */}
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-2xl font-bold">
          {mode === 'create' ? 'Create Secret' : `Edit Secret: ${secret?.secret_name}`}
        </h2>
        <button
          type="button"
          onClick={() => setShowValues(!showValues)}
          className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
        >
          {showValues ? '🙈 Hide Values' : '👁️ Show Values'}
        </button>
      </div>

      {/* Legend (if Git comparison available) */}
      {gitData && (
        <div className="mb-4 p-3 bg-blue-50 border border-blue-200 rounded-md text-sm">
          <strong>Legend:</strong> 
          <span className="ml-2">🟢 Matches Git</span>
          <span className="ml-4">🟡 Drifted (differs from Git)</span>
        </div>
      )}

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
          <option value="">Select namespace...</option>
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
          <div key={index} className="mb-6 p-4 border border-gray-200 rounded-md">
            {/* Key Name */}
            <div className="mb-4">
              <label className="block text-sm font-medium text-gray-700 mb-1">Key</label>
              <input
                id={`key-${index}`}
                type="text"
                value={kv.key}
                onChange={(e) => updateKey(index, e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-md"
                placeholder="key"
                aria-label="Key"
              />
            </div>

            {/* Two-Column Layout: Current vs Git */}
            {gitData ? (
              <div className="grid grid-cols-2 gap-4">
                {/* Current Draft Column */}
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Current Draft {isDrifted(kv.key, kv.value) ? '🟡' : '🟢'}
                  </label>
                  <input
                    id={`value-${index}`}
                    type={showValues ? 'text' : 'password'}
                    value={decodeValue(kv.value)}
                    onChange={(e) => updateValue(index, e.target.value)}
                    className="block w-full px-3 py-2 border border-gray-300 rounded-md"
                    placeholder="value"
                    aria-label="Value"
                  />
                  {isDrifted(kv.key, kv.value) && (
                    <p className="mt-1 text-xs text-yellow-600">⚠️ Modified (differs from Git)</p>
                  )}
                </div>

                {/* Git Version Column (Read-Only) */}
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">
                    Published Git Version
                  </label>
                  <input
                    type={showValues ? 'text' : 'password'}
                    value={gitData[kv.key] ? decodeValue(gitData[kv.key]) : '(not in Git)'}
                    readOnly
                    className="block w-full px-3 py-2 border border-gray-200 rounded-md bg-gray-50 text-gray-600 cursor-not-allowed"
                    aria-label="Git Version"
                  />
                </div>
              </div>
            ) : (
              /* Single Column (no Git comparison) */
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Value</label>
                <input
                  id={`value-${index}`}
                  type={showValues ? 'text' : 'password'}
                  value={decodeValue(kv.value)}
                  onChange={(e) => updateValue(index, e.target.value)}
                  className="block w-full px-3 py-2 border border-gray-300 rounded-md"
                  placeholder="value"
                  aria-label="Value"
                />
              </div>
            )}

            {/* Delete Key Button */}
            {keyValues.length > 1 && (
              <button
                type="button"
                onClick={() => removeKeyValue(index)}
                className="mt-3 text-sm text-red-600 hover:text-red-800"
              >
                Delete Key
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
