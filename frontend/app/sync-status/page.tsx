'use client';

import { useEffect, useState } from 'react';
import { api, type Namespace, type SyncStatus } from '@/lib/api';

interface NamespaceSyncState {
  namespace: Namespace;
  syncStatus: SyncStatus | null;
  loading: boolean;
  error: string | null;
}

export default function SyncStatusPage() {
  const [namespaceStates, setNamespaceStates] = useState<NamespaceSyncState[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    loadNamespacesAndSyncStatus();
  }, []);

  async function loadNamespacesAndSyncStatus() {
    try {
      setLoading(true);
      setError('');
      
      // Load all namespaces
      const namespaces = await api.getNamespaces();
      
      // Initialize state with loading flags
      const initialStates: NamespaceSyncState[] = namespaces.map(ns => ({
        namespace: ns,
        syncStatus: null,
        loading: true,
        error: null,
      }));
      setNamespaceStates(initialStates);
      
      // Load sync status for each namespace in parallel
      await Promise.all(
        namespaces.map(async (ns, index) => {
          try {
            const status = await api.getSyncStatus(ns.id);
            setNamespaceStates(prev => {
              const updated = [...prev];
              updated[index] = {
                ...updated[index],
                syncStatus: status,
                loading: false,
                error: null,
              };
              return updated;
            });
          } catch (err) {
            setNamespaceStates(prev => {
              const updated = [...prev];
              updated[index] = {
                ...updated[index],
                syncStatus: null,
                loading: false,
                error: err instanceof Error ? err.message : 'Failed to load sync status',
              };
              return updated;
            });
          }
        })
      );
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to load namespaces';
      console.error('Failed to load namespaces:', err);
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  }

  async function refreshSyncStatus(namespaceId: string, index: number) {
    setNamespaceStates(prev => {
      const updated = [...prev];
      updated[index] = {
        ...updated[index],
        loading: true,
        error: null,
      };
      return updated;
    });

    try {
      const status = await api.getSyncStatus(namespaceId);
      setNamespaceStates(prev => {
        const updated = [...prev];
        updated[index] = {
          ...updated[index],
          syncStatus: status,
          loading: false,
          error: null,
        };
        return updated;
      });
    } catch (err) {
      setNamespaceStates(prev => {
        const updated = [...prev];
        updated[index] = {
          ...updated[index],
          loading: false,
          error: err instanceof Error ? err.message : 'Failed to refresh sync status',
        };
        return updated;
      });
    }
  }

  function getStatusBadge(state: NamespaceSyncState) {
    if (state.loading) {
      return (
        <span className="inline-flex items-center px-4 py-2 text-sm font-semibold rounded-lg bg-gray-100 text-gray-700">
          ⏳ Checking...
        </span>
      );
    }

    if (state.error) {
      return (
        <span className="inline-flex items-center px-4 py-2 text-sm font-semibold rounded-lg bg-red-100 text-red-800">
          ❌ Error
        </span>
      );
    }

    if (!state.syncStatus) {
      return (
        <span className="inline-flex items-center px-4 py-2 text-sm font-semibold rounded-lg bg-gray-100 text-gray-700">
          ❓ Unknown
        </span>
      );
    }

    if (state.syncStatus.error) {
      return (
        <span className="inline-flex items-center px-4 py-2 text-sm font-semibold rounded-lg bg-red-100 text-red-800">
          ❌ Error
        </span>
      );
    }

    if (state.syncStatus.synced) {
      return (
        <span className="inline-flex items-center px-4 py-2 text-sm font-semibold rounded-lg bg-green-100 text-green-800">
          ✅ Synced
        </span>
      );
    }

    return (
      <span className="inline-flex items-center px-4 py-2 text-sm font-semibold rounded-lg bg-yellow-100 text-yellow-800">
        ⚠️ Out of Sync
      </span>
    );
  }

  function formatRelativeTime(timestamp: string): string {
    try {
      const date = new Date(timestamp);
      const now = Date.now();
      const diff = now - date.getTime();
      
      const seconds = Math.floor(diff / 1000);
      const minutes = Math.floor(seconds / 60);
      const hours = Math.floor(minutes / 60);
      const days = Math.floor(hours / 24);

      if (days > 7) {
        return date.toLocaleString();
      } else if (days > 0) {
        return `${days} day${days > 1 ? 's' : ''} ago`;
      } else if (hours > 0) {
        return `${hours} hour${hours > 1 ? 's' : ''} ago`;
      } else if (minutes > 0) {
        return `${minutes} minute${minutes > 1 ? 's' : ''} ago`;
      } else {
        return 'Just now';
      }
    } catch (err) {
      return timestamp;
    }
  }

  function truncateSha(sha: string): string {
    return sha.substring(0, 8);
  }

  if (loading && namespaceStates.length === 0) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-gray-50 to-purple-50 p-8">
        <div className="max-w-7xl mx-auto">
          <h1 className="text-4xl font-bold mb-8 text-gray-900">Sync Status Dashboard</h1>
          <div className="text-center py-16">
            <div className="animate-spin rounded-full h-16 w-16 border-b-4 border-purple-600 mx-auto"></div>
            <p className="mt-6 text-gray-600 text-lg">Loading namespaces...</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-purple-50 p-8">
      <div className="max-w-7xl mx-auto">
        <div className="flex justify-between items-center mb-8">
          <div>
            <h1 className="text-4xl font-bold text-gray-900">Sync Status Dashboard</h1>
            <p className="mt-2 text-lg text-gray-600">
              FluxCD synchronization status for all namespaces
            </p>
          </div>
          <button
            onClick={loadNamespacesAndSyncStatus}
            className="px-6 py-2 bg-gradient-to-r from-blue-600 to-purple-600 text-white rounded-lg hover:from-blue-700 hover:to-purple-700 font-medium transition-all duration-200 shadow-md hover:shadow-lg flex items-center gap-2"
          >
            🔄 Refresh All
          </button>
        </div>

        {error && (
          <div className="mb-6 bg-red-50 border-l-4 border-red-500 rounded-lg p-4 shadow-sm">
            <div className="flex items-center gap-2">
              <span className="text-red-600 font-medium">❌ Error:</span>
              <p className="text-red-700">{error}</p>
            </div>
          </div>
        )}

        {/* Info Box */}
        <div className="mb-6 bg-blue-50 border-l-4 border-blue-500 rounded-lg p-6 shadow-sm">
          <h3 className="font-semibold text-gray-900 mb-3 flex items-center gap-2">
            <span className="text-xl">ℹ️</span>
            About Sync Status
          </h3>
          <ul className="text-sm text-gray-700 space-y-2">
            <li className="flex items-start gap-2">
              <span className="font-semibold text-green-700">✅ Synced:</span>
              <span>Git commit SHA matches FluxCD deployed commit SHA</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="font-semibold text-yellow-700">⚠️ Out of Sync:</span>
              <span>Git has newer changes not yet deployed by FluxCD</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="font-semibold text-red-700">❌ Error:</span>
              <span>Failed to check sync status (FluxCD may not be configured)</span>
            </li>
          </ul>
        </div>

        {namespaceStates.length === 0 ? (
          <div className="bg-white rounded-xl shadow-md p-16 text-center">
            <span className="text-6xl mb-4 block">📦</span>
            <h3 className="text-xl font-semibold text-gray-900 mb-2">No namespaces found</h3>
            <p className="text-gray-600">Add namespaces to start monitoring sync status.</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {namespaceStates.map((state, index) => (
              <div
                key={state.namespace.id}
                className="bg-white rounded-xl shadow-md p-6 flex flex-col border border-gray-200 hover:shadow-lg transition-all duration-200 transform hover:-translate-y-1"
              >
                {/* Header */}
                <div className="mb-4">
                  <h3 className="text-xl font-bold text-gray-900 mb-2">
                    {state.namespace.name}
                  </h3>
                  <div className="flex items-center gap-2">
                    <span className="inline-flex items-center px-3 py-1 rounded-full bg-gray-100 text-gray-800 text-xs font-semibold">
                      {state.namespace.cluster}
                    </span>
                    <span className="text-xs text-gray-600">{state.namespace.environment}</span>
                  </div>
                </div>

                {/* Status Badge */}
                <div className="mb-4">
                  {getStatusBadge(state)}
                </div>

                {/* Commit SHAs */}
                {state.syncStatus && !state.error && (
                  <div className="mb-4 space-y-3">
                    <div>
                      <p className="text-xs font-semibold text-gray-600 mb-2">Git Commit</p>
                      <div className="relative group">
                        <code
                          className={`block text-sm font-mono px-3 py-2 rounded-lg ${
                            state.syncStatus.synced
                              ? 'bg-gray-100 text-gray-800'
                              : 'bg-yellow-50 text-yellow-900 border border-yellow-200'
                          }`}
                          title={state.syncStatus.git_commit}
                        >
                          {truncateSha(state.syncStatus.git_commit)}
                        </code>
                      </div>
                    </div>
                    <div>
                      <p className="text-xs font-semibold text-gray-600 mb-2">Flux Commit</p>
                      <div className="relative group">
                        <code
                          className={`block text-sm font-mono px-3 py-2 rounded-lg ${
                            state.syncStatus.synced
                              ? 'bg-gray-100 text-gray-800'
                              : 'bg-red-50 text-red-900 border border-red-200'
                          }`}
                          title={state.syncStatus.flux_commit}
                        >
                          {truncateSha(state.syncStatus.flux_commit)}
                        </code>
                      </div>
                    </div>
                  </div>
                )}

                {/* Error Message */}
                {state.error && (
                  <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
                    {state.error}
                  </div>
                )}

                {state.syncStatus?.error && (
                  <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
                    {state.syncStatus.error}
                  </div>
                )}

                {/* Last Sync Time */}
                {state.syncStatus && !state.error && state.syncStatus.last_sync && (
                  <div className="mb-4 bg-gray-50 rounded-lg p-3">
                    <p className="text-xs text-gray-600">
                      <span className="font-semibold">Last sync:</span> {formatRelativeTime(state.syncStatus.last_sync)}
                    </p>
                  </div>
                )}

                {/* Actions */}
                <div className="mt-auto space-y-2">
                  <button
                    onClick={() => refreshSyncStatus(state.namespace.id, index)}
                    disabled={state.loading}
                    className="w-full px-4 py-2 text-sm font-medium text-blue-600 bg-blue-50 rounded-lg hover:bg-blue-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-200 border border-blue-200"
                  >
                    {state.loading ? (
                      <span className="flex items-center justify-center gap-2">
                        <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600"></div>
                        Refreshing...
                      </span>
                    ) : (
                      '🔄 Refresh Status'
                    )}
                  </button>

                  {state.syncStatus && !state.syncStatus.synced && !state.syncStatus.error && (
                    <a
                      href={`/drift?namespace=${state.namespace.id}`}
                      className="block w-full px-4 py-2 text-sm font-medium text-center text-yellow-700 bg-yellow-50 rounded-lg hover:bg-yellow-100 transition-colors duration-200 border border-yellow-200"
                    >
                      🔍 View Drift →
                    </a>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
