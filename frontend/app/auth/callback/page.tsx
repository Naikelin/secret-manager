'use client';

import { Suspense, useEffect } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';

function CallbackContent() {
  const router = useRouter();
  const searchParams = useSearchParams();

  useEffect(() => {
    const handleCallback = async () => {
      // Check for token first (mock auth flow - token provided directly)
      const token = searchParams.get('token');
      
      if (token) {
        try {
          // Decode JWT to extract user info
          // JWT format: header.payload.signature
          const payloadBase64 = token.split('.')[1];
          
          if (!payloadBase64) {
            throw new Error('Invalid JWT format');
          }
          
          // Decode base64url (handle padding)
          const base64 = payloadBase64.replace(/-/g, '+').replace(/_/g, '/');
          const jsonPayload = atob(base64);
          const payload = JSON.parse(jsonPayload);
          
          // Extract user info from JWT claims
          const user = {
            id: payload.user_id,
            email: payload.email,
            name: payload.name,
            groups: payload.groups || [],
          };
          
          // Store token and user info in localStorage
          localStorage.setItem('auth_token', token);
          localStorage.setItem('user', JSON.stringify(user));
          
          console.log('Mock auth successful:', user.email);
          
          // Redirect to dashboard
          router.push('/');
          return;
        } catch (error) {
          console.error('JWT decode failed:', error);
          router.push('/auth/login?error=invalid_token');
          return;
        }
      }
      
      // Traditional OAuth2 flow - check for code parameter
      const code = searchParams.get('code');
      const state = searchParams.get('state');

      if (!code) {
        router.push('/auth/login?error=missing_auth_params');
        return;
      }

      try {
        // Exchange code for token
        const response = await fetch(
          `http://localhost:8080/api/v1/auth/callback?code=${code}&state=${state}`,
          {
            method: 'GET',
            credentials: 'include',
          }
        );

        const data = await response.json();

        if (data.token) {
          // Store JWT token in localStorage
          localStorage.setItem('auth_token', data.token);
          localStorage.setItem('user', JSON.stringify(data.user));

          // Redirect to dashboard
          router.push('/');
        } else {
          throw new Error('No token received');
        }
      } catch (error) {
        console.error('Callback failed:', error);
        router.push('/auth/login?error=auth_failed');
      }
    };

    handleCallback();
  }, [router, searchParams]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="text-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mx-auto"></div>
        <p className="mt-4 text-muted-foreground">Completing authentication...</p>
      </div>
    </div>
  );
}

export default function CallbackPage() {
  return (
    <Suspense fallback={
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mx-auto"></div>
          <p className="mt-4 text-muted-foreground">Loading...</p>
        </div>
      </div>
    }>
      <CallbackContent />
    </Suspense>
  );
}
