import { test, expect } from '@playwright/test';

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
    // Mock authentication
    await page.addInitScript(() => {
      localStorage.setItem('auth_token', 'mock-token');
      localStorage.setItem('user', JSON.stringify({
        email: 'admin@example.com',
        name: 'Admin User'
      }));
    });
    
    await page.goto('/');
    await expect(page).toHaveURL('/dashboard');
    await expect(page.getByText('Secret Manager')).toBeVisible();
  });

  test('should show user email in navbar when authenticated', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('auth_token', 'mock-token');
      localStorage.setItem('user', JSON.stringify({
        email: 'test@example.com',
        name: 'Test User'
      }));
    });
    
    await page.goto('/dashboard');
    await expect(page.getByText('test@example.com')).toBeVisible();
  });

  test('should logout and clear storage', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('auth_token', 'mock-token');
      localStorage.setItem('user', JSON.stringify({ email: 'test@example.com' }));
    });
    
    await page.goto('/dashboard');
    await page.getByRole('button', { name: /logout/i }).click();
    
    // Confirm dialog
    page.once('dialog', dialog => dialog.accept());
    
    await expect(page).toHaveURL(/.*login/);
    
    // Check storage cleared
    const token = await page.evaluate(() => localStorage.getItem('auth_token'));
    expect(token).toBeNull();
  });
});
