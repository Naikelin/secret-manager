'use client';

import { useEffect, useState } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import { api, type DriftEvent } from '@/lib/api';

export default function Navbar() {
  const router = useRouter();
  const pathname = usePathname();
  const [driftCount, setDriftCount] = useState(0);
  const [recentDrift, setRecentDrift] = useState<DriftEvent[]>([]);
  const [showDropdown, setShowDropdown] = useState(false);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [userEmail, setUserEmail] = useState<string | null>(null);

  useEffect(() => {
    // Check if user is authenticated
    const token = typeof window !== 'undefined' ? localStorage.getItem('auth_token') : null;
    let email = null;
    if (typeof window !== 'undefined') {
      const userStr = localStorage.getItem('user');
      if (userStr) {
        try {
          const user = JSON.parse(userStr);
          email = user.email;
        } catch (e) {
          console.error('Failed to parse user:', e);
        }
      }
    }
    setIsAuthenticated(!!token);
    setUserEmail(email);

    if (token) {
      loadDriftCount();
      const interval = setInterval(loadDriftCount, 60000); // Every 60s
      return () => clearInterval(interval);
    }
  }, []);

  async function loadDriftCount() {
    try {
      const namespaces = await api.getNamespaces();
      let total = 0;
      let allEvents: DriftEvent[] = [];

      for (const ns of namespaces) {
        const response = await api.getDriftEvents(ns.id);
        const unresolved = response.events.filter(e => !e.resolved_at);
        total += unresolved.length;
        allEvents = [...allEvents, ...unresolved];
      }

      setDriftCount(total);
      // Sort by detected_at DESC, take top 5
      setRecentDrift(
        allEvents
          .sort((a, b) => new Date(b.detected_at).getTime() - new Date(a.detected_at).getTime())
          .slice(0, 5)
      );
    } catch (err) {
      console.error('Failed to load drift count:', err);
    }
  }

  function handleLogout() {
    if (confirm('Are you sure you want to logout?')) {
      localStorage.removeItem('auth_token');
      localStorage.removeItem('user');
      router.push('/auth/login');
    }
  }

  if (!isAuthenticated) return null;

  return (
    <nav className="bg-white shadow-md border-b border-gray-200">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex justify-between items-center h-16">
          {/* Logo */}
          <div className="flex items-center gap-3 cursor-pointer" onClick={() => router.push('/dashboard')}>
            <span className="text-2xl">🔐</span>
            <span className="text-xl font-bold bg-gradient-to-r from-blue-600 to-purple-600 bg-clip-text text-transparent">
              Secret Manager
            </span>
          </div>

          {/* Nav Links */}
          <div className="flex items-center gap-6">
            <NavLink href="/dashboard" active={pathname === '/dashboard'}>Dashboard</NavLink>
            <NavLink href="/secrets" active={pathname.startsWith('/secrets')}>Secrets</NavLink>
            <NavLink href="/drift" active={pathname === '/drift'}>Drift</NavLink>
            <NavLink href="/audit" active={pathname === '/audit'}>Audit</NavLink>
            <NavLink href="/sync-status" active={pathname === '/sync-status'}>Sync Status</NavLink>

            {/* Drift Badge with Dropdown */}
            {driftCount > 0 && (
              <div className="relative">
                <button
                  onClick={() => setShowDropdown(!showDropdown)}
                  className="relative p-2 rounded-lg hover:bg-gray-100 transition-colors duration-200"
                >
                  <span className="text-2xl">⚠️</span>
                  <span className="absolute -top-1 -right-1 bg-red-600 text-white text-xs font-bold rounded-full h-5 w-5 flex items-center justify-center animate-pulse">
                    {driftCount}
                  </span>
                </button>

                {/* Dropdown */}
                {showDropdown && (
                  <div className="absolute right-0 mt-2 w-80 bg-white rounded-lg shadow-xl border border-gray-200 z-50">
                    <div className="p-4 border-b border-gray-200">
                      <h3 className="text-lg font-bold text-gray-900">Drift Detected</h3>
                      <p className="text-sm text-gray-600">{driftCount} unresolved event{driftCount !== 1 ? 's' : ''}</p>
                    </div>
                    <div className="max-h-64 overflow-y-auto">
                      {recentDrift.map(event => (
                        <div key={event.id} className="p-3 border-b border-gray-100 hover:bg-gray-50 cursor-pointer" onClick={() => {
                          setShowDropdown(false);
                          router.push('/drift');
                        }}>
                          <div className="flex items-center gap-2 mb-1">
                            <span className="text-yellow-600 font-semibold">{event.secret_name}</span>
                          </div>
                          <p className="text-xs text-gray-500">
                            {new Date(event.detected_at).toLocaleString()}
                          </p>
                        </div>
                      ))}
                    </div>
                    <div className="p-3 border-t border-gray-200">
                      <button
                        onClick={() => {
                          setShowDropdown(false);
                          router.push('/drift');
                        }}
                        className="w-full px-4 py-2 bg-yellow-500 text-white rounded-lg hover:bg-yellow-600 font-medium transition-colors duration-200"
                      >
                        View All Drift Events
                      </button>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* User Menu */}
            <div className="flex items-center gap-3 ml-4 pl-4 border-l border-gray-300">
              {userEmail && (
                <span className="text-sm text-gray-600">{userEmail}</span>
              )}
              <button
                onClick={handleLogout}
                className="px-3 py-2 text-sm font-medium text-red-600 hover:bg-red-50 rounded-lg transition-colors duration-200"
                title="Logout"
              >
                Logout
              </button>
            </div>
          </div>
        </div>
      </div>
    </nav>
  );
}

function NavLink({ href, active, children }: { href: string; active: boolean; children: React.ReactNode }) {
  const router = useRouter();
  return (
    <button
      onClick={() => router.push(href)}
      className={`px-3 py-2 rounded-lg font-medium transition-colors duration-200 ${
        active
          ? 'bg-blue-100 text-blue-700'
          : 'text-gray-700 hover:bg-gray-100'
      }`}
    >
      {children}
    </button>
  );
}
