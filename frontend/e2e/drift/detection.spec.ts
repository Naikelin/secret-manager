import { test, expect } from '@playwright/test';

test.describe('Drift Detection Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('auth_token', 'mock-token');
      localStorage.setItem('user', JSON.stringify({
        email: 'admin@example.com',
        name: 'Admin'
      }));
    });
  });

  test('should display drift detection page', async ({ page }) => {
    await page.goto('/drift');
    
    await expect(page.getByRole('heading', { name: /drift detection/i })).toBeVisible();
    await expect(page.getByRole('combobox')).toBeVisible();
    await expect(page.getByRole('button', { name: /check for drift/i })).toBeVisible();
  });

  test('should trigger manual drift check', async ({ page }) => {
    await page.goto('/drift');
    
    // Wait for namespaces to load
    await page.waitForSelector('select', { timeout: 5000 });
    
    // Select first namespace
    const select = page.getByRole('combobox');
    await select.selectOption({ index: 0 });
    
    // Click check button
    const checkButton = page.getByRole('button', { name: /check for drift/i });
    await checkButton.click();
    
    // Should show loading state
    await expect(page.getByText(/checking/i)).toBeVisible();
    
    // Wait for completion
    await expect(checkButton).toBeEnabled({ timeout: 10000 });
  });

  test('should display drift events list', async ({ page }) => {
    await page.goto('/drift');
    
    // Wait for page to load
    await page.waitForSelector('select', { timeout: 5000 });
    
    // Select namespace
    const select = page.getByRole('combobox');
    await select.selectOption({ index: 0 });
    
    // Wait for drift events to load (or "no drift" message)
    await page.waitForTimeout(2000);
    
    // Should show either drift events or no drift message
    const hasDrift = await page.locator('.bg-white.rounded-xl').filter({ hasText: /detected|resolved/i }).count() > 0;
    const noDrift = await page.getByText(/no drift detected/i).isVisible();
    
    expect(hasDrift || noDrift).toBeTruthy();
  });

  test('should show drift event details', async ({ page }) => {
    await page.goto('/drift');
    
    // Wait for namespaces to load
    await page.waitForSelector('select', { timeout: 5000 });
    
    // Select first namespace
    await page.getByRole('combobox').selectOption({ index: 0 });
    
    // Wait for drift events
    await page.waitForTimeout(2000);
    
    // Check if drift events exist
    const driftCard = page.locator('.bg-white.rounded-xl').filter({ hasText: /detected/i }).first();
    
    if (await driftCard.count() > 0) {
      // Verify drift event has required information
      await expect(driftCard).toBeVisible();
      
      // Should show timestamp
      const hasTimestamp = await driftCard.locator('text=/ago|just now/i').count() > 0;
      expect(hasTimestamp).toBeTruthy();
    }
  });

  test('should display namespace selector with options', async ({ page }) => {
    await page.goto('/drift');
    
    // Wait for namespaces to load
    await page.waitForSelector('select option', { timeout: 5000 });
    
    const select = page.getByRole('combobox');
    const options = await select.locator('option').count();
    
    expect(options).toBeGreaterThan(0);
  });

  test('should show info box about drift types', async ({ page }) => {
    await page.goto('/drift');
    
    // Check for info box
    await expect(page.getByText(/about drift detection/i)).toBeVisible();
    await expect(page.getByText(/modified.*secret values differ/i)).toBeVisible();
    await expect(page.getByText(/deleted.*exists in git but not/i)).toBeVisible();
    await expect(page.getByText(/added.*exists in kubernetes but not/i)).toBeVisible();
  });

  test('should display error message on API failure', async ({ page }) => {
    await page.route('**/api/namespaces', route => route.abort());
    
    await page.goto('/drift');
    
    // Should show error
    await expect(page.getByText(/error|failed/i)).toBeVisible({ timeout: 5000 });
  });
});
