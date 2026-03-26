import { test as base, Page } from '@playwright/test';
import * as crypto from 'crypto';

// Generate a valid UUID v4
function generateUUID(): string {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = Math.random() * 16 | 0;
    const v = c === 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

// Base64URL encoding (URL-safe base64)
function base64urlEncode(str: string): string {
  return Buffer.from(str)
    .toString('base64')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
}

// Generate valid JWT token for E2E tests
function generateTestJWT(): string {
  // JWT secret from docker-compose.e2e.yml (backend/JWT_SECRET)
  const secret = 'dev-secret-change-in-production';
  
  // JWT header (HS256 algorithm)
  const header = {
    alg: 'HS256',
    typ: 'JWT'
  };
  
  // JWT payload with all required claims
  const now = Math.floor(Date.now() / 1000);
  const payload = {
    user_id: '00000000-0000-0000-0000-000000000001', // Fixed user ID from seed data (admin@example.com)
    email: 'admin@example.com',
    name: 'Test Admin',
    groups: ['admin'],
    exp: now + 86400, // 24 hours from now
    iat: now,
    nbf: now,
    iss: 'secret-manager'
  };
  
  // Create base64url-encoded header and payload
  const encodedHeader = base64urlEncode(JSON.stringify(header));
  const encodedPayload = base64urlEncode(JSON.stringify(payload));
  
  // Create signature using HMAC-SHA256
  const signatureInput = `${encodedHeader}.${encodedPayload}`;
  const signature = crypto
    .createHmac('sha256', secret)
    .update(signatureInput)
    .digest('base64')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
  
  // Return complete JWT token
  return `${encodedHeader}.${encodedPayload}.${signature}`;
}

// Test user data (matches JWT payload and seed data)
const testUser = {
  id: '00000000-0000-0000-0000-000000000001', // Fixed user ID from seed data
  email: 'admin@example.com',
  name: 'Test Admin'
};

// Mock authenticated state
export const test = base.extend<{ authenticatedPage: Page }>({
  authenticatedPage: async ({ page }, use) => {
    // Set valid JWT auth token
    const token = generateTestJWT();
    await page.addInitScript(({ token, user }) => {
      localStorage.setItem('auth_token', token);
      localStorage.setItem('user', JSON.stringify(user));
    }, { token, user: testUser });
    await use(page);
  },
});

// Authenticated test fixture with automatic auth setup
export const authenticatedTest = base.extend<{ session: { token: string; user: typeof testUser } }>({
  page: async ({ page }, use) => {
    // Set valid JWT auth token before each test
    const token = generateTestJWT();
    await page.addInitScript(({ token, user }) => {
      localStorage.setItem('auth_token', token);
      localStorage.setItem('user', JSON.stringify(user));
    }, { token, user: testUser });
    await use(page);
  },
  session: async ({}, use) => {
    // Provide session data for tests that need it
    const token = generateTestJWT();
    await use({
      token,
      user: testUser
    });
  },
});
