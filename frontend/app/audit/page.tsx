'use client';

import { useEffect, useState } from 'react';
import { api, type AuditLog, type Namespace, type AuditLogFilters } from '@/lib/api';

export default function AuditLogsPage() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [namespaces, setNamespaces] = useState<Namespace[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [limit] = useState(50);
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState('');
  const [showFilters, setShowFilters] = useState(false);

  // Filter state
  const [filters, setFilters] = useState<AuditLogFilters>({});
  const [filterForm, setFilterForm] = useState({
    action: '',
    resource_type: '',
    resource_name: '',
    namespace_id: '',
    start_date: '',
    end_date: '',
  });

  useEffect(() => {
    loadNamespaces();
  }, []);

  useEffect(() => {
    loadAuditLogs();
  }, [page, filters]);

  async function loadNamespaces() {
    try {
      const data = await api.getNamespaces();
      setNamespaces(data);
    } catch (err) {
      console.error('Failed to load namespaces:', err);
    }
  }

  async function loadAuditLogs() {
    try {
      setLoading(true);
      setError('');
      const response = await api.getAuditLogs({ ...filters, page, limit });
      setLogs(response.logs || []);
      setTotal(response.total);
    } catch (err: any) {
      setError(err.message || 'Failed to load audit logs');
    } finally {
      setLoading(false);
    }
  }

  function applyFilters() {
    const newFilters: AuditLogFilters = {};
    
    if (filterForm.action) newFilters.action = filterForm.action;
    if (filterForm.resource_type) newFilters.resource_type = filterForm.resource_type;
    if (filterForm.resource_name) newFilters.resource_name = filterForm.resource_name;
    if (filterForm.namespace_id) newFilters.namespace_id = filterForm.namespace_id;
    
    // Convert date inputs to RFC3339 format
    if (filterForm.start_date) {
      const startDate = new Date(filterForm.start_date);
      startDate.setHours(0, 0, 0, 0);
      newFilters.start_date = startDate.toISOString();
    }
    
    if (filterForm.end_date) {
      const endDate = new Date(filterForm.end_date);
      endDate.setHours(23, 59, 59, 999);
      newFilters.end_date = endDate.toISOString();
    }

    setFilters(newFilters);
    setPage(1);
  }

  function clearFilters() {
    setFilterForm({
      action: '',
      resource_type: '',
      resource_name: '',
      namespace_id: '',
      start_date: '',
      end_date: '',
    });
    setFilters({});
    setPage(1);
  }

  async function handleExport() {
    try {
      setExporting(true);
      setError('');
      await api.exportAuditLogsCSV(filters);
    } catch (err: any) {
      setError(err.message || 'Failed to export audit logs');
    } finally {
      setExporting(false);
    }
  }

  function formatTimestamp(timestamp: string): string {
    const date = new Date(timestamp);
    return date.toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
      hour12: true,
    });
  }

  function getActionBadgeColor(action: string): string {
    if (action.includes('created')) return 'bg-green-100 text-green-800';
    if (action.includes('updated')) return 'bg-blue-100 text-blue-800';
    if (action.includes('deleted')) return 'bg-red-100 text-red-800';
    if (action.includes('published')) return 'bg-purple-100 text-purple-800';
    if (action.includes('unpublished')) return 'bg-gray-100 text-gray-800';
    if (action.includes('synced') || action.includes('imported') || action.includes('marked')) {
      return 'bg-yellow-100 text-yellow-800';
    }
    return 'bg-gray-100 text-gray-800';
  }

  const totalPages = Math.ceil(total / limit);

  if (loading && logs.length === 0) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-gray-50 to-green-50 p-8">
        <div className="max-w-7xl mx-auto">
          <div className="flex items-center justify-center h-64">
            <div className="text-center">
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-green-600 mx-auto"></div>
              <p className="mt-4 text-gray-600">Loading audit logs...</p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-green-50 p-8">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="flex justify-between items-center mb-8">
          <div>
            <h1 className="text-4xl font-bold text-gray-900">Audit Logs</h1>
            <p className="text-gray-600 mt-2">Review all secret management activities</p>
          </div>
          <div className="flex gap-3">
            <button
              onClick={() => setShowFilters(!showFilters)}
              className="px-6 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 font-medium transition-all duration-200 shadow-md hover:shadow-lg flex items-center gap-2"
            >
              {showFilters ? '✕ Hide Filters' : '🔍 Show Filters'}
            </button>
            <button
              onClick={handleExport}
              disabled={exporting || total === 0}
              className="px-6 py-2 bg-gradient-to-r from-green-600 to-emerald-600 text-white rounded-lg hover:from-green-700 hover:to-emerald-700 disabled:opacity-50 font-medium transition-all duration-200 shadow-md hover:shadow-lg flex items-center gap-2"
            >
              {exporting ? (
                <>
                  <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div>
                  Exporting...
                </>
              ) : (
                <>📥 Export CSV</>
              )}
            </button>
          </div>
        </div>

        {error && (
          <div className="mb-6 bg-red-50 border-l-4 border-red-500 rounded-lg p-4 shadow-sm">
            <div className="flex items-center gap-2">
              <span className="text-red-600 font-medium">❌ Error:</span>
              <p className="text-red-700">{error}</p>
            </div>
          </div>
        )}

        {/* Filters Panel */}
        {showFilters && (
          <div className="bg-white rounded-xl shadow-md p-6 mb-6 border border-gray-200">
            <h3 className="font-semibold text-gray-900 mb-4 flex items-center gap-2">
              <span className="text-xl">🔍</span>
              Filters
            </h3>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-4">
              {/* Action Type */}
              <div>
                <label className="block text-sm font-semibold text-gray-700 mb-1">Action Type</label>
                <select
                  value={filterForm.action}
                  onChange={(e) => setFilterForm({ ...filterForm, action: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent transition-all duration-200"
                >
                  <option value="">All Actions</option>
                  <option value="created">Created</option>
                  <option value="updated">Updated</option>
                  <option value="deleted">Deleted</option>
                  <option value="published">Published</option>
                  <option value="unpublished">Unpublished</option>
                  <option value="synced_from_git">Synced from Git</option>
                  <option value="imported_to_git">Imported to Git</option>
                  <option value="marked_resolved">Marked Resolved</option>
                </select>
              </div>

              {/* Resource Type */}
              <div>
                <label className="block text-sm font-semibold text-gray-700 mb-1">Resource Type</label>
                <select
                  value={filterForm.resource_type}
                  onChange={(e) => setFilterForm({ ...filterForm, resource_type: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent transition-all duration-200"
                >
                  <option value="">All Types</option>
                  <option value="secret">Secret</option>
                  <option value="drift_event">Drift Event</option>
                  <option value="namespace">Namespace</option>
                  <option value="user">User</option>
                  <option value="group">Group</option>
                </select>
              </div>

              {/* Namespace */}
              <div>
                <label className="block text-sm font-semibold text-gray-700 mb-1">Namespace</label>
                <select
                  value={filterForm.namespace_id}
                  onChange={(e) => setFilterForm({ ...filterForm, namespace_id: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent transition-all duration-200"
                >
                  <option value="">All Namespaces</option>
                  {namespaces.map((ns) => (
                    <option key={ns.id} value={ns.id}>
                      {ns.name} ({ns.cluster})
                    </option>
                  ))}
                </select>
              </div>

              {/* Resource Name */}
              <div>
                <label className="block text-sm font-semibold text-gray-700 mb-1">Resource Name</label>
                <input
                  type="text"
                  value={filterForm.resource_name}
                  onChange={(e) => setFilterForm({ ...filterForm, resource_name: e.target.value })}
                  placeholder="e.g., my-secret"
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent transition-all duration-200"
                />
              </div>

              {/* Start Date */}
              <div>
                <label className="block text-sm font-semibold text-gray-700 mb-1">Start Date</label>
                <input
                  type="date"
                  value={filterForm.start_date}
                  onChange={(e) => setFilterForm({ ...filterForm, start_date: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent transition-all duration-200"
                />
              </div>

              {/* End Date */}
              <div>
                <label className="block text-sm font-semibold text-gray-700 mb-1">End Date</label>
                <input
                  type="date"
                  value={filterForm.end_date}
                  onChange={(e) => setFilterForm({ ...filterForm, end_date: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent transition-all duration-200"
                />
              </div>
            </div>

            <div className="flex gap-3">
              <button
                onClick={applyFilters}
                className="px-6 py-2 bg-gradient-to-r from-blue-600 to-purple-600 text-white rounded-lg hover:from-blue-700 hover:to-purple-700 font-medium transition-all duration-200 shadow-md hover:shadow-lg"
              >
                Apply Filters
              </button>
              <button
                onClick={clearFilters}
                className="px-6 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300 font-medium transition-colors duration-200"
              >
                Clear Filters
              </button>
            </div>
          </div>
        )}

        {/* Stats */}
        <div className="mb-4 bg-white rounded-lg shadow-sm px-4 py-3 border border-gray-200">
          <p className="text-sm font-medium text-gray-700">
            Showing <span className="font-bold text-gray-900">{logs.length}</span> of <span className="font-bold text-gray-900">{total}</span> total logs
            {Object.keys(filters).length > 0 && <span className="ml-2 px-2 py-1 bg-blue-100 text-blue-800 rounded text-xs font-semibold">Filtered</span>}
          </p>
        </div>

        {/* Audit Logs Table */}
        <div className="bg-white rounded-xl shadow-md overflow-hidden border border-gray-200">
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-gradient-to-r from-gray-50 to-green-50 border-b border-gray-200">
                <tr>
                  <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">
                    Timestamp
                  </th>
                  <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">
                    User
                  </th>
                  <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">
                    Action
                  </th>
                  <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">
                    Resource Type
                  </th>
                  <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">
                    Resource Name
                  </th>
                  <th className="px-6 py-4 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">
                    Namespace
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {logs.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="px-6 py-16 text-center">
                      <div className="flex flex-col items-center justify-center">
                        <span className="text-6xl mb-4">📋</span>
                        <h3 className="text-lg font-semibold text-gray-900 mb-2">
                          {Object.keys(filters).length > 0
                            ? 'No audit logs found matching your filters'
                            : 'No audit logs found'}
                        </h3>
                        {Object.keys(filters).length > 0 && (
                          <button
                            onClick={clearFilters}
                            className="mt-3 px-4 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 text-sm font-medium transition-colors duration-200"
                          >
                            Clear Filters
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ) : (
                  logs.map((log, idx) => (
                    <tr key={log.id} className={`${idx % 2 === 0 ? 'bg-white' : 'bg-gray-50'} hover:bg-blue-50 transition-colors duration-150`}>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 font-medium">
                        {formatTimestamp(log.timestamp)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm">
                        <div className="text-gray-900 font-medium">{log.user?.email || 'System'}</div>
                        {log.user?.name && (
                          <div className="text-gray-500 text-xs">{log.user.name}</div>
                        )}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className={`px-3 py-1 text-xs font-semibold rounded-full ${getActionBadgeColor(log.action_type)}`}>
                          {log.action_type}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-600">
                        {log.resource_type}
                      </td>
                      <td className="px-6 py-4 text-sm text-gray-900 font-semibold">
                        {log.resource_name}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-600">
                        {log.namespace?.name || '-'}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="mt-6 flex items-center justify-between bg-white rounded-lg shadow-sm px-6 py-4 border border-gray-200">
            <button
              onClick={() => setPage(page - 1)}
              disabled={page === 1 || loading}
              className="px-6 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed font-medium transition-all duration-200"
            >
              ← Previous
            </button>

            <div className="text-sm font-medium text-gray-700">
              Page <span className="font-bold text-gray-900">{page}</span> of <span className="font-bold text-gray-900">{totalPages}</span>
            </div>

            <button
              onClick={() => setPage(page + 1)}
              disabled={page === totalPages || loading}
              className="px-6 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed font-medium transition-all duration-200"
            >
              Next →
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
