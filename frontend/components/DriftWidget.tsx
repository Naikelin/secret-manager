'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { api, type Namespace, type DriftEvent } from '@/lib/api';

interface DriftSummary {
  namespace_id: string;
  namespace_name: string;
  unresolved_count: number;
}

export default function DriftWidget() {
  const router = useRouter();
  const [summary, setSummary] = useState<DriftSummary[]>([]);
  const [totalDrift, setTotalDrift] = useState(0);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadDriftSummary();
    
    // Auto-refresh every 30 seconds
    const interval = setInterval(loadDriftSummary, 30000);
    return () => clearInterval(interval);
  }, []);

  async function loadDriftSummary() {
    try {
      const namespaces = await api.getNamespaces();
      const summaryData: DriftSummary[] = [];
      let total = 0;

      for (const ns of namespaces) {
        const response = await api.getDriftEvents(ns.id);
        const unresolved = response.events.filter(e => !e.resolved_at).length;
        
        if (unresolved > 0) {
          summaryData.push({
            namespace_id: ns.id,
            namespace_name: ns.name,
            unresolved_count: unresolved,
          });
          total += unresolved;
        }
      }

      setSummary(summaryData);
      setTotalDrift(total);
    } catch (err) {
      console.error('Failed to load drift summary:', err);
    } finally {
      setLoading(false);
    }
  }

  const severityColor = () => {
    if (totalDrift === 0) return 'green';
    if (totalDrift <= 5) return 'yellow';
    return 'red';
  };

  const colorClasses = {
    green: 'border-green-500 bg-green-50',
    yellow: 'border-yellow-500 bg-yellow-50',
    red: 'border-red-500 bg-red-50',
  };

  if (loading) {
    return (
      <div className="bg-white rounded-xl shadow-md p-6 border-l-4 border-gray-300">
        <div className="animate-pulse">
          <div className="h-6 bg-gray-200 rounded w-1/3 mb-4"></div>
          <div className="h-4 bg-gray-200 rounded w-2/3"></div>
        </div>
      </div>
    );
  }

  return (
    <div className={`rounded-xl shadow-md p-6 border-l-4 transition-all duration-200 hover:shadow-lg ${colorClasses[severityColor()]}`}>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <span className="text-4xl">⚠️</span>
          <div>
            <h3 className="text-xl font-bold text-gray-900">Drift Detection</h3>
            <p className="text-sm text-gray-600">Configuration drift monitoring</p>
          </div>
        </div>
        <div className="text-right">
          <div className={`text-4xl font-bold ${
            totalDrift === 0 ? 'text-green-600' :
            totalDrift <= 5 ? 'text-yellow-600' :
            'text-red-600'
          }`}>
            {totalDrift}
          </div>
          <p className="text-sm text-gray-600">Unresolved</p>
        </div>
      </div>

      {summary.length > 0 && (
        <div className="space-y-2 mb-4">
          {summary.map(item => (
            <div key={item.namespace_id} className="flex justify-between items-center text-sm">
              <span className="text-gray-700 font-medium">{item.namespace_name}</span>
              <span className="px-3 py-1 bg-white rounded-full text-gray-900 font-semibold">
                {item.unresolved_count}
              </span>
            </div>
          ))}
        </div>
      )}

      <button
        onClick={() => router.push('/drift')}
        className="w-full px-4 py-2 bg-gradient-to-r from-yellow-500 to-amber-600 text-white rounded-lg hover:from-yellow-600 hover:to-amber-700 font-medium transition-all duration-200 shadow-sm hover:shadow-md"
      >
        {totalDrift > 0 ? 'View & Resolve Drift' : 'View Drift Monitor'}
      </button>
    </div>
  );
}
