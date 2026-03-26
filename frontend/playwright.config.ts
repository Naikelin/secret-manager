import { defineConfig, devices } from '@playwright/test';

// Use environment variable to determine if running in Docker
const isDocker = process.env.PLAYWRIGHT_BASE_URL !== undefined;
const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:3000';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 4 : 16, // Use 4 workers in CI, 16 locally for parallel execution
  reporter: 'html',
  timeout: 30000, // 30 seconds per test
  use: {
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    actionTimeout: 10000, // 10 seconds for actions
    navigationTimeout: 15000, // 15 seconds for navigation
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // Only start webServer if NOT in Docker (Docker Compose manages servers)
  webServer: isDocker ? undefined : {
    command: 'npm run dev',
    url: 'http://localhost:3000',
    reuseExistingServer: !process.env.CI,
  },
});
