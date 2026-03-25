'use client';

import { useEffect, useState } from 'react';
import { api, type Namespace, type DriftEvent } from '@/lib/api';

export default function DriftPage() {
  const [namespaces, setNamespaces] = useState<Namespace[]>([]);
  const [selectedNs, setSelectedNs] = useState<string>('');
  const [driftEvents, setDriftEvents] = useState<DriftEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    loadNamespaces();
  }, []);

  useEffect(() => {
    if (selectedNs) {
      loadDriftEvents(selectedNs);
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

  async function loadDriftEvents(nsId: string) {
    try {
      setLoading(true);
      const data = await api.getDriftEvents(nsId);
      setDriftEvents(data.events || []);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to load drift events';
      console.error('Failed to load drift events:', err);
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  }

  async function handleTriggerCheck() {
    if (!selectedNs) return;
    try {
      setChecking(true);
      setError('');
      await api.triggerDriftCheck(selectedNs);
      await loadDriftEvents(selectedNs);
    } catch (err) {
      setError('Failed to trigger drift check');
    } finally {
      setChecking(false);
    }
  }

  async function handleResolution(event: DriftEvent, action: 'sync' | 'import' | 'mark') {
    const actions = {
      sync: { fn: () => api.syncFromGit(event.id), label: 'Sync from Git' },
      import: { fn: () => api.importToGit(event.id), label: 'Import to Git' },
      mark: { fn: () => api.markDriftResolved(event.id), label: 'Mark as resolved' }
    };

    const { fn, label } = actions[action];
    
    if (!confirm(`${label} for ${event.secret_name}?`)) return;

    try {
      await fn();
      await loadDriftEvents(selectedNs);
    } catch (err) {
      alert(`Failed to ${label.toLowerCase()}`);
    }
  }

  function getDriftBadgeColor(type: string) {
    switch (type) {
      case 'modified': return 'bg-yellow-100 text-yellow-800';
      case 'deleted': return 'bg-red-100 text-red-800';
      case 'added': return 'bg-blue-100 text-blue-800';
      default: return 'bg-gray-100 text-gray-800';
    }
  }

  function getDriftIcon(type: string) {
    switch (type) {
      case 'modified': return '⚠️';
      case 'deleted': return '🗑️';
      case 'added': return '➕';
      default: return '❓';
    }
  }

  if (loading) return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-yellow-50 p-8">
      <div className="max-w-6xl mx-auto">
        <div className="flex items-center justify-center h-64">
          <div className="text-center">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-yellow-600 mx-auto"></div>
            <p className="mt-4 text-gray-600">Loading drift events...</p>
          </div>
        </div>
      </div>
    </div>
  );

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-yellow-50 p-8">
      <div className="max-w-6xl mx-auto">
        <div className="flex justify-between items-center mb-8">
          <div>
            <h1 className="text-4xl font-bold text-gray-900">Drift Detection</h1>
            <p className="mt-2 text-gray-600">Monitor and resolve configuration drift</p>
          </div>
          <button
            onClick={handleTriggerCheck}
            disabled={checking || !selectedNs}
            className="px-6 py-2 bg-gradient-to-r from-blue-600 to-purple-600 text-white rounded-lg hover:from-blue-700 hover:to-purple-700 disabled:opacity-50 font-medium transition-all duration-200 shadow-md hover:shadow-lg flex items-center gap-2"
          >
            {checking ? (
              <>
                <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div>
                Checking...
              </>
            ) : (
              <>🔍 Check for Drift</>
            )}
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

        {/* Namespace Selector */}
        <div className="mb-6 bg-white rounded-xl shadow-md p-6">
          <label className="block text-sm font-semibold text-gray-700 mb-2">Namespace</label>
          <select
            value={selectedNs}
            onChange={(e) => setSelectedNs(e.target.value)}
            className="w-full max-w-md px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-yellow-500 focus:border-transparent transition-all duration-200"
          >
            {namespaces.map((ns) => (
              <option key={ns.id} value={ns.id}>
                {ns.name} ({ns.cluster})
              </option>
            ))}
          </select>
        </div>

        {/* Info Box */}
        <div className="mb-6 bg-blue-50 border-l-4 border-blue-500 rounded-lg p-6 shadow-sm">
          <h3 className="font-semibold text-gray-900 mb-3 flex items-center gap-2">
            <span className="text-xl">ℹ️</span>
            About Drift Detection
          </h3>
          <ul className="text-sm text-gray-700 space-y-2">
            <li className="flex items-start gap-2">
              <span className="font-semibold text-yellow-700">⚠️ Modified:</span>
              <span>Secret values differ between Git and Kubernetes</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="font-semibold text-red-700">🗑️ Deleted:</span>
              <span>Secret exists in Git but not in Kubernetes</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="font-semibold text-blue-700">➕ Added:</span>
              <span>Secret exists in Kubernetes but not in Git (orphaned)</span>
            </li>
          </ul>
        </div>

        {/* Drift Events List */}
        <div className="bg-white rounded-xl shadow-md overflow-hidden">
          {driftEvents.length === 0 ? (
            <div className="p-16 text-center">
              <div className="text-6xl mb-4">✅</div>
              <h3 className="text-2xl font-bold text-gray-900 mb-2">No Drift Detected</h3>
              <p className="text-gray-600">All secrets are in sync between Git and Kubernetes.</p>
            </div>
          ) : (
            <div className="divide-y divide-gray-200">
              {driftEvents.map((event) => (
                <div key={event.id} className="p-6 hover:bg-gray-50 transition-colors duration-150">
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-3">
                        <span className="text-3xl">{getDriftIcon(event.drift_type)}</span>
                        <h3 className="text-xl font-semibold text-gray-900">{event.secret_name}</h3>
                        <span className={`px-3 py-1 text-xs font-bold rounded-full shadow-sm ${getDriftBadgeColor(event.drift_type)}`}>
                          {event.drift_type}
                        </span>
                        {event.resolved_at && (
                          <span className="px-3 py-1 text-xs font-bold rounded-full bg-green-100 text-green-800 shadow-sm">
                            ✓ Resolved
                          </span>
                        )}
                      </div>

                      <p className="text-sm text-gray-600 mb-3">
                        Detected: <span className="font-medium">{new Date(event.detected_at).toLocaleString()}</span>
                      </p>

                      {event.details?.message && (
                        <div className="p-4 bg-gray-50 border border-gray-200 rounded-lg text-sm font-mono mb-3">
                          {event.details.message}
                        </div>
                      )}

                      {event.details?.keys_changed && (
                        <div className="text-sm text-gray-600 mb-3">
                          Keys affected: <span className="font-semibold text-gray-900">{event.details.keys_changed.join(', ')}</span>
                        </div>
                      )}

                      {event.resolved_at && (
                        <div className="mt-3 text-sm text-gray-500 bg-green-50 border border-green-200 rounded-lg p-3">
                          <span className="font-semibold text-green-700">Resolved:</span> {new Date(event.resolved_at).toLocaleString()}
                          {event.resolution_action && <span className="ml-2">({event.resolution_action})</span>}
                        </div>
                      )}
                    </div>

                    {/* Resolution Actions (only for unresolved events) */}
                    {!event.resolved_at && (
                      <div className="flex flex-col gap-2 ml-4 min-w-[180px]">
                        <button
                          onClick={() => handleResolution(event, 'sync')}
                          className="px-4 py-2 text-sm font-medium bg-green-600 text-white rounded-lg hover:bg-green-700 whitespace-nowrap transition-all duration-200 shadow-md hover:shadow-lg"
                          title="Overwrite Kubernetes with Git version"
                        >
                          ⬇️ Sync from Git
                        </button>
                        <button
                          onClick={() => handleResolution(event, 'import')}
                          className="px-4 py-2 text-sm font-medium bg-blue-600 text-white rounded-lg hover:bg-blue-700 whitespace-nowrap transition-all duration-200 shadow-md hover:shadow-lg"
                          title="Import Kubernetes changes to Git"
                        >
                          ⬆️ Import to Git
                        </button>
                        <button
                          onClick={() => handleResolution(event, 'mark')}
                          className="px-4 py-2 text-sm font-medium bg-gray-600 text-white rounded-lg hover:bg-gray-700 whitespace-nowrap transition-all duration-200 shadow-md hover:shadow-lg"
                          title="Mark as resolved without action"
                        >
                          ✓ Mark Resolved
                        </button>
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
