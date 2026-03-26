import { test, expect } from '@playwright/test';
import { authenticatedTest } from '../fixtures/auth';

authenticatedTest.describe('Edit and Publish Secret', () => {
  authenticatedTest('should edit existing draft secret', async ({ page, session }) => {
    await page.goto('/secrets');
    
    // Find a draft secret
    const draftRow = page.getByRole('row').filter({ hasText: 'draft' }).first();
    await draftRow.getByRole('button', { name: 'Edit' }).click();
    
    // Modify key value
    await page.getByLabel('Value').first().fill('updated-value');
    await page.getByRole('button', { name: 'Save Draft' }).click();
    
    // Verify success
    await expect(page.getByText('Secret updated')).toBeVisible();
  });

  authenticatedTest('should publish secret to GitOps', async ({ page, session }) => {
    await page.goto('/secrets');
    
    // Find a draft secret
    const draftRow = page.getByRole('row').filter({ hasText: 'draft' }).first();
    await draftRow.getByRole('button', { name: 'Publish' }).click();
    
    // Confirm publish dialog
    await page.getByRole('button', { name: 'Confirm Publish' }).click();
    
    // Verify success message appears
    await expect(page.getByTestId('success-message')).toBeVisible({ timeout: 10000 });
    await expect(page.getByTestId('success-message')).toHaveText('Secret published');
    
    // Verify status changes to published
    await expect(page.getByText('published')).toBeVisible();
  });

  authenticatedTest('should prevent publishing drifted secrets', async ({ page, session }) => {
    await page.goto('/secrets');
    
    // Find a drifted secret (if exists)
    const driftedRow = page.getByRole('row').filter({ hasText: 'drifted' }).first();
    
    if (await driftedRow.count() > 0) {
      const publishBtn = driftedRow.getByRole('button', { name: 'Publish' });
      
      // Verify publish button is disabled
      await expect(publishBtn).toBeDisabled();
      
      // Verify warning message
      await expect(page.getByText('Resolve drift before publishing')).toBeVisible();
    }
  });
});
