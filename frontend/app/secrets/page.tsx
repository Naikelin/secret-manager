'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { api, type Namespace, type Secret } from '@/lib/api';

export default function SecretsPage() {
  const router = useRouter();
  const [namespaces, setNamespaces] = useState<Namespace[]>([]);
  const [selectedNs, setSelectedNs] = useState<string>('');
  const [secrets, setSecrets] = useState<Secret[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    loadNamespaces();
  }, []);

  useEffect(() => {
    if (selectedNs) {
      loadSecrets(selectedNs);
    }
  }, [selectedNs]);

  async function loadNamespaces() {
    try {
      const data = await api.getNamespaces();
      setNamespaces(data);
      if (data.length > 0) {
        setSelectedNs(data[0].id);
      }
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to load namespaces';
      console.error('Failed to load namespaces:', err);
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  }

  async function loadSecrets(nsId: string) {
    try {
      setLoading(true);
      const data = await api.getSecrets(nsId);
      setSecrets(data);
      setError('');
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to load secrets';
      console.error('Failed to load secrets:', err);
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  }

  async function handlePublish(secret: Secret) {
    if (!confirm(`Publish ${secret.secret_name} to Git?`)) return;
    try {
      await api.publishSecret(selectedNs, secret.secret_name);
      loadSecrets(selectedNs);
    } catch (err: any) {
      // Check if it's a drift conflict error
      if (err.response?.status === 409) {
        alert('❌ Cannot publish: This secret has unresolved drift.\n\n' +
              'Please resolve the drift first by going to the Drift page.');
        // Optionally redirect to drift page
        // router.push(`/drift?secret=${secret.secret_name}&namespace=${selectedNs}`);
      } else {
        alert('Failed to publish secret: ' + (err.response?.data?.error || err.message));
      }
    }
  }

  if (loading) return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-blue-50 p-8">
      <div className="max-w-6xl mx-auto">
        <div className="flex items-center justify-center h-64">
          <div className="text-center">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
            <p className="mt-4 text-gray-600">Loading...</p>
          </div>
        </div>
      </div>
    </div>
  );
  
  if (error && namespaces.length === 0) return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-blue-50 p-8">
      <div className="max-w-6xl mx-auto">
        <div className="bg-white rounded-xl shadow-md p-8 border-l-4 border-red-500">
          <div className="flex items-center gap-3 mb-4">
            <span className="text-3xl">❌</span>
            <h2 className="text-xl font-semibold text-gray-900">Error Loading Data</h2>
          </div>
          <p className="text-red-600 mb-4">{error}</p>
          <button
            onClick={() => {
              setError('');
              setLoading(true);
              loadNamespaces();
            }}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors duration-200"
          >
            🔄 Retry
          </button>
        </div>
      </div>
    </div>
  );

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-blue-50 p-8">
      <div className="max-w-6xl mx-auto">
        <div className="flex justify-between items-center mb-8">
          <div>
            <h1 className="text-4xl font-bold text-gray-900">Secrets</h1>
            <p className="mt-2 text-gray-600">Manage Kubernetes secrets with GitOps workflow</p>
          </div>
          <div className="flex items-center gap-4">
            {/* Drift Warning */}
            {secrets.some(s => s.status === 'drifted') && (
              <a
                href="/drift"
                className="px-4 py-2 bg-yellow-100 text-yellow-800 rounded-lg hover:bg-yellow-200 flex items-center gap-2 font-medium transition-colors duration-200 shadow-sm"
              >
                ⚠️ Drift Detected
              </a>
            )}
            <button
              onClick={() => router.push(`/secrets/new?namespace=${selectedNs}`)}
              className="px-6 py-2 bg-gradient-to-r from-blue-600 to-purple-600 text-white rounded-lg hover:from-blue-700 hover:to-purple-700 font-medium transition-all duration-200 shadow-md hover:shadow-lg disabled:opacity-50 disabled:cursor-not-allowed"
              disabled={!selectedNs}
            >
              + Create Secret
            </button>
          </div>
        </div>

        {/* Namespace Selector */}
        <div className="mb-6 bg-white rounded-xl shadow-md p-6">
          <label className="block text-sm font-semibold text-gray-700 mb-2">Namespace</label>
          <select
            value={selectedNs}
            onChange={(e) => setSelectedNs(e.target.value)}
            className="w-full max-w-md px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all duration-200"
          >
            {namespaces.map((ns) => (
              <option key={ns.id} value={ns.id}>
                {ns.name} ({ns.cluster})
              </option>
            ))}
          </select>
        </div>

        {error && (
          <div className="mb-6 bg-red-50 border-l-4 border-red-500 rounded-lg p-4 shadow-sm">
            <div className="flex items-center gap-2">
              <span className="text-red-600 font-medium">❌ Error:</span>
              <p className="text-red-700">{error}</p>
            </div>
            <button
              onClick={() => {
                setError('');
                loadSecrets(selectedNs);
              }}
              className="mt-3 px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 text-sm font-medium transition-colors duration-200"
            >
              🔄 Retry
            </button>
          </div>
        )}

        {/* Secrets List */}
        <div className="bg-white rounded-xl shadow-md overflow-hidden">
          <table className="w-full">
            <thead className="bg-gradient-to-r from-gray-50 to-blue-50 border-b border-gray-200">
              <tr>
                <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">Name</th>
                <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">Status</th>
                <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">Keys</th>
                <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">Updated</th>
                <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {secrets.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-6 py-16 text-center">
                    <div className="flex flex-col items-center justify-center">
                      <span className="text-6xl mb-4">🔐</span>
                      <h3 className="text-lg font-semibold text-gray-900 mb-2">No secrets found</h3>
                      <p className="text-gray-500 mb-4">Create your first secret to get started!</p>
                      <button
                        onClick={() => router.push(`/secrets/new?namespace=${selectedNs}`)}
                        className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium transition-colors duration-200"
                      >
                        + Create Secret
                      </button>
                    </div>
                  </td>
                </tr>
              ) : (
                secrets.map((secret) => (
                  <tr key={secret.id} className="hover:bg-gray-50 transition-colors duration-150">
                    <td className="px-6 py-4 font-semibold text-gray-900">{secret.secret_name}</td>
                    <td className="px-6 py-4">
                      <span className={`px-3 py-1 text-xs font-semibold rounded-full ${
                        secret.status === 'published' ? 'bg-green-100 text-green-800' :
                        secret.status === 'drifted' ? 'bg-red-100 text-red-800' :
                        'bg-gray-100 text-gray-800'
                      }`}>
                        {secret.status === 'drifted' ? '⚠️ drifted' : secret.status}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-600">
                      {Object.keys(secret.data).join(', ')}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {new Date(secret.updated_at).toLocaleString()}
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-3">
                        <button
                          onClick={() => router.push(`/secrets/edit/${secret.secret_name}?namespace=${selectedNs}`)}
                          className="text-blue-600 hover:text-blue-800 font-medium hover:underline transition-colors duration-150"
                          title={secret.status !== 'draft' ? 'Editing will create a new draft version' : 'Edit secret'}
                        >
                          Edit
                          {secret.status !== 'draft' && (
                            <span className="ml-1 text-xs">✏️</span>
                          )}
                        </button>
                        {secret.status === 'drifted' ? (
                          <a
                            href={`/drift?secret=${secret.secret_name}&namespace=${selectedNs}`}
                            className="text-yellow-600 hover:text-yellow-800 font-medium hover:underline transition-colors duration-150 flex items-center gap-1"
                            title="Resolve drift before publishing"
                          >
                            ⚠️ Resolve Drift
                          </a>
                        ) : (secret.status === 'draft' || secret.status === 'published') && (
                          <button
                            onClick={() => handlePublish(secret)}
                            className="text-green-600 hover:text-green-800 font-medium hover:underline transition-colors duration-150"
                            title={secret.status === 'draft' ? 'Publish to Git' : 'Re-publish changes to Git'}
                          >
                            {secret.status === 'draft' ? 'Publish' : 'Re-Publish'}
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
