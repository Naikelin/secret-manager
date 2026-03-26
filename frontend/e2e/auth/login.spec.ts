import { test, expect } from '@playwright/test';
import * as crypto from 'crypto';

// Base64URL encoding (URL-safe base64)
function base64urlEncode(str: string): string {
  return Buffer.from(str)
    .toString('base64')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
}

// Generate a valid UUID v4
function generateUUID(): string {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = Math.random() * 16 | 0;
    const v = c === 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

// Generate valid JWT token for E2E tests
function generateTestJWT(): string {
  const secret = 'dev-secret-change-in-production';
  
  const header = {
    alg: 'HS256',
    typ: 'JWT'
  };
  
  const now = Math.floor(Date.now() / 1000);
  const payload = {
    user_id: '00000000-0000-0000-0000-000000000001', // Fixed user ID from seed data (admin@example.com)
    email: 'admin@example.com',
    name: 'Test Admin',
    groups: ['admin'],
    exp: now + 86400,
    iat: now,
    nbf: now,
    iss: 'secret-manager'
  };
  
  const encodedHeader = base64urlEncode(JSON.stringify(header));
  const encodedPayload = base64urlEncode(JSON.stringify(payload));
  
  const signatureInput = `${encodedHeader}.${encodedPayload}`;
  const signature = crypto
    .createHmac('sha256', secret)
    .update(signatureInput)
    .digest('base64')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
  
  return `${encodedHeader}.${encodedPayload}.${signature}`;
}

test.describe('Authentication Flow', () => {
  test('should redirect to login when not authenticated', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveURL(/.*login/);
  });

  test('should show login page with OAuth button', async ({ page }) => {
    await page.goto('/auth/login');
    const oauthButton = page.getByRole('button', { name: /login with oauth/i });
    await expect(oauthButton).toBeVisible();
  });

  test('should show dashboard when authenticated', async ({ page }) => {
    // Set up valid JWT authentication
    const token = generateTestJWT();
    await page.addInitScript(({ token }) => {
      localStorage.setItem('auth_token', token);
      localStorage.setItem('user', JSON.stringify({
        email: 'admin@example.com',
        name: 'Admin User'
      }));
    }, { token });
    
    await page.goto('/');
    await expect(page).toHaveURL('/dashboard');
    await expect(page.getByText('Secret Manager')).toBeVisible();
  });

  test('should show user email in navbar when authenticated', async ({ page }) => {
    const token = generateTestJWT();
    await page.addInitScript(({ token }) => {
      localStorage.setItem('auth_token', token);
      localStorage.setItem('user', JSON.stringify({
        email: 'test@example.com',
        name: 'Test User'
      }));
    }, { token });
    
    await page.goto('/dashboard');
    await expect(page.getByText('test@example.com')).toBeVisible();
  });

  test('should logout and clear storage', async ({ page }) => {
    const token = generateTestJWT();
    await page.addInitScript(({ token }) => {
      localStorage.setItem('auth_token', token);
      localStorage.setItem('user', JSON.stringify({ email: 'test@example.com' }));
    }, { token });
    
    await page.goto('/dashboard');
    await page.getByRole('button', { name: /logout/i }).click();
    
    // Confirm dialog
    page.once('dialog', dialog => dialog.accept());
    
    await expect(page).toHaveURL(/.*login/);
    
    // Check storage cleared
    const storedToken = await page.evaluate(() => localStorage.getItem('auth_token'));
    expect(storedToken).toBeNull();
  });
});
