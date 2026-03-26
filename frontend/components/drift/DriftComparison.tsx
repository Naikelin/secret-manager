'use client';

import { useState, useEffect } from 'react';
import { api } from '@/lib/api';

interface ComparisonData {
  git_data: Record<string, string>;
  k8s_data: Record<string, string>;
  keys_added: string[];
  keys_removed: string[];
  keys_modified: string[];
}

interface Props {
  driftEventId: string;
  secretName: string;
  namespace: string;
}

export function DriftComparison({ driftEventId, secretName, namespace }: Props) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [showValues, setShowValues] = useState(false);
  const [comparison, setComparison] = useState<ComparisonData | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (isExpanded && !comparison && !loading) {
      loadComparison();
    }
  }, [isExpanded]);

  const loadComparison = async () => {
    setLoading(true);
    setError(null);
    
    try {
      const data = await api.getDriftComparison(driftEventId);
      
      // Compute diff metadata from git_data and k8s_data
      const gitKeys = Object.keys(data.git_data || {});
      const k8sKeys = Object.keys(data.k8s_data || {});
      const keys_added = k8sKeys.filter(k => !gitKeys.includes(k));
      const keys_removed = gitKeys.filter(k => !k8sKeys.includes(k));
      const keys_modified = gitKeys.filter(k => 
        k8sKeys.includes(k) && data.git_data[k] !== data.k8s_data[k]
      );
      
      setComparison({
        git_data: data.git_data,
        k8s_data: data.k8s_data,
        keys_added,
        keys_removed,
        keys_modified,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error loading comparison');
    } finally {
      setLoading(false);
    }
  };

  const maskValue = (value: string) => {
    return showValues ? value : '••••••••';
  };

  if (!isExpanded) {
    return (
      <div className="mt-4 border-t pt-4">
        <button
          onClick={() => setIsExpanded(true)}
          className="flex items-center gap-2 text-sm text-blue-600 hover:text-blue-800 transition-colors"
        >
          <span>▶</span>
          <span>Expand Comparison</span>
        </button>
      </div>
    );
  }

  const gitKeys = Object.keys(comparison?.git_data || {});
  const k8sKeys = Object.keys(comparison?.k8s_data || {});
  const allKeys = Array.from(new Set([...gitKeys, ...k8sKeys])).sort();

  return (
    <div className="mt-4 border-t pt-4">
      <button
        onClick={() => setIsExpanded(false)}
        className="flex items-center gap-2 text-sm text-blue-600 hover:text-blue-800 transition-colors mb-4"
      >
        <span>▼</span>
        <span>Collapse Comparison</span>
      </button>

      {loading && (
        <div className="flex items-center gap-2 text-gray-600 py-4">
          <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600"></div>
          <span className="text-sm">Loading comparison...</span>
        </div>
      )}

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700 text-sm">
          <strong>Error:</strong> {error}
        </div>
      )}

      {comparison && !loading && (
        <div>
          {/* Header with toggle */}
          <div className="flex justify-between items-center mb-4">
            <h4 className="font-semibold text-gray-900">Visual Comparison</h4>
            <button
              onClick={() => setShowValues(!showValues)}
              className="px-3 py-1.5 text-sm bg-gray-200 hover:bg-gray-300 rounded-md transition-colors font-medium"
            >
              {showValues ? 'Hide Values' : 'Show Values'}
            </button>
          </div>

          {/* Diff Legend */}
          <div className="flex gap-4 mb-4 text-xs text-gray-600">
            <div className="flex items-center gap-1.5">
              <div className="w-3 h-3 bg-green-100 border border-green-400 rounded"></div>
              <span>Added</span>
            </div>
            <div className="flex items-center gap-1.5">
              <div className="w-3 h-3 bg-red-100 border border-red-400 rounded"></div>
              <span>Deleted</span>
            </div>
            <div className="flex items-center gap-1.5">
              <div className="w-3 h-3 bg-yellow-100 border border-yellow-400 rounded"></div>
              <span>Modified</span>
            </div>
          </div>

          {/* Side-by-side comparison */}
          <div className="grid grid-cols-2 gap-4">
            {/* Git (Source of Truth) */}
            <div className="border border-blue-200 rounded-lg p-4 bg-blue-50">
              <h5 className="font-semibold text-blue-900 mb-2">
                Git (Source of Truth)
              </h5>
              <p className="text-xs text-gray-600 mb-3">
                {gitKeys.length} keys
              </p>
              <div className="space-y-2">
                {allKeys.map(key => {
                  const inGit = key in (comparison.git_data || {});
                  const inK8s = key in (comparison.k8s_data || {});
                  
                  if (!inGit) return null; // Skip keys only in K8s for this column
                  
                  const isRemoved = comparison.keys_removed.includes(key);
                  const isModified = comparison.keys_modified.includes(key);
                  
                  return (
                    <div
                      key={key}
                      className={`p-2 rounded text-sm ${
                        isRemoved ? 'bg-red-100 border border-red-400' :
                        isModified ? 'bg-yellow-100 border border-yellow-400' :
                        'bg-white border border-gray-200'
                      }`}
                    >
                      <div className="font-mono font-semibold text-gray-800">{key}</div>
                      <div className="text-gray-600 text-xs mt-1 break-all">
                        {maskValue(comparison.git_data[key])}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>

            {/* Kubernetes (Actual State) */}
            <div className="border border-green-200 rounded-lg p-4 bg-green-50">
              <h5 className="font-semibold text-green-900 mb-2">
                Kubernetes (Actual State)
              </h5>
              <p className="text-xs text-gray-600 mb-3">
                {k8sKeys.length} keys
              </p>
              <div className="space-y-2">
                {allKeys.map(key => {
                  const inGit = key in (comparison.git_data || {});
                  const inK8s = key in (comparison.k8s_data || {});
                  
                  if (!inK8s) return null; // Skip keys only in Git for this column
                  
                  const isAdded = comparison.keys_added.includes(key);
                  const isModified = comparison.keys_modified.includes(key);
                  
                  return (
                    <div
                      key={key}
                      className={`p-2 rounded text-sm ${
                        isAdded ? 'bg-green-100 border border-green-400' :
                        isModified ? 'bg-yellow-100 border border-yellow-400' :
                        'bg-white border border-gray-200'
                      }`}
                    >
                      <div className="font-mono font-semibold text-gray-800">{key}</div>
                      <div className="text-gray-600 text-xs mt-1 break-all">
                        {maskValue(comparison.k8s_data[key])}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
