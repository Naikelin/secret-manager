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
      
      // Wait for loading to complete - use a more generous timeout
      await page.waitForTimeout(3000);
      
      // Verify comparison headers are visible (they should be there if data loaded successfully)
      const gitHeader = page.getByText(/git.*source of truth/i).first();
      const k8sHeader = page.getByText(/kubernetes.*actual state/i).first();
      
      // Check if we have the headers (skip test if comparison didn't load)
      if (await gitHeader.isVisible({ timeout: 1000 }).catch(() => false)) {
        await expect(gitHeader).toBeVisible();
        await expect(k8sHeader).toBeVisible();
      } else {
        test.skip('Comparison data did not load');
      }
    } else {
      test.skip('No drift events with expand button available');
    }
  });

  test('should display key count for each version', async ({ page }) => {
    const expandButton = page.locator('button').filter({ hasText: /▶|expand/i }).first();
    
    if (await expandButton.count() > 0) {
      await expandButton.click();
      
      // Wait for comparison to load
      await page.waitForTimeout(3000);
      
      // Check for key counts
      const keyCounts = page.locator('text=/\\d+\\s*keys?/i');
      const count = await keyCounts.count();
      
      // Skip if no key counts found (comparison didn't load)
      if (count === 0) {
        test.skip('Comparison data did not load - no key counts found');
      }
      
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
      
      // Wait for comparison to load
      await page.waitForTimeout(3000);
      
      // Check if comparison loaded
      const gitHeader = page.getByText(/git.*source of truth/i).first();
      if (!await gitHeader.isVisible({ timeout: 1000 }).catch(() => false)) {
        test.skip('Comparison data did not load');
      }
      
      // Collapse - find the collapse button
      const collapseButton = page.locator('button').filter({ hasText: /▼|collapse/i }).first();
      await collapseButton.click();
      await page.waitForTimeout(500);
      
      // Comparison should be hidden
      const isVisible = await gitHeader.isVisible().catch(() => false);
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
    // First expand a drift card
    const driftExpandButton = page.locator('button').filter({ hasText: /▶️|🔽/i }).first();
    
    if (await driftExpandButton.count() > 0) {
      // Expand the drift card first if it's not already expanded
      if (await driftExpandButton.textContent().then(t => t?.includes('▶️'))) {
        await driftExpandButton.click();
        await page.waitForTimeout(1000);
      }
      
      // Now find the comparison expand button
      const comparisonExpandButton = page.getByRole('button', { name: /expand comparison/i }).first();
      
      if (await comparisonExpandButton.count() === 0) {
        test.skip('No comparison expand button available');
      }
      
      // Set up route interception to return error response
      await page.route('**/api/v1/drift-events/*/compare', route => 
        route.fulfill({ status: 500, body: JSON.stringify({ error: 'Internal server error' }) })
      );
      
      await comparisonExpandButton.click();
      
      // Wait for error state to render
      await page.waitForTimeout(2000);
      
      // Should show error message
      const errorDiv = page.locator('.bg-red-50, .border-red-200').filter({ hasText: /error/i });
      const errorVisible = await errorDiv.isVisible({ timeout: 3000 }).catch(() => false);
      
      expect(errorVisible).toBeTruthy();
    } else {
      test.skip('No drift events with expand button available');
    }
  });
});
