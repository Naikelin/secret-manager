'use client';

import { useState, useEffect, useRef, useLayoutEffect } from 'react';
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
  const [formData, setFormData] = useState<Record<string, string>>(secret?.data || {});
  const [loading, setLoading] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [showValues, setShowValues] = useState(false);
  const [showOnlyDrifted, setShowOnlyDrifted] = useState(false);
  const [resettingKey, setResettingKey] = useState<string | null>(null);
  const focusedKeyRef = useRef<string | null>(null);

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

  // Restore focus to key input after state updates
  useLayoutEffect(() => {
    if (focusedKeyRef.current) {
      // Find the input with data-key attribute matching the focused key
      const input = document.querySelector(`input[data-key="${focusedKeyRef.current}"]`) as HTMLInputElement;
      if (input) {
        input.focus();
        // Move cursor to end of input
        input.setSelectionRange(input.value.length, input.value.length);
      }
      focusedKeyRef.current = null; // Clear after restoration
    }
  }, [formData]);

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

  // Get list of keys to display based on filter
  function getDisplayKeys(): string[] {
    const allKeys = Object.keys(formData);
    
    if (!showOnlyDrifted || !gitData) {
      return allKeys;
    }
    
    // Filter to only show drifted keys
    return allKeys.filter(key => isDrifted(key, formData[key]));
  }

  // Reset a specific key to its Git value
  function handleResetToGit(key: string) {
    if (!gitData || !gitData[key]) return;
    
    setResettingKey(key);
    
    // Copy Git value to current draft
    setFormData({
      ...formData,
      [key]: gitData[key] // Git value is already base64 encoded
    });
    
    // Clear resetting state after animation
    setTimeout(() => setResettingKey(null), 500);
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
    const newKey = `key${Object.keys(formData).length + 1}`;
    setFormData({ ...formData, [newKey]: btoa('') });
  }

  function handleDeleteKey(key: string) {
    const updated = { ...formData };
    delete updated[key];
    setFormData(updated);
  }

  function handleKeyChange(oldKey: string, newKey: string) {
    if (oldKey === newKey) return;
    focusedKeyRef.current = newKey; // Capture new key for focus restoration
    const updated = { ...formData };
    updated[newKey] = updated[oldKey];
    delete updated[oldKey];
    setFormData(updated);
  }

  function handleValueChange(key: string, base64Value: string) {
    setFormData(prev => ({ ...prev, [key]: base64Value }));
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

    if (Object.keys(formData).length === 0) {
      newErrors.keyValues = 'At least one key-value pair required';
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors);
      return;
    }

    try {
      setLoading(true);
      if (mode === 'create') {
        await api.createSecret(selectedNamespace, { name, data: formData });
        router.push('/secrets?message=Secret draft created');
      } else {
        await api.updateSecret(selectedNamespace, name, formData);
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
      {/* Header with Show/Hide and Filter */}
      <div className="flex justify-between items-center mb-4">
        <div className="flex items-center gap-4">
          <h2 className="text-xl font-semibold">
            {secret ? `Edit Secret: ${secret.secret_name}` : 'Create Secret'}
          </h2>
          
          {/* Status Badge */}
          {secret && (
            <span className={`px-2 py-1 text-xs font-medium rounded ${
              secret.status === 'published' ? 'bg-green-100 text-green-800' :
              secret.status === 'drifted' ? 'bg-yellow-100 text-yellow-800' :
              'bg-gray-100 text-gray-800'
            }`}>
              {secret.status}
            </span>
          )}
        </div>
        
        <div className="flex items-center gap-3">
          {/* Show Only Drifted Filter (only show if gitData exists) */}
          {gitData && (
            <label className="flex items-center gap-2 text-sm text-gray-700 cursor-pointer">
              <input
                type="checkbox"
                checked={showOnlyDrifted}
                onChange={(e) => setShowOnlyDrifted(e.target.checked)}
                className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              <span>Show only drifted</span>
            </label>
          )}
          
          {/* Show/Hide Values Toggle */}
          <button
            type="button"
            onClick={() => setShowValues(!showValues)}
            className="p-2 text-gray-500 hover:text-gray-700 rounded border border-gray-300 hover:border-gray-400 transition-colors"
            aria-label={showValues ? "Hide secret values" : "Show secret values"}
            title={showValues ? "Hide values" : "Show values"}
          >
            {showValues ? (
              // Closed eye (with slash)
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                      d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
              </svg>
            ) : (
              // Open eye
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                      d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                      d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
              </svg>
            )}
          </button>
        </div>
      </div>

      {/* Legend with tooltips */}
      {gitData && (
        <div className="mb-4 p-3 bg-blue-50 border border-blue-200 rounded-md text-sm">
          <div className="flex items-center gap-6">
            <strong>Legend:</strong>
            
            <div className="flex items-center gap-1" title="This key-value pair matches the version in Git">
              <span>🟢 Matches Git</span>
            </div>
            
            <div className="flex items-center gap-1" title="This key-value pair has been modified and differs from Git">
              <span>🟡 Drifted</span>
            </div>
            
            <div className="ml-auto text-xs text-blue-700">
              Comparing with Git SHA: {secret?.git_base_sha?.substring(0, 7) || 'unknown'}
            </div>
          </div>
        </div>
      )}

      {/* Warning if Git data failed to load for published/drifted secret */}
      {!gitData && secret?.status !== 'draft' && (
        <div className="mb-4 p-3 bg-yellow-50 border border-yellow-300 rounded-md text-sm text-yellow-800">
          ⚠️ <strong>Git comparison unavailable.</strong> Could not fetch the published version from Git. 
          You can still edit this secret, but drift detection is disabled.
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
        
        {/* Key-Value Pairs */}
        {getDisplayKeys().length === 0 ? (
          <div className="mb-6 p-8 text-center text-gray-500 border border-dashed border-gray-300 rounded-md">
            {showOnlyDrifted 
              ? "No drifted keys. All values match Git! 🎉" 
              : "No keys yet. Click 'Add Key-Value' to start."}
          </div>
        ) : (
          getDisplayKeys().map(key => (
            <div 
              key={key} 
              className={`mb-6 p-4 border rounded-md transition-all ${
                resettingKey === key 
                  ? 'border-green-500 bg-green-50' 
                  : isDrifted(key, formData[key])
                  ? 'border-yellow-300 bg-yellow-50'
                  : 'border-gray-200 bg-white'
              }`}
            >
              {/* Key Name */}
              <div className="mb-2">
                <label className="block text-sm font-medium text-gray-700">
                  Key
                  {isDrifted(key, formData[key]) && (
                    <span className="ml-2 text-xs text-yellow-600" title="This key has been modified">
                      🟡 Drifted
                    </span>
                  )}
                </label>
                <input
                  type="text"
                  value={key}
                  onChange={(e) => handleKeyChange(key, e.target.value)}
                  data-key={key}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                  placeholder="e.g., DB_PASSWORD"
                />
              </div>

              {/* Two-Column Layout: Current vs Git */}
              {gitData ? (
                <div className="grid grid-cols-2 gap-4">
                  {/* Current Draft Column */}
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      Current Draft
                      {isDrifted(key, formData[key]) ? (
                        <span className="ml-2 text-xs text-yellow-600">🟡</span>
                      ) : (
                        <span className="ml-2 text-xs text-green-600">🟢</span>
                      )}
                    </label>
                    <input
                      type={showValues ? 'text' : 'password'}
                      value={decodeValue(formData[key])}
                      onChange={(e) => handleValueChange(key, btoa(e.target.value))}
                      className="block w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                      placeholder="Enter secret value"
                    />
                    {isDrifted(key, formData[key]) && (
                      <p className="mt-1 text-xs text-yellow-600" title="Current value differs from Git">
                        ⚠️ Modified (differs from Git)
                      </p>
                    )}
                  </div>

                  {/* Git Version Column (Read-Only) */}
                  <div>
                    <label className="block text-sm font-medium text-gray-500 mb-1 flex items-center justify-between">
                      <span>Published Git Version</span>
                      
                      {/* Reset to Git Button */}
                      {gitData[key] && isDrifted(key, formData[key]) && (
                        <button
                          type="button"
                          onClick={() => handleResetToGit(key)}
                          className="text-xs px-2 py-1 text-blue-600 hover:text-blue-800 hover:bg-blue-50 rounded border border-blue-300 flex items-center gap-1"
                          title="Reset this key to the Git version"
                        >
                          ↺ Reset to Git
                        </button>
                      )}
                    </label>
                    <input
                      type={showValues ? 'text' : 'password'}
                      value={gitData[key] ? decodeValue(gitData[key]) : '(not in Git)'}
                      readOnly
                      className="block w-full px-3 py-2 border border-gray-200 rounded-md bg-gray-50 text-gray-600 cursor-not-allowed"
                      title="This is the published version from Git (read-only)"
                    />
                    {!gitData[key] && (
                      <p className="mt-1 text-xs text-gray-500">
                        ℹ️ This key doesn't exist in Git yet
                      </p>
                    )}
                  </div>
                </div>
              ) : (
                /* Single Column (no Git comparison) */
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Value</label>
                  <input
                    type={showValues ? 'text' : 'password'}
                    value={decodeValue(formData[key])}
                    onChange={(e) => handleValueChange(key, btoa(e.target.value))}
                    className="block w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                    placeholder="Enter secret value"
                  />
                </div>
              )}

              {/* Delete Key Button */}
              <button
                type="button"
                onClick={() => handleDeleteKey(key)}
                className="mt-3 text-sm text-red-600 hover:text-red-800 hover:underline"
              >
                🗑️ Delete Key
              </button>
            </div>
          ))
        )}
        
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

      {/* Form Actions */}
      <div className="flex items-center justify-between pt-6 border-t">
        <div className="text-sm text-gray-600">
          {gitData && (
            <span>
              {getDisplayKeys().filter(k => isDrifted(k, formData[k])).length} drifted key(s) · {' '}
              {getDisplayKeys().filter(k => !isDrifted(k, formData[k])).length} matching Git
            </span>
          )}
        </div>
        
        <div className="flex gap-3">
          <button
            type="button"
            onClick={() => router.push('/secrets')}
            className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={loading}
            className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700 disabled:opacity-50"
          >
            {loading ? 'Saving...' : (secret ? 'Save Changes' : 'Create Secret')}
          </button>
        </div>
      </div>
    </form>
  );
}
