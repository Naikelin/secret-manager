# Secret Manager Documentation

Welcome to the Secret Manager documentation. This directory contains all project documentation organized by topic.

## 📚 Documentation Index

### Setup & Deployment
- [**Quickstart Guide**](setup/quickstart.md) - Get started quickly with local development and testing
- [**FluxCD Setup**](setup/fluxcd-setup.md) - GitOps deployment with FluxCD on Kubernetes
- [**Deployment Options**](../README.md#deployment) - Overview of different deployment methods

### Architecture & Design
- [**Implementation Summary**](architecture/implementation-summary.md) - Technical architecture and design decisions
- [**Project Roadmap**](roadmap.md) - Feature roadmap and development plans

### Testing & Validation
- [**Drift Validation Report**](testing/drift-validation.md) - E2E drift detection testing results
- [**Browser Flow Testing**](testing/test-browser-flow.md) - Manual browser testing guide

## 🚀 Quick Links

### For Developers
1. Start with the [Quickstart Guide](setup/quickstart.md)
2. Review [Architecture](architecture/implementation-summary.md)
3. Check [Roadmap](roadmap.md) for planned features

### For DevOps
1. See [FluxCD Setup](setup/fluxcd-setup.md) for Kubernetes deployment
2. Review `../terraform/` for infrastructure as code
3. Check `../k8s/` for Kubernetes manifests

### For QA/Testing
1. [Drift Validation](testing/drift-validation.md) - E2E test results
2. [Browser Testing](testing/test-browser-flow.md) - Manual test procedures
3. `../scripts/dev/` - Development and testing scripts

## 📂 Repository Structure

```
secret-manager/
├── docs/              # 📖 All documentation (you are here)
├── backend/           # 🔧 Go backend API
├── frontend/          # 🎨 Next.js frontend
├── scripts/           # 🔨 Helper scripts
│   └── dev/          #    Development & testing scripts
├── k8s/              # ☸️  Kubernetes manifests
├── terraform/        # 🏗️  Infrastructure as code
├── helm/             # ⎈  Helm charts
├── flux-config/      # 🔄 FluxCD GitOps configuration
└── docker-compose*.yml # 🐳 Docker compose configs
```

## 🛠️ Common Tasks

### Local Development
```bash
# Start all services
docker compose up -d

# Backend only
cd backend && make dev

# Frontend only
cd frontend && npm run dev

# Run E2E tests
docker compose -f docker-compose.e2e.yml up
```

### Testing
```bash
# API tests
./scripts/dev/test-api.sh

# Auth flow test
./scripts/dev/test-auth-flow.sh

# E2E tests
cd frontend && npm run test:e2e
```

## 📝 Contributing

When adding new documentation:
1. Place it in the appropriate subdirectory
2. Update this README.md index
3. Use clear, descriptive filenames (kebab-case)
4. Include code examples where relevant

## 🔗 External Resources

- [Project Repository](https://github.com/Naikelin/secret-manager)
- [Issue Tracker](https://github.com/Naikelin/secret-manager/issues)
- Main README: [`../README.md`](../README.md)
