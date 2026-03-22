'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { getCurrentUser, logout, isAuthenticated } from '@/lib/api';

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
    <div className="min-h-screen bg-background">
      {/* Navigation Bar */}
      <nav className="bg-card border-b">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16">
            <div className="flex items-center">
              <h1 className="text-xl font-bold text-foreground">Secret Manager</h1>
            </div>
            <div className="flex items-center space-x-4">
              <span className="text-sm text-muted-foreground">{user.email}</span>
              <button
                onClick={handleLogout}
                className="px-4 py-2 text-sm font-medium text-primary-foreground bg-primary rounded-md hover:opacity-90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-ring transition-opacity"
              >
                Logout
              </button>
            </div>
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="mb-8">
          <h2 className="text-3xl font-bold text-foreground">Dashboard</h2>
          <p className="mt-2 text-muted-foreground">
            Welcome to the GitOps-based Kubernetes Secret Management Platform
          </p>
        </div>

        {/* User Info Card */}
        <div className="bg-card rounded-lg shadow-md p-6 mb-8">
          <h3 className="text-lg font-semibold text-foreground mb-4">User Information</h3>
          <div className="space-y-3">
            <div>
              <span className="text-sm font-medium text-muted-foreground">Name:</span>
              <p className="text-foreground mt-1">{user.name}</p>
            </div>
            <div>
              <span className="text-sm font-medium text-muted-foreground">Email:</span>
              <p className="text-foreground mt-1">{user.email}</p>
            </div>
            <div>
              <span className="text-sm font-medium text-muted-foreground">User ID:</span>
              <p className="text-foreground mt-1 font-mono text-sm">{user.id}</p>
            </div>
            <div>
              <span className="text-sm font-medium text-muted-foreground">Groups:</span>
              <div className="mt-2 flex flex-wrap gap-2">
                {user.groups && user.groups.length > 0 ? (
                  user.groups.map((group) => (
                    <span
                      key={group}
                      className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-primary/10 text-primary"
                    >
                      {group}
                    </span>
                  ))
                ) : (
                  <span className="text-muted-foreground text-sm">No groups assigned</span>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Quick Actions */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <div className="bg-card rounded-lg shadow-md p-6 border-l-4 border-blue-500">
            <h4 className="text-lg font-semibold text-foreground mb-2">Secrets</h4>
            <p className="text-sm text-muted-foreground mb-4">
              Manage Kubernetes secrets with GitOps workflow
            </p>
            <button className="text-sm text-primary hover:underline font-medium">
              View Secrets →
            </button>
          </div>

          <div className="bg-card rounded-lg shadow-md p-6 border-l-4 border-yellow-500">
            <h4 className="text-lg font-semibold text-foreground mb-2">Drift Detection</h4>
            <p className="text-sm text-muted-foreground mb-4">
              Monitor and resolve configuration drift
            </p>
            <button className="text-sm text-primary hover:underline font-medium">
              View Drift Events →
            </button>
          </div>

          <div className="bg-card rounded-lg shadow-md p-6 border-l-4 border-green-500">
            <h4 className="text-lg font-semibold text-foreground mb-2">Audit Logs</h4>
            <p className="text-sm text-muted-foreground mb-4">
              Review all secret management activities
            </p>
            <button className="text-sm text-primary hover:underline font-medium">
              View Audit Logs →
            </button>
          </div>
        </div>

        {/* Status Info */}
        <div className="mt-8 bg-card rounded-lg shadow-md p-6">
          <h3 className="text-lg font-semibold text-foreground mb-4">System Status</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="flex items-center space-x-3">
              <div className="w-3 h-3 rounded-full bg-green-500"></div>
              <div>
                <p className="text-sm font-medium text-foreground">Authentication</p>
                <p className="text-xs text-muted-foreground">Active (Mock Provider)</p>
              </div>
            </div>
            <div className="flex items-center space-x-3">
              <div className="w-3 h-3 rounded-full bg-gray-400"></div>
              <div>
                <p className="text-sm font-medium text-foreground">FluxCD</p>
                <p className="text-xs text-muted-foreground">Not configured</p>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
