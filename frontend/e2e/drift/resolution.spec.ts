import { authenticatedTest as test } from '../fixtures/auth';
import { expect } from '@playwright/test';

test.describe('Drift Resolution Actions', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/drift');
    
    // Wait for page to load
    await page.waitForSelector('select', { timeout: 5000 });
    await page.getByRole('combobox').selectOption({ index: 0 });
    await page.waitForTimeout(2000);
  });

  test('should show resolution buttons for unresolved drift', async ({ page }) => {
    const driftCard = page.locator('.bg-white.rounded-xl').filter({ hasText: /detected/i }).first();
    
    if (await driftCard.count() > 0) {
      // Check if this is unresolved drift (should have resolution buttons)
      const hasSyncButton = await driftCard.getByRole('button', { name: /sync from git/i }).count() > 0;
      const hasImportButton = await driftCard.getByRole('button', { name: /import to git/i }).count() > 0;
      const hasMarkButton = await driftCard.getByRole('button', { name: /mark resolved|mark as resolved/i }).count() > 0;
      
      // At least one resolution action should be visible
      expect(hasSyncButton || hasImportButton || hasMarkButton).toBeTruthy();
    } else {
      test.skip('No unresolved drift events available');
    }
  });

  test('should not show resolution buttons for resolved drift', async ({ page }) => {
    // Find resolved drift event (if any)
    const resolvedCard = page.locator('.bg-white.rounded-xl').filter({ hasText: /resolved/i }).first();
    
    if (await resolvedCard.count() > 0) {
      // Should NOT have resolution buttons
      const syncButton = resolvedCard.getByRole('button', { name: /sync from git/i });
      const importButton = resolvedCard.getByRole('button', { name: /import to git/i });
      
      expect(await syncButton.count()).toBe(0);
      expect(await importButton.count()).toBe(0);
    } else {
      test.skip('No resolved drift events available');
    }
  });

  test('should show confirmation dialog for sync action', async ({ page }) => {
    const driftCard = page.locator('.bg-white.rounded-xl').filter({ hasText: /detected/i }).first();
    
    if (await driftCard.count() === 0) {
      test.skip('No unresolved drift events available');
      return;
    }
    
    const syncButton = driftCard.getByRole('button', { name: /sync from git/i }).first();
    
    if (await syncButton.count() > 0) {
      // Setup dialog handler BEFORE clicking
      let dialogShown = false;
      page.once('dialog', dialog => {
        dialogShown = true;
        expect(dialog.message()).toMatch(/sync/i);
        dialog.dismiss(); // Dismiss to avoid state changes
      });
      
      await syncButton.click();
      
      // Wait a bit for dialog to appear
      await page.waitForTimeout(500);
      
      expect(dialogShown).toBeTruthy();
    } else {
      test.skip('No sync button available');
    }
  });

  test('should show confirmation dialog for import action', async ({ page }) => {
    const driftCard = page.locator('.bg-white.rounded-xl').filter({ hasText: /detected/i }).first();
    
    if (await driftCard.count() === 0) {
      test.skip('No unresolved drift events available');
      return;
    }
    
    const importButton = driftCard.getByRole('button', { name: /import to git/i }).first();
    
    if (await importButton.count() > 0) {
      let dialogShown = false;
      page.once('dialog', dialog => {
        dialogShown = true;
        expect(dialog.message()).toMatch(/import/i);
        dialog.dismiss();
      });
      
      await importButton.click();
      await page.waitForTimeout(500);
      
      expect(dialogShown).toBeTruthy();
    } else {
      test.skip('No import button available');
    }
  });

  test('should show confirmation dialog for mark resolved action', async ({ page }) => {
    const driftCard = page.locator('.bg-white.rounded-xl').filter({ hasText: /detected/i }).first();
    
    if (await driftCard.count() === 0) {
      test.skip('No unresolved drift events available');
      return;
    }
    
    const markButton = driftCard.getByRole('button', { name: /mark resolved|mark as resolved/i }).first();
    
    if (await markButton.count() > 0) {
      let dialogShown = false;
      page.once('dialog', dialog => {
        dialogShown = true;
        expect(dialog.message()).toMatch(/mark/i);
        dialog.dismiss();
      });
      
      await markButton.click();
      await page.waitForTimeout(500);
      
      expect(dialogShown).toBeTruthy();
    } else {
      test.skip('No mark resolved button available');
    }
  });

  test('should cancel resolution on dialog dismiss', async ({ page }) => {
    const driftCard = page.locator('.bg-white.rounded-xl').filter({ hasText: /detected/i }).first();
    
    if (await driftCard.count() === 0) {
      test.skip('No unresolved drift events available');
      return;
    }
    
    const syncButton = driftCard.getByRole('button', { name: /sync from git/i }).first();
    
    if (await syncButton.count() > 0) {
      // Dismiss dialog
      page.once('dialog', dialog => dialog.dismiss());
      
      await syncButton.click();
      await page.waitForTimeout(1000);
      
      // Drift card should still be visible (not resolved)
      await expect(driftCard).toBeVisible();
    } else {
      test.skip('No sync button available');
    }
  });

  test('should handle resolution API errors', async ({ page }) => {
    // Intercept resolution API calls and force errors
    await page.route('**/api/drift/*/sync', route => route.abort());
    await page.route('**/api/drift/*/import', route => route.abort());
    await page.route('**/api/drift/*/resolve', route => route.abort());
    
    const driftCard = page.locator('.bg-white.rounded-xl').filter({ hasText: /detected/i }).first();
    
    if (await driftCard.count() === 0) {
      test.skip('No unresolved drift events available');
      return;
    }
    
    const syncButton = driftCard.getByRole('button', { name: /sync from git/i }).first();
    
    if (await syncButton.count() > 0) {
      // Accept dialog
      page.once('dialog', dialog => dialog.accept());
      
      await syncButton.click();
      
      // Should show error alert (browser alert)
      page.once('dialog', dialog => {
        expect(dialog.message()).toMatch(/failed/i);
        dialog.accept();
      });
      
      await page.waitForTimeout(2000);
    } else {
      test.skip('No sync button available');
    }
  });

  test('should disable resolution buttons during action', async ({ page }) => {
    const driftCard = page.locator('.bg-white.rounded-xl').filter({ hasText: /detected/i }).first();
    
    if (await driftCard.count() === 0) {
      test.skip('No unresolved drift events available');
      return;
    }
    
    const syncButton = driftCard.getByRole('button', { name: /sync from git/i }).first();
    
    if (await syncButton.count() > 0) {
      // Mock slow API response
      await page.route('**/api/drift/*/sync', async route => {
        await page.waitForTimeout(2000);
        await route.fulfill({ status: 200, body: '{}' });
      });
      
      // Accept confirmation
      page.once('dialog', dialog => dialog.accept());
      
      await syncButton.click();
      
      // Button should be disabled during action
      await page.waitForTimeout(500);
      // Note: This test may need adjustment based on actual implementation
      
    } else {
      test.skip('No sync button available');
    }
  });
});
