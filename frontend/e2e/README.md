# E2E Testing with Playwright

## Overview

This directory contains end-to-end tests for the Secret Manager frontend using Playwright.

## Installation

Playwright and Chromium browser are already installed. If you need to reinstall:

```bash
npm install -D @playwright/test
npx playwright install chromium
```

## System Dependencies

Playwright requires certain system libraries. On most systems, install them with:

```bash
# Ubuntu/Debian
npx playwright install-deps chromium

# Or manually
sudo apt-get install -y \
  libnss3 \
  libnspr4 \
  libatk1.0-0 \
  libatk-bridge2.0-0 \
  libcups2 \
  libdrm2 \
  libxkbcommon0 \
  libxcomposite1 \
  libxdamage1 \
  libxfixes3 \
  libxrandr2 \
  libgbm1 \
  libasound2
```

If your system doesn't have package managers, you can:
1. Run tests in Docker (see below)
2. Use a CI/CD environment with proper dependencies
3. Run tests in headed mode if GUI is available: `npm run test:e2e:headed`

## Running Tests

```bash
# Run all tests (headless)
npm run test:e2e

# Run with UI mode (interactive)
npm run test:e2e:ui

# Run in headed mode (watch browser)
npm run test:e2e:headed

# View last test report
npm run test:e2e:report
```

## Test Structure

```
e2e/
├── auth/              # Authentication flow tests
│   └── login.spec.ts
├── secrets/           # Secret lifecycle tests (11 tests)
│   ├── list.spec.ts
│   ├── create.spec.ts
│   ├── edit-publish.spec.ts
│   └── delete.spec.ts
├── drift/             # Drift detection tests
│   ├── detection.spec.ts
│   ├── comparison.spec.ts
│   ├── dashboard-widget.spec.ts
│   └── resolution.spec.ts
└── fixtures/          # Test fixtures and helpers
    └── auth.ts
```

## Writing Tests

### Authentication Fixture

Use the `authenticatedTest` fixture for tests requiring authentication:

```typescript
import { authenticatedTest } from '../fixtures/auth';

authenticatedTest('should access protected page', async ({ page, session }) => {
  await page.goto('/dashboard');
  // Test authenticated features
  // session.user and session.token are available
});
```

Or use the `authenticatedPage` fixture:

```typescript
import { test } from '../fixtures/auth';

test('should access protected page', async ({ authenticatedPage }) => {
  await authenticatedPage.goto('/dashboard');
  // Test authenticated features
});
```

### Mock Authentication

For custom authentication states:

```typescript
test('should work with custom user', async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('auth_token', 'mock-token');
    localStorage.setItem('user', JSON.stringify({
      email: 'test@example.com',
      name: 'Test User'
    }));
  });
  await page.goto('/dashboard');
});
```

## Docker Testing

Run tests in complete isolation with Docker (includes database, backend, frontend, and test runner):

```bash
# Run full E2E test suite in Docker
./scripts/e2e-docker.sh
```

This script:
1. Cleans up any existing containers and volumes
2. Starts PostgreSQL database
3. Runs seed data population script
4. Starts backend API server
5. Starts frontend Next.js app
6. Runs Playwright tests
7. On success: Cleans up containers
8. On failure: Keeps containers running for debugging

### Seed Data

The E2E environment is pre-populated with test data:

**Users:**
- `admin@example.com` - Admin User
- `test@example.com` - Test User  
- `developer@example.com` - Developer User

**Namespaces:**
- `development` (dev environment)
- `staging` (staging environment)
- `production` (prod environment)

**Secrets:**
- 9 secrets across all namespaces
- Mix of `draft` and `published` statuses
- Examples: `api-credentials`, `database-config`, `oauth-config`, `tls-certificates`, `smtp-config`

**Drift Events:**
- 5 drift events total
- 3 unresolved (keys modified, added, removed)
- 2 resolved (synced from git, ignored)
- Realistic comparison data with Git vs K8s differences

### Debugging Failed Docker Tests

If tests fail, containers remain running for debugging:

