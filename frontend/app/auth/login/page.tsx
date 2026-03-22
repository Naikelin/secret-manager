'use client';

import { useState } from 'react';

export default function LoginPage() {
  const [email, setEmail] = useState('dev@example.com');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleLogin = async () => {
    // Clear previous errors
    setError('');
    setLoading(true);

    try {
      const response = await fetch('http://localhost:8080/api/v1/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify({ email }),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({ error: 'Login failed' }));
        throw new Error(errorData.error || `HTTP ${response.status}: ${response.statusText}`);
      }

      const data = await response.json();
      
      if (data.redirect_url) {
        // Redirect to OAuth provider
        window.location.href = data.redirect_url;
      } else {
        throw new Error('No redirect URL received from server');
      }
    } catch (err) {
      console.error('Login failed:', err);
      setError(err instanceof Error ? err.message : 'Login failed. Please try again.');
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="max-w-md w-full space-y-8 p-8 bg-card rounded-lg shadow-md">
        <div className="text-center">
          <h1 className="text-3xl font-bold">Secret Manager</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            GitOps-based Kubernetes Secret Management
          </p>
        </div>
        
        <div className="mt-8 space-y-4">
          <div>
            <label htmlFor="email" className="block text-sm font-medium mb-2">
              Email Address
            </label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => {
                setEmail(e.target.value);
                setError('');
              }}
              placeholder="Enter your email"
              className="w-full px-3 py-2 border border-input rounded-md bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:border-transparent"
              disabled={loading}
              required
            />
          </div>

          <button
            onClick={handleLogin}
            disabled={loading || !email || !email.includes('@')}
            className="w-full flex justify-center py-3 px-4 border border-transparent text-sm font-medium rounded-md text-primary-foreground bg-primary hover:opacity-90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-ring transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? 'Logging in...' : 'Login with OAuth'}
          </button>

          {error && (
            <div className="p-3 rounded-md bg-destructive/10 border border-destructive/20">
              <p className="text-sm text-destructive">{error}</p>
            </div>
          )}
        </div>

        <div className="mt-4 text-center text-xs text-muted-foreground space-y-1">
          <p>Development Mode: Mock OAuth Provider</p>
          <p>Use dev@example.com or admin@example.com for testing</p>
        </div>
      </div>
    </div>
  );
}
