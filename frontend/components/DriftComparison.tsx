'use client';

import { useEffect, useState } from 'react';
import ReactDiffViewer, { DiffMethod } from 'react-diff-viewer-continued';
import { api } from '@/lib/api';

interface DriftComparisonProps {
  driftEventId: string;
}

export function DriftComparison({ driftEventId }: DriftComparisonProps) {
  const [gitData, setGitData] = useState<Record<string, string>>({});
  const [k8sData, setK8sData] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showValues, setShowValues] = useState(false);

  useEffect(() => {
    loadComparison();
  }, [driftEventId]);

  async function loadComparison() {
    try {
      setLoading(true);
      const data = await api.getDriftComparison(driftEventId);
      setGitData(data.git_data || {});
      setK8sData(data.k8s_data || {});
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to load comparison';
      console.error('[DriftComparison] Failed to load drift comparison:', err);
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  }

  function formatData(data: Record<string, string>, maskValues: boolean): string {
    if (Object.keys(data).length === 0) {
      return '{}';
    }

    const formatted: Record<string, string> = {};
    for (const [key, value] of Object.entries(data)) {
      formatted[key] = maskValues ? '••••••••' : value;
    }

    return JSON.stringify(formatted, null, 2);
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center p-8">
        <div className="text-center">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
          <p className="mt-2 text-gray-600 text-sm">Loading comparison...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-4 bg-red-50 border border-red-200 rounded-lg">
        <p className="text-red-700 text-sm">❌ {error}</p>
      </div>
    );
  }

  const gitFormatted = formatData(gitData, !showValues);
  const k8sFormatted = formatData(k8sData, !showValues);

  return (
    <div className="space-y-4">
      {/* Controls */}
      <div className="flex items-center justify-between bg-gray-50 p-4 rounded-lg">
        <div className="flex items-center gap-6">
          <div className="flex items-center gap-2">
            <span className="text-2xl">⬅️</span>
            <div>
              <div className="font-semibold text-gray-900">Git (Source of Truth)</div>
              <div className="text-xs text-gray-600">{Object.keys(gitData).length} keys</div>
            </div>
          </div>
          <div className="text-2xl text-gray-400">vs</div>
          <div className="flex items-center gap-2">
            <span className="text-2xl">➡️</span>
            <div>
              <div className="font-semibold text-gray-900">Kubernetes (Actual State)</div>
              <div className="text-xs text-gray-600">{Object.keys(k8sData).length} keys</div>
            </div>
          </div>
        </div>
        <button
          onClick={() => setShowValues(!showValues)}
          className="px-4 py-2 text-sm font-medium bg-gray-700 text-white rounded-lg hover:bg-gray-800 transition-colors duration-200 flex items-center gap-2"
        >
          {showValues ? '🙈 Hide Values' : '👁️ Show Values'}
        </button>
      </div>

      {/* Diff Viewer */}
      <div className="border border-gray-200 rounded-lg overflow-hidden shadow-sm">
        <ReactDiffViewer
          oldValue={gitFormatted}
          newValue={k8sFormatted}
          splitView={true}
          compareMethod={DiffMethod.WORDS}
          leftTitle="Git (Source of Truth)"
          rightTitle="Kubernetes (Actual State)"
          styles={{
            variables: {
              light: {
                diffViewerBackground: '#ffffff',
                addedBackground: '#e6ffec',
                addedColor: '#24292e',
                removedBackground: '#ffebe9',
                removedColor: '#24292e',
                wordAddedBackground: '#acf2bd',
                wordRemovedBackground: '#fdb8c0',
                addedGutterBackground: '#cdffd8',
                removedGutterBackground: '#ffdce0',
                gutterBackground: '#f6f8fa',
                gutterBackgroundDark: '#f3f4f6',
                highlightBackground: '#fffbdd',
                highlightGutterBackground: '#fff5b1',
              },
            },
            line: {
              padding: '8px',
              fontSize: '14px',
              fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace',
            },
            gutter: {
              padding: '0 8px',
              minWidth: '40px',
            },
          }}
          useDarkTheme={false}
        />
      </div>

      {/* Legend */}
      <div className="flex items-center gap-6 text-sm p-4 bg-blue-50 border border-blue-200 rounded-lg">
        <div className="font-semibold text-gray-900">Legend:</div>
        <div className="flex items-center gap-2">
          <div className="w-4 h-4 bg-green-200 border border-green-400 rounded"></div>
          <span className="text-gray-700">Added in K8s</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-4 h-4 bg-red-200 border border-red-400 rounded"></div>
          <span className="text-gray-700">Missing in K8s</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-4 h-4 bg-yellow-200 border border-yellow-400 rounded"></div>
          <span className="text-gray-700">Modified</span>
        </div>
      </div>
    </div>
  );
}
