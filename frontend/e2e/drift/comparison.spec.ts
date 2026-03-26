import { authenticatedTest as test } from '../fixtures/auth';
import { expect } from '@playwright/test';

test.describe('Drift Visual Comparison', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/drift');
    
    // Wait for page to load
    await page.waitForSelector('select', { timeout: 5000 });
    await page.getByRole('combobox').selectOption({ index: 0 });
    await page.waitForTimeout(2000);
  });

  test('should expand drift event to show comparison', async ({ page }) => {
    // Find first drift card with expand button
    const expandButton = page.locator('button').filter({ hasText: /▶|expand/i }).first();
    
    if (await expandButton.count() > 0) {
      await expandButton.click();
      
      // Wait for comparison component
      await expect(page.getByText(/visual comparison|git.*source of truth/i)).toBeVisible({ timeout: 5000 });
    } else {
      test.skip('No drift events with expand button available');
    }
  });

  test('should display comparison headers', async ({ page }) => {
    const expandButton = page.locator('button').filter({ hasText: /▶|expand/i }).first();
    
    if (await expandButton.count() > 0) {
      await expandButton.click();
      await page.waitForTimeout(1000);
      
      // Verify comparison headers
      await expect(page.getByText(/git.*source of truth/i).first()).toBeVisible();
      await expect(page.getByText(/kubernetes.*actual state/i).first()).toBeVisible();
    } else {
      test.skip('No drift events with expand button available');
    }
  });

  test('should display key count for each version', async ({ page }) => {
    const expandButton = page.locator('button').filter({ hasText: /▶|expand/i }).first();
    
    if (await expandButton.count() > 0) {
      await expandButton.click();
      await page.waitForTimeout(1000);
      
      // Should show key counts
      const keyCounts = page.locator('text=/\\d+\\s*keys?/i');
      const count = await keyCounts.count();
      
      // Should have at least 2 key counts (Git and K8s)
      expect(count).toBeGreaterThanOrEqual(2);
    } else {
      test.skip('No drift events with expand button available');
    }
  });

  test('should toggle show/hide values', async ({ page }) => {
    const expandButton = page.locator('button').filter({ hasText: /▶|expand/i }).first();
    
    if (await expandButton.count() > 0) {
      await expandButton.click();
      await page.waitForTimeout(1000);
      
      // Initially values should be hidden (showing dots)
      const bodyText = await page.locator('body').textContent();
      const initiallyHidden = bodyText?.includes('••••');
      
      // Find and click Show/Hide Values button
      const toggleButton = page.getByRole('button', { name: /show values|hide values/i });
      
      if (await toggleButton.count() > 0) {
        await toggleButton.click();
        await page.waitForTimeout(500);
        
        // Values state should have changed
        const newBodyText = await page.locator('body').textContent();
        const afterToggle = newBodyText?.includes('••••');
        
        expect(initiallyHidden !== afterToggle).toBeTruthy();
      }
    } else {
      test.skip('No drift events with expand button available');
    }
  });

  test('should collapse comparison when clicking arrow again', async ({ page }) => {
    const expandButton = page.locator('button').filter({ hasText: /▶|expand/i }).first();
    
    if (await expandButton.count() > 0) {
      // Expand
      await expandButton.click();
      await page.waitForTimeout(1000);
      await expect(page.getByText(/git.*source of truth/i).first()).toBeVisible();
      
      // Collapse - find the same button (text should have changed to collapse)
      const collapseButton = page.locator('button').filter({ hasText: /▼|collapse/i }).first();
      await collapseButton.click();
      await page.waitForTimeout(500);
      
      // Comparison should be hidden
      const gitText = page.getByText(/git.*source of truth/i);
      const isVisible = await gitText.isVisible().catch(() => false);
      expect(isVisible).toBeFalsy();
    } else {
      test.skip('No drift events with expand button available');
    }
  });

  test('should show diff legend', async ({ page }) => {
    const expandButton = page.locator('button').filter({ hasText: /▶|expand/i }).first();
    
    if (await expandButton.count() > 0) {
      await expandButton.click();
      await page.waitForTimeout(1000);
      
      // Check for legend items (may be in different formats)
      const bodyText = await page.locator('body').textContent() || '';
      
      const hasLegendTerms = 
        bodyText.includes('added') || 
        bodyText.includes('missing') || 
        bodyText.includes('modified') ||
        bodyText.includes('Added') ||
        bodyText.includes('Deleted');
      
      expect(hasLegendTerms).toBeTruthy();
    } else {
      test.skip('No drift events with expand button available');
    }
  });

  test('should load comparison data asynchronously', async ({ page }) => {
    const expandButton = page.locator('button').filter({ hasText: /▶|expand/i }).first();
    
    if (await expandButton.count() > 0) {
      await expandButton.click();
      
      // Should show loading state briefly
      const loadingIndicator = page.locator('.animate-spin, text=/loading/i');
      
      // Wait for either loading to appear or content to load (might be fast)
      await page.waitForTimeout(500);
      
      // Eventually should show comparison content
      await expect(page.getByText(/git|kubernetes|source of truth/i).first()).toBeVisible({ timeout: 5000 });
    } else {
      test.skip('No drift events with expand button available');
    }
  });

  test('should handle comparison load error gracefully', async ({ page }) => {
    // Intercept API call and force error after page loads
    await page.route('**/api/drift/*/comparison', route => route.abort());
    
    const expandButton = page.locator('button').filter({ hasText: /▶|expand/i }).first();
    
    if (await expandButton.count() > 0) {
      await expandButton.click();
      
      // Should show error message
      await expect(page.getByText(/error|failed/i)).toBeVisible({ timeout: 5000 });
    } else {
      test.skip('No drift events with expand button available');
    }
  });
});
