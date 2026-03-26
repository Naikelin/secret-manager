import { test, expect } from '@playwright/test';
import { authenticatedTest } from '../fixtures/auth';

authenticatedTest.describe('Delete Secret', () => {
  authenticatedTest('should delete draft secret', async ({ page, session }) => {
    await page.goto('/secrets');
    
    // Create a test secret first
    await page.getByRole('button', { name: 'Create Secret' }).first().click();
    const secretName = `e2e-delete-${Date.now()}`;
    await page.getByLabel('Secret Name').fill(secretName);
    await page.getByLabel('Namespace').selectOption('10000000-0000-0000-0000-000000000001');
    await page.getByLabel('Key').fill('test');
    await page.getByLabel('Value').fill('value');
    await page.getByRole('button', { name: 'Add Key' }).click();
    await page.getByRole('button', { name: 'Create Draft' }).click();
    
    // Wait for create success message to appear and then disappear
    await expect(page.getByTestId('success-message')).toBeVisible({ timeout: 10000 });
    await expect(page.getByTestId('success-message')).not.toBeVisible({ timeout: 6000 });
    
    // Now delete it
    const secretRow = page.getByRole('row').filter({ hasText: secretName });
    await secretRow.getByRole('button', { name: 'Delete' }).click();
    await page.getByRole('button', { name: 'Confirm Delete' }).click();
    
    // Verify deletion success message
    await expect(page.getByTestId('success-message')).toBeVisible({ timeout: 10000 });
    await expect(page.getByTestId('success-message')).toHaveText('Secret deleted');
    
    // Verify secret is removed from list
    await expect(secretRow).not.toBeVisible();
  });

  authenticatedTest('should prevent deletion of published secrets', async ({ page, session }) => {
    await page.goto('/secrets');
    
    // Find a published secret
    const publishedRow = page.getByRole('row').filter({ hasText: 'published' }).first();
    
    if (await publishedRow.count() > 0) {
      const deleteBtn = publishedRow.getByRole('button', { name: 'Delete' });
      
      // Verify delete button is disabled or shows warning
      await expect(deleteBtn).toBeDisabled();
    }
  });
});
