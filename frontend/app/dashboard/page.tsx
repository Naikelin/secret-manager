'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { getCurrentUser, logout, isAuthenticated } from '@/lib/api';
import DriftWidget from '@/components/DriftWidget';

interface User {
  id: string;
  email: string;
  name: string;
  groups: string[];
}

export default function DashboardPage() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Check authentication
    if (!isAuthenticated()) {
      router.push('/auth/login');
      return;
    }

    // Load user data
    const userData = getCurrentUser();
    if (userData) {
      setUser(userData);
    }
    setLoading(false);
  }, [router]);

  const handleLogout = () => {
    logout();
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mx-auto"></div>
          <p className="mt-4 text-muted-foreground">Loading...</p>
        </div>
      </div>
    );
  }

  if (!user) {
    return null; // Redirecting to login
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-blue-50">
      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="mb-8">
          <h2 className="text-4xl font-bold text-gray-900">Dashboard</h2>
          <p className="mt-2 text-lg text-gray-600">
            Welcome to the GitOps-based Kubernetes Secret Management Platform
          </p>
        </div>

        {/* User Info Card */}
        <div className="bg-white rounded-xl shadow-md p-6 mb-8 border border-gray-100 hover:shadow-lg transition-shadow duration-200">
          <h3 className="text-lg font-semibold text-gray-900 mb-4 flex items-center gap-2">
            <span className="text-2xl">👤</span>
            User Information
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <span className="text-sm font-medium text-gray-500">Name</span>
              <p className="text-gray-900 mt-1 font-medium">{user.name}</p>
            </div>
            <div>
              <span className="text-sm font-medium text-gray-500">Email</span>
              <p className="text-gray-900 mt-1 font-medium">{user.email}</p>
            </div>
            <div>
              <span className="text-sm font-medium text-gray-500">User ID</span>
              <p className="text-gray-900 mt-1 font-mono text-sm bg-gray-50 px-3 py-1 rounded inline-block">{user.id}</p>
            </div>
            <div>
              <span className="text-sm font-medium text-gray-500">Groups</span>
              <div className="mt-2 flex flex-wrap gap-2">
                {user.groups && user.groups.length > 0 ? (
                  user.groups.map((group) => (
                    <span
                      key={group}
                      className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-800"
                    >
                      {group}
                    </span>
                  ))
                ) : (
                  <span className="text-gray-500 text-sm">No groups assigned</span>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Quick Actions */}
        <div className="mb-8">
          <h3 className="text-2xl font-bold text-gray-900 mb-4">Quick Actions</h3>
        </div>
        
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mb-8">
          <a 
            href="/secrets" 
            className="group bg-white rounded-xl shadow-md p-6 border-l-4 border-blue-500 hover:shadow-xl transition-all duration-200 transform hover:-translate-y-1"
          >
            <div className="flex items-center gap-3 mb-3">
              <span className="text-3xl">🔐</span>
              <h4 className="text-lg font-semibold text-gray-900">Secrets</h4>
            </div>
            <p className="text-sm text-gray-600 mb-4">
              Manage Kubernetes secrets with GitOps workflow
            </p>
            <span className="text-sm text-blue-600 font-medium group-hover:underline">
              View Secrets →
            </span>
          </a>

          <a 
            href="/sync-status" 
            className="group bg-white rounded-xl shadow-md p-6 border-l-4 border-purple-500 hover:shadow-xl transition-all duration-200 transform hover:-translate-y-1"
          >
            <div className="flex items-center gap-3 mb-3">
              <span className="text-3xl">🔄</span>
              <h4 className="text-lg font-semibold text-gray-900">Sync Status</h4>
            </div>
            <p className="text-sm text-gray-600 mb-4">
              Monitor FluxCD synchronization status
            </p>
            <span className="text-sm text-purple-600 font-medium group-hover:underline">
              View Sync Status →
            </span>
          </a>

          <a 
            href="/audit" 
            className="group bg-white rounded-xl shadow-md p-6 border-l-4 border-green-500 hover:shadow-xl transition-all duration-200 transform hover:-translate-y-1"
          >
            <div className="flex items-center gap-3 mb-3">
              <span className="text-3xl">📋</span>
              <h4 className="text-lg font-semibold text-gray-900">Audit Logs</h4>
            </div>
            <p className="text-sm text-gray-600 mb-4">
              Review all secret management activities
            </p>
            <span className="text-sm text-green-600 font-medium group-hover:underline">
              View Audit Logs →
            </span>
          </a>
        </div>

        {/* Drift Detection Widget */}
        <div className="mb-8">
          <h3 className="text-2xl font-bold text-gray-900 mb-4">Monitoring</h3>
        </div>
        
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
          <DriftWidget />
        </div>

        {/* Status Info */}
        <div className="mt-8 bg-white rounded-xl shadow-md p-6 border border-gray-100">
          <h3 className="text-lg font-semibold text-gray-900 mb-4">System Status</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="flex items-center space-x-3">
              <div className="w-3 h-3 rounded-full bg-green-500 shadow-lg shadow-green-500/50"></div>
              <div>
                <p className="text-sm font-medium text-gray-900">Authentication</p>
                <p className="text-xs text-gray-500">Active (Mock Provider)</p>
              </div>
            </div>
            <div className="flex items-center space-x-3">
              <div className="w-3 h-3 rounded-full bg-gray-400"></div>
              <div>
                <p className="text-sm font-medium text-gray-900">FluxCD</p>
                <p className="text-xs text-gray-500">Not configured</p>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