```bash
# View service logs
docker-compose -f docker-compose.e2e.yml logs postgres
docker-compose -f docker-compose.e2e.yml logs seed
docker-compose -f docker-compose.e2e.yml logs backend
docker-compose -f docker-compose.e2e.yml logs frontend

# Access running services
# Backend:  http://localhost:8081
# Frontend: http://localhost:3001

# Check database data
docker-compose -f docker-compose.e2e.yml exec postgres psql -U dev -d secretmanager -c "SELECT * FROM users;"
docker-compose -f docker-compose.e2e.yml exec postgres psql -U dev -d secretmanager -c "SELECT * FROM namespaces;"
docker-compose -f docker-compose.e2e.yml exec postgres psql -U dev -d secretmanager -c "SELECT * FROM drift_events;"

# Clean up when done
docker-compose -f docker-compose.e2e.yml down -v
```

### Adding More Seed Data

Edit `backend/scripts/seed-e2e.go` to add more test data:

1. **Add Users**: Update `createUsers()` function
2. **Add Namespaces**: Update `createNamespaces()` function
3. **Add Secrets**: Update `createSecrets()` function
4. **Add Drift Events**: Update `createDriftEvents()` function

The seed script is idempotent - it cleans all existing data before seeding.

### Manual Docker Testing

If you prefer system dependencies, run tests locally:

```bash
# Build test image
docker build -t secret-manager-e2e -f Dockerfile.e2e .

# Run tests
docker run --rm \
  --network host \
  secret-manager-e2e \
  npm run test:e2e
```

Create `Dockerfile.e2e`:

```dockerfile
FROM mcr.microsoft.com/playwright:v1.58.2-jammy

WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .

CMD ["npm", "run", "test:e2e"]
```

## CI/CD Integration

### GitHub Actions

```yaml
name: E2E Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '20'
      - name: Install dependencies
        run: |
          cd frontend
          npm ci
          npx playwright install --with-deps chromium
      - name: Run E2E tests
        run: |
          cd frontend
          npm run test:e2e
      - name: Upload test results
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: playwright-report
          path: frontend/playwright-report/
```

## Configuration

Test configuration is in `playwright.config.ts`:

- **Base URL**: `http://localhost:3000`
- **Browser**: Chromium (Desktop Chrome)
- **Retries**: 2 (in CI), 0 (local)
- **Screenshot**: On failure only
- **Trace**: On first retry

## Test Scenarios

### Authentication Tests (`e2e/auth/login.spec.ts`)

- ✅ Redirect to login when not authenticated
- ✅ Show login page with OAuth button
- ✅ Show dashboard when authenticated
- ✅ Show user email in navbar
- ✅ Logout and clear storage

### Secret Lifecycle Tests (`e2e/secrets/`)

- **List Tests** (`list.spec.ts`) - 3 tests
  - ✅ Display secrets list with correct columns
  - ✅ Filter secrets by namespace
  - ✅ Search secrets by name

- **Create Tests** (`create.spec.ts`) - 3 tests
  - ✅ Create new secret draft successfully
  - ✅ Validate required fields
  - ✅ Require at least one key-value pair

- **Edit & Publish Tests** (`edit-publish.spec.ts`) - 3 tests
  - ✅ Edit existing draft secret
  - ✅ Publish secret to GitOps
  - ✅ Prevent publishing drifted secrets

- **Delete Tests** (`delete.spec.ts`) - 2 tests
  - ✅ Delete draft secret
  - ✅ Prevent deletion of published secrets

### Drift Detection Tests (`e2e/drift/`)

Tests for drift detection, comparison, resolution, and dashboard widget features.

## Troubleshooting

### "Target page, context or browser has been closed"

This usually means missing system libraries. Install dependencies:

```bash
npx playwright install-deps chromium
```

### Port 3000 already in use

If the dev server is already running, Playwright will reuse it. Otherwise, it starts a new instance automatically.

### Tests timing out

Increase timeout in specific tests:

```typescript
test('slow operation', async ({ page }) => {
  test.setTimeout(60000); // 60 seconds
  // ... test code
});
```

## Next Steps

1. ✅ ~~Add secret management test suite (`e2e/secrets/`)~~
2. Add audit log verification tests
3. Add sync status monitoring tests
4. Add integration tests with backend API
5. Set up visual regression testing
6. Configure parallel test execution

## Resources

- [Playwright Documentation](https://playwright.dev)
- [Playwright Best Practices](https://playwright.dev/docs/best-practices)
- [Debugging Tests](https://playwright.dev/docs/debug)
