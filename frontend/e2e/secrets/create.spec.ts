import { test, expect } from '@playwright/test';
import { authenticatedTest } from '../fixtures/auth';

authenticatedTest.describe('Create Secret', () => {
  authenticatedTest('should create new secret draft successfully', async ({ page, session }) => {
    await page.goto('/secrets');
    
    // Click create button
    await page.getByRole('button', { name: 'Create Secret' }).first().click();
    
    // Fill form
    await page.getByLabel('Secret Name').fill(`e2e-test-${Date.now()}`);
    // Select by visible label (namespace displays as "name (cluster)")
    await page.getByLabel('Namespace').selectOption({ label: /development/ });
    await page.getByLabel('Key').fill('username');
    await page.getByLabel('Value').fill('testuser');
    await page.getByRole('button', { name: 'Add Key' }).click();
    
    // Submit
    await page.getByRole('button', { name: 'Create Draft' }).click();
    
    // Verify success
    await expect(page.getByText('Secret draft created')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText('draft')).toBeVisible();
  });

  authenticatedTest('should validate required fields', async ({ page, session }) => {
    await page.goto('/secrets');
    await page.getByRole('button', { name: 'Create Secret' }).first().click();
    
    // Try to submit without filling required fields
    await page.getByRole('button', { name: 'Create Draft' }).click();
    
    // Verify validation errors
    await expect(page.getByText('Secret name is required')).toBeVisible();
    await expect(page.getByText('Namespace is required')).toBeVisible();
  });

  authenticatedTest('should require at least one key-value pair', async ({ page, session }) => {
    await page.goto('/secrets');
    await page.getByRole('button', { name: 'Create Secret' }).first().click();
    
    // Fill name and namespace only
    await page.getByLabel('Secret Name').fill('test-no-keys');
    await page.getByLabel('Namespace').selectOption({ label: /development/ });
    await page.getByRole('button', { name: 'Create Draft' }).click();
    
    // Verify error
    await expect(page.getByText('At least one key-value pair required')).toBeVisible();
  });
});
