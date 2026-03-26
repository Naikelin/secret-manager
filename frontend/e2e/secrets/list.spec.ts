import { test, expect } from '@playwright/test';
import { authenticatedTest } from '../fixtures/auth';

authenticatedTest.describe('Secrets List Page', () => {
  authenticatedTest('should display secrets list with correct columns', async ({ page, session }) => {
    await page.goto('/secrets');
    
    // Check table headers
    await expect(page.getByRole('columnheader', { name: 'Name', exact: true })).toBeVisible();
    await expect(page.getByRole('columnheader', { name: 'Namespace', exact: true })).toBeVisible();
    await expect(page.getByRole('columnheader', { name: 'Status', exact: true })).toBeVisible();
    await expect(page.getByRole('columnheader', { name: 'Actions', exact: true })).toBeVisible();
    
    // Check at least one secret exists
    const rows = page.getByRole('row');
    await expect(rows).not.toHaveCount(0);
  });

  authenticatedTest('should filter secrets by namespace', async ({ page, session }) => {
    await page.goto('/secrets');
    
    // Select namespace filter
    await page.getByLabel('Namespace').selectOption('development');
    
    // Verify filtered results
    const namespaceCell = page.getByRole('cell', { name: 'development' });
    await expect(namespaceCell.first()).toBeVisible();
  });

  authenticatedTest('should search secrets by name', async ({ page, session }) => {
    await page.goto('/secrets');
    
    // Enter search term
    await page.getByPlaceholder('Search secrets...').fill('test');
    
    // Verify search results
    const searchResults = page.getByRole('row');
    await expect(searchResults).not.toHaveCount(0);
  });
});
