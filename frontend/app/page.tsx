'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { isAuthenticated } from '@/lib/api';

export default function Home() {
  const router = useRouter();

  useEffect(() => {
    // Redirect authenticated users to dashboard, unauthenticated to login
    if (isAuthenticated()) {
      router.push('/dashboard');
    } else {
      router.push('/auth/login');
    }
  }, [router]);

  return (
    <main className="flex min-h-screen flex-col items-center justify-center p-8 bg-gradient-to-br from-blue-50 via-white to-purple-50">
      <div className="max-w-4xl w-full text-center space-y-8">
        {/* Hero Section */}
        <div className="space-y-4">
          <div className="inline-block px-4 py-2 bg-blue-100 text-blue-800 rounded-full text-sm font-medium mb-4">
            🔐 Secure Secret Management
          </div>
          <h1 className="text-5xl md:text-6xl font-bold bg-gradient-to-r from-blue-600 to-purple-600 bg-clip-text text-transparent">
            Secret Manager
          </h1>
          <p className="text-xl text-gray-600 max-w-2xl mx-auto">
            GitOps-based secret management with SOPS encryption for Kubernetes
          </p>
        </div>

        {/* Features Grid */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mt-12">
          <div className="bg-white rounded-xl shadow-md p-6 hover:shadow-lg transition-shadow duration-200">
            <div className="text-4xl mb-3">🔒</div>
            <h3 className="text-lg font-semibold text-gray-900 mb-2">Encrypted Secrets</h3>
            <p className="text-sm text-gray-600">
              SOPS encryption ensures your secrets are secure at rest and in transit
            </p>
          </div>
          
          <div className="bg-white rounded-xl shadow-md p-6 hover:shadow-lg transition-shadow duration-200">
            <div className="text-4xl mb-3">🔄</div>
            <h3 className="text-lg font-semibold text-gray-900 mb-2">GitOps Workflow</h3>
            <p className="text-sm text-gray-600">
              Full audit trail with Git-based version control and FluxCD sync
            </p>
          </div>
          
          <div className="bg-white rounded-xl shadow-md p-6 hover:shadow-lg transition-shadow duration-200">
            <div className="text-4xl mb-3">⚡</div>
            <h3 className="text-lg font-semibold text-gray-900 mb-2">Drift Detection</h3>
            <p className="text-sm text-gray-600">
              Automatic detection and resolution of configuration drift
            </p>
          </div>
        </div>

        {/* CTA Button */}
        <div className="mt-12">
          <a 
            href="/auth/login"
            className="inline-block px-8 py-4 bg-gradient-to-r from-blue-600 to-purple-600 text-white text-lg font-semibold rounded-lg hover:from-blue-700 hover:to-purple-700 transition-all duration-200 transform hover:scale-105 shadow-lg hover:shadow-xl"
          >
            Get Started →
          </a>
        </div>

        {/* Trust Indicators */}
        <div className="mt-12 pt-8 border-t border-gray-200">
          <p className="text-sm text-gray-500 mb-4">Powered by</p>
          <div className="flex items-center justify-center gap-8 text-gray-400 text-sm font-medium">
            <span>🔐 SOPS</span>
            <span>⎈ Kubernetes</span>
            <span>🔄 FluxCD</span>
            <span>📦 Git</span>
          </div>
        </div>
      </div>
    </main>
  );
}
