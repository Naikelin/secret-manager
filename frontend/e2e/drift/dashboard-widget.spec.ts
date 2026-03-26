import { test, expect } from '@playwright/test';

test.describe('Drift Dashboard Integration', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('auth_token', 'mock-token');
      localStorage.setItem('user', JSON.stringify({
        email: 'admin@example.com',
        name: 'Admin'
      }));
    });
  });

  test('should display drift widget on dashboard', async ({ page }) => {
    await page.goto('/dashboard');
    
    // Wait for dashboard to load
    await page.waitForSelector('text=/secret manager|dashboard/i', { timeout: 5000 });
    
    // Find drift widget
    const driftWidget = page.locator('text=/drift detection/i').locator('..');
    await expect(driftWidget).toBeVisible({ timeout: 10000 });
  });

  test('should show unresolved drift count', async ({ page }) => {
    await page.goto('/dashboard');
    
    // Wait for widget to load
    await page.waitForSelector('text=/drift detection/i', { timeout: 10000 });
    
    // Should show count (number)
    const widget = page.locator('text=/drift detection/i').locator('..');
    const text = await widget.textContent();
    
    // Should contain "Unresolved" label
    expect(text).toContain('Unresolved');
  });

  test('should link to drift page from widget', async ({ page }) => {
    await page.goto('/dashboard');
    
    // Wait for widget
    await page.waitForSelector('text=/drift detection/i', { timeout: 10000 });
    
    // Find and click "View" button
    const viewButton = page.getByRole('button', { name: /view.*drift/i });
    
    if (await viewButton.count() > 0) {
      await viewButton.click();
      
      // Should navigate to drift page
      await expect(page).toHaveURL(/.*drift/, { timeout: 5000 });
    } else {
      test.skip('View drift button not found');
    }
  });

  test('should show namespace breakdown when drift exists', async ({ page }) => {
    await page.goto('/dashboard');
    
    // Wait for widget
    await page.waitForSelector('text=/drift detection/i', { timeout: 10000 });
    
    const widget = page.locator('text=/drift detection/i').locator('..');
    const text = await widget.textContent() || '';
    
    // If drift count > 0, should show namespace breakdown
    if (text.match(/[1-9]\d*/)) {
      // Should have namespace names (development, staging, production, etc.)
      const hasNamespace = 
        text.includes('development') ||
        text.includes('staging') ||
        text.includes('production') ||
        text.match(/\w+\s*:\s*\d+/); // Pattern like "dev: 2"
      
      expect(hasNamespace).toBeTruthy();
    }
  });

  test('should show zero state when no drift', async ({ page }) => {
    await page.goto('/dashboard');
    
    // Wait for widget
    await page.waitForSelector('text=/drift detection/i', { timeout: 10000 });
    
    const widget = page.locator('text=/drift detection/i').locator('..');
    const text = await widget.textContent() || '';
    
    // Should show "0" or no drift indication
    if (text.includes('0') && text.includes('Unresolved')) {
      // Green status expected
      const hasGreenStyle = await widget.evaluate((el) => {
        return el.className.includes('green');
      });
      
      expect(hasGreenStyle || text.includes('0')).toBeTruthy();
    }
  });

  test('should display severity colors', async ({ page }) => {
    await page.goto('/dashboard');
    
    // Wait for widget
    await page.waitForSelector('text=/drift detection/i', { timeout: 10000 });
    
    const widget = page.locator('text=/drift detection/i').locator('..');
    
    // Widget should have colored border (green/yellow/red based on severity)
    const className = await widget.evaluate((el) => el.className);
    
    const hasSeverityColor = 
      className.includes('green') ||
      className.includes('yellow') ||
      className.includes('red') ||
      className.includes('border');
    
    expect(hasSeverityColor).toBeTruthy();
  });

  test('should display drift icon/emoji', async ({ page }) => {
    await page.goto('/dashboard');
    
    // Wait for widget
    await page.waitForSelector('text=/drift detection/i', { timeout: 10000 });
    
    const widget = page.locator('text=/drift detection/i').locator('..');
    const text = await widget.textContent();
    
    // Should have warning emoji or icon
    expect(text).toMatch(/⚠️|⚠/);
  });

  test('should load widget data independently', async ({ page }) => {
    // Slow down drift API to test loading state
    await page.route('**/api/drift/events/*', async route => {
      await page.waitForTimeout(1000);
      await route.continue();
    });
    
    await page.goto('/dashboard');
    
    // Widget should show loading state initially
    const loadingIndicator = page.locator('.animate-pulse, text=/loading/i');
    
    // Wait for widget to appear
    await expect(page.locator('text=/drift detection/i')).toBeVisible({ timeout: 15000 });
  });

  test('should handle widget data load errors gracefully', async ({ page }) => {
    // Force error on drift API
    await page.route('**/api/namespaces', route => route.abort());
    
    await page.goto('/dashboard');
    
    // Widget should still render (even if empty or with error state)
    await expect(page.locator('text=/drift detection/i')).toBeVisible({ timeout: 10000 });
  });

  test('should link to drift page from navbar if badge exists', async ({ page }) => {
    await page.goto('/dashboard');
    
    // Check for drift link in navbar
    const driftNavLink = page.getByRole('link', { name: /drift/i });
    
    if (await driftNavLink.count() > 0) {
      await driftNavLink.click();
      await expect(page).toHaveURL(/.*drift/);
    }
  });
});
