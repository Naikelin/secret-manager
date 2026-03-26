import { authenticatedTest as test } from '../fixtures/auth';
import { expect } from '@playwright/test';

test.describe('Drift Detection Page', () => {
  // Auth is automatically set up by authenticatedTest fixture

  test('should display drift detection page', async ({ page }) => {
    await page.goto('/drift');
    
    await expect(page.getByRole('heading', { name: /drift detection/i, level: 1 })).toBeVisible();
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
    
    // Get initial drift events count
    const initialDriftCards = await page.locator('.bg-white.rounded-xl').filter({ hasText: /detected/i }).count();
    
    // Click check button
    const checkButton = page.getByRole('button', { name: /check for drift/i });
    await checkButton.click();
    
    // Wait a moment for the check to complete (button should be briefly disabled then re-enabled)
    await page.waitForTimeout(500);
    
    // Button should be enabled again after completion
    await expect(checkButton).toBeEnabled({ timeout: 10000 });
    
    // Verify the page is still functional (can click again)
    await expect(checkButton).toContainText(/check for drift/i);
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
    
    // Wait for namespaces to load using data attribute
    await page.waitForSelector('select[data-loaded="true"]', { timeout: 5000 });
    
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
    // Set up route interception to return HTTP 500 error BEFORE navigation
    await page.route('**/api/v1/namespaces', route => {
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Internal server error' })
      });
    });
    
    await page.goto('/drift');
    
    // Wait for error banner to appear
    const errorBanner = page.getByTestId('error-banner');
    await expect(errorBanner).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/internal server error|http 500/i)).toBeVisible();
  });
});
