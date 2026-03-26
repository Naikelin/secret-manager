import { test as base } from '@playwright/test';

// Mock authenticated state
export const test = base.extend({
  authenticatedPage: async ({ page }, use) => {
    // Set mock auth token
    await page.addInitScript(() => {
      localStorage.setItem('auth_token', 'mock-jwt-token-for-testing');
      localStorage.setItem('user', JSON.stringify({
        id: 'test-user-id',
        email: 'admin@example.com',
        name: 'Test Admin'
      }));
    });
    await use(page);
  },
});
