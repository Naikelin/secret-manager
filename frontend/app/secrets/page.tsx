'use client';

import { Suspense, useEffect, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { api, type Namespace, type Secret } from '@/lib/api';
import { ConfirmDialog } from '@/components/ConfirmDialog';

function SecretsContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [namespaces, setNamespaces] = useState<Namespace[]>([]);
  const [selectedNs, setSelectedNs] = useState<string>('');
  const [secrets, setSecrets] = useState<Secret[]>([]);
  const [filteredSecrets, setFilteredSecrets] = useState<Secret[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [publishDialog, setPublishDialog] = useState<{ secret: Secret } | null>(null);
  const [deleteDialog, setDeleteDialog] = useState<{ secret: Secret } | null>(null);
  const [unpublishDialog, setUnpublishDialog] = useState<{ secret: Secret } | null>(null);
  const [successMessage, setSuccessMessage] = useState('');

  useEffect(() => {
    // Check for success message in URL params
    const message = searchParams.get('message');
    if (message) {
      setSuccessMessage(message);
      setTimeout(() => setSuccessMessage(''), 5000); // Increased to 5 seconds
      // Clean up URL
      window.history.replaceState({}, '', '/secrets');
    }
  }, [searchParams]);

  useEffect(() => {
    loadNamespaces();
  }, []);

  useEffect(() => {
    if (selectedNs) {
      loadSecrets(selectedNs);
    }
  }, [selectedNs]);

  useEffect(() => {
    // Filter secrets based on search query
    if (!searchQuery.trim()) {
      setFilteredSecrets(secrets);
    } else {
      const query = searchQuery.toLowerCase();
      setFilteredSecrets(
        secrets.filter((s) => s.secret_name.toLowerCase().includes(query))
      );
    }
  }, [searchQuery, secrets]);

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
      setFilteredSecrets(data);
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
    setPublishDialog({ secret });
  }

  async function confirmPublish() {
    if (!publishDialog) return;
    
    const secret = publishDialog.secret;
    setPublishDialog(null);
    
    try {
      await api.publishSecret(selectedNs, secret.secret_name);
      setSuccessMessage('Secret published');
      setTimeout(() => setSuccessMessage(''), 5000); // Increased to 5 seconds
      loadSecrets(selectedNs);
    } catch (err: any) {
      // Check if it's a drift conflict error
      if (err.response?.status === 409) {
        setError('❌ Cannot publish: This secret has unresolved drift. Please resolve the drift first by going to the Drift page.');
      } else {
        setError('Failed to publish secret: ' + (err.response?.data?.error || err.message));
      }
    }
  }

  async function handleUnpublish(secret: Secret) {
    setUnpublishDialog({ secret });
  }

  async function confirmUnpublish() {
    if (!unpublishDialog) return;
    const secret = unpublishDialog.secret;
    setUnpublishDialog(null);
    
    try {
      await api.unpublishSecret(selectedNs, secret.secret_name);
      setSuccessMessage('Secret unpublished and reverted to draft');
      loadSecrets(selectedNs);
    } catch (err: any) {
      setError('Failed to unpublish secret: ' + (err.message || 'Unknown error'));
    }
  }

  async function handleDelete(secret: Secret) {
    if (secret.status === 'published') {
      return; // Should not reach here due to disabled button
    }
    setDeleteDialog({ secret });
  }

  async function confirmDelete() {
    if (!deleteDialog) return;
    const secret = deleteDialog.secret;
    setDeleteDialog(null);
    
    try {
      const response = await api.deleteSecret(selectedNs, secret.secret_name);
      
      // Handle drifted secret reset (returns 200 with message)
      if (response && typeof response === 'object' && 'message' in response) {
        setSuccessMessage(response.message || 'Secret reset to Git state');
      } else {
        setSuccessMessage('Secret deleted');
      }
      
      loadSecrets(selectedNs);
    } catch (err: any) {
      setError('Failed to delete secret: ' + (err.message || 'Unknown error'));
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
          <label htmlFor="namespace-select" className="block text-sm font-semibold text-gray-700 mb-2">Namespace</label>
          <select
            id="namespace-select"
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

        {/* Search */}
        <div className="mb-6 bg-white rounded-xl shadow-md p-6">
          <label htmlFor="search-secrets" className="block text-sm font-semibold text-gray-700 mb-2">Search</label>
          <input
            id="search-secrets"
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search secrets..."
            className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all duration-200"
          />
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
                <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">Namespace</th>
                <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">Status</th>
                <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {filteredSecrets.length === 0 ? (
                <tr>
                  <td colSpan={4} className="px-6 py-16 text-center">
                    <div className="flex flex-col items-center justify-center">
                      <span className="text-6xl mb-4">🔐</span>
                      <h3 className="text-lg font-semibold text-gray-900 mb-2">
                        {searchQuery ? 'No secrets found' : 'No secrets found'}
                      </h3>
                      <p className="text-gray-500 mb-4">
                        {searchQuery ? 'Try a different search term' : 'Create your first secret to get started!'}
                      </p>
                      {!searchQuery && (
                        <button
                          onClick={() => router.push(`/secrets/new?namespace=${selectedNs}`)}
                          className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium transition-colors duration-200"
                        >
                          Create Secret
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ) : (
                filteredSecrets.map((secret) => {
                  const namespace = namespaces.find(ns => ns.id === selectedNs);
                  return (
                    <tr key={secret.id} className="hover:bg-gray-50 transition-colors duration-150">
                      <td className="px-6 py-4 font-semibold text-gray-900">
                        <div className="flex items-center gap-2">
                          {secret.secret_name}
                          {secret.status !== 'draft' && (
                            <a
                              href={`https://github.com/Naikelin/secret-manager-gitops/blob/main/namespaces/${encodeURIComponent(namespace?.name || '')}/secrets/${encodeURIComponent(secret.secret_name)}.yaml`}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-gray-400 hover:text-blue-600 transition-colors"
                              title="View in GitHub"
                            >
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                                      d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                              </svg>
                            </a>
                          )}
                        </div>
                      </td>
                      <td className="px-6 py-4 text-sm text-gray-600">{namespace?.name || selectedNs}</td>
                      <td className="px-6 py-4">
                        <span className={`px-3 py-1 text-xs font-semibold rounded-full ${
                          secret.status === 'published' ? 'bg-green-100 text-green-800' :
                          secret.status === 'drifted' ? 'bg-red-100 text-red-800' :
                          'bg-gray-100 text-gray-800'
                        }`}>
                          {secret.status === 'drifted' ? '⚠️ drifted' : secret.status}
                        </span>
                      </td>
                      <td className="px-6 py-4">
                        <div className="flex items-center gap-3">
                          {/* Edit button - always available */}
                          <button
                            onClick={() => router.push(`/secrets/edit/${secret.secret_name}?namespace=${selectedNs}`)}
                            className="text-blue-600 hover:text-blue-800 font-medium hover:underline"
                          >
                            Edit
                          </button>
                          
                          {/* Publish/Unpublish/Resolve Drift */}
                          {secret.status === 'drifted' ? (
                            <a
                              href={`/drift?secret=${secret.secret_name}&namespace=${selectedNs}`}
                              className="text-yellow-600 hover:text-yellow-800 font-medium hover:underline flex items-center gap-1"
                            >
                              ⚠️ Resolve Drift
                            </a>
                          ) : secret.status === 'published' ? (
                            <button
                              onClick={() => handleUnpublish(secret)}
                              className="text-orange-600 hover:text-orange-800 font-medium hover:underline"
                              title="Remove from Git and revert to draft"
                            >
                              Unpublish
                            </button>
                          ) : (
                            <button
                              onClick={() => handlePublish(secret)}
                              className="text-green-600 hover:text-green-800 font-medium hover:underline"
                            >
                              Publish
                            </button>
                          )}
                          
                          {/* Delete/Reset button */}
                          {secret.status === 'drifted' ? (
                            <button
                              onClick={() => handleDelete(secret)}
                              className="text-orange-600 hover:text-orange-800 font-medium hover:underline"
                              title="Discard local changes and sync from Git"
                            >
                              🔄 Reset to Git
                            </button>
                          ) : (
                            <button
                              onClick={() => handleDelete(secret)}
                              disabled={secret.status === 'published'}
                              className="text-red-600 hover:text-red-800 font-medium hover:underline disabled:opacity-50 disabled:cursor-not-allowed"
                              title={secret.status === 'published' 
                                ? 'Cannot delete published secrets. Use Unpublish to remove from Git.' 
                                : 'Delete secret'}
                            >
                              Delete
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Success Message */}
      {successMessage && (
        <div 
          className="fixed top-4 right-4 bg-green-500 text-white px-6 py-3 rounded-lg shadow-lg z-50"
          data-testid="success-message"
        >
          {successMessage}
        </div>
      )}

      {/* Publish Confirmation Dialog */}
      <ConfirmDialog
        isOpen={!!publishDialog}
        title="Publish Secret"
        message={`Are you sure you want to publish "${publishDialog?.secret.secret_name}" to Git?`}
        confirmLabel="Confirm Publish"
        confirmVariant="primary"
        onConfirm={confirmPublish}
        onCancel={() => setPublishDialog(null)}
      />

      {/* Delete Confirmation Dialog */}
      <ConfirmDialog
        isOpen={!!deleteDialog}
        title={deleteDialog?.secret.status === 'drifted' ? 'Reset Secret to Git' : 'Delete Secret'}
        message={
          deleteDialog?.secret.status === 'drifted'
            ? `Reset "${deleteDialog.secret.secret_name}" to the version in Git? This will discard any local drift.`
            : `Are you sure you want to delete "${deleteDialog?.secret.secret_name}"? This action cannot be undone.`
        }
        confirmLabel={deleteDialog?.secret.status === 'drifted' ? 'Confirm Reset' : 'Confirm Delete'}
        confirmVariant="danger"
        onConfirm={confirmDelete}
        onCancel={() => setDeleteDialog(null)}
      />

      {/* Unpublish Confirmation Dialog */}
      <ConfirmDialog
        isOpen={!!unpublishDialog}
        title="Unpublish Secret"
        message={`Are you sure you want to unpublish "${unpublishDialog?.secret.secret_name}"? This will remove the secret from Git and revert it to draft status.`}
        confirmLabel="Confirm Unpublish"
        confirmVariant="primary"
        onConfirm={confirmUnpublish}
        onCancel={() => setUnpublishDialog(null)}
      />
    </div>
  );
}

export default function SecretsPage() {
  return (
    <Suspense fallback={
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
    }>
      <SecretsContent />
    </Suspense>
  );
}
