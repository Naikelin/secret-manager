# Secret Manager - Roadmap & Next Steps

**Last Updated:** March 25, 2026  
**Current Status:** Phase 17/20 Complete (85%)  
**Project:** GitOps-based Kubernetes Secret Management System

---

## 🎯 Project Overview

Production-grade secret management system with:
- Draft → Validate → Publish workflow (PostgreSQL staging → SOPS + Git → FluxCD → K8s)
- Azure AD group-based RBAC (multi-tenancy)
- Drift detection & resolution
- Audit trail & compliance
- Web UI for secret management

---

## ✅ Completed Phases (1-17)

### **Phase 1-11: Backend Foundation** ✅
- PostgreSQL database (8 models)
- Mock OAuth2 authentication
- RBAC with permission evaluation
- Git client with retry logic
- SOPS Age encryption
- FluxCD integration
- Kubernetes client
- Drift detection engine

### **Phase 12-13: Advanced Features** ✅
- Drift resolution (sync from Git, import to Git, mark resolved)
- Audit trail API (query + CSV export)

### **Phase 14-16: Frontend Implementation** ✅
- Secret Editor UI (CRUD with namespace selector)
- Drift Viewer UI (detection + resolution actions)
- Audit Log Viewer UI (filtering + export)

### **Phase 17: Production Polish** ✅
- Sync Status Dashboard (Git vs Flux SHA comparison)
- Fixed edit workflow (published secrets → draft → re-publish)
- FluxCD installed in local K8s
- SOPS Age decryption configured
- Professional UI (gradients, shadows, hover effects)
- Tailwind CSS v3 setup

**Git Commit:** `eff0931` (March 25, 2026)

---

## 📋 Remaining Phases (18-20+)

### **Phase 18: Background Drift Detection & Alerts** 🆕
**Priority:** HIGH  
**Estimated Time:** 60-90 minutes  

**Current State:**
- ❌ Drift detection is **manual only** (user clicks "Check for Drift")
- ❌ No automatic monitoring
- ❌ No alerts/notifications

**Implementation Plan:**

#### **18.1: Background Drift Detection Job**
- Add ticker-based job in `main.go` (runs every 5 minutes)
- New endpoint: `POST /api/v1/drift/check-all` (checks all namespaces)
- Update secret statuses to "drifted" automatically
- Log drift events to audit trail

**Files to modify:**
- `backend/cmd/server/main.go` - Add background ticker
- `backend/internal/api/drift.go` - Add `CheckAllNamespaces()` handler
- `backend/internal/drift/detector.go` - Add batch detection method

#### **18.2: Dashboard Drift Widget**
- Add drift summary card to `/dashboard`
- Show count of unresolved drift events
- Link directly to `/drift` page
- Color-coded by severity (green/yellow/red)

**Files to create:**
- `frontend/components/DriftWidget.tsx`

**Files to modify:**
- `frontend/app/dashboard/page.tsx`

#### **18.3: Navbar Drift Badge**
- Add red badge to navbar when drift detected
- Dropdown showing recent drift events
- "View All" link to `/drift`

**Files to modify:**
- `frontend/app/layout.tsx` or create `frontend/components/Navbar.tsx`

#### **18.4: Optional Webhooks**
- Configurable webhook URL (env var: `DRIFT_WEBHOOK_URL`)
- Send POST to webhook when drift detected
- Payload: namespace, secret name, drift type, timestamp
- Use case: Slack/Discord/PagerDuty integration

**Files to create:**
- `backend/internal/notifications/webhook.go`

**Success Criteria:**
- [ ] Drift checked automatically every 5 minutes
- [ ] Dashboard shows drift count widget
- [ ] Navbar badge appears when drift exists
- [ ] (Optional) Webhook fires on drift detection

---

### **Phase 19: E2E Tests & Integration Testing** 
**Priority:** HIGH  
**Estimated Time:** 60-90 minutes

**Scope:**
- Integration tests for full workflows
- API endpoint coverage
- RBAC permission checks
- Database transaction tests

**Test Categories:**

#### **19.1: Draft → Publish Workflow**
```go
TestPublishWorkflow:
- Create draft secret
- Validate SOPS encryption
- Verify Git commit + push
- Confirm status = "published"
- Check audit log entry
```

#### **19.2: Edit → Re-Publish Workflow**
```go
TestEditPublishedSecret:
- Edit published secret
- Verify status reverts to "draft"
- Re-publish
- Verify Git commit updated
- Check audit log entries
```

#### **19.3: Drift Detection → Resolution**
```go
TestDriftDetection:
- Publish secret to K8s
- Manually modify K8s secret
- Run drift check
- Verify drift event created
- Test sync-from-git resolution
- Verify drift resolved
```

#### **19.4: RBAC Permission Tests**
```go
TestRBACPermissions:
- Viewer: read-only access
- Editor: create/edit drafts
- Publisher: publish to Git
- Admin: all operations
```

#### **19.5: Audit Log Tests**
```go
TestAuditTrail:
- Verify all actions logged
- Test filtering by user/action/date
- Verify CSV export
```

**Files to create:**
- `backend/internal/api/integration_test.go`
- `backend/internal/api/workflow_test.go`
- `backend/internal/api/rbac_test.go`

**Tools:**
- Go `testing` package
- `testcontainers-go` (PostgreSQL)
- `httptest` (API testing)

**Success Criteria:**
- [ ] 80%+ code coverage on critical paths
- [ ] All workflows tested end-to-end
- [ ] RBAC enforced correctly
- [ ] Audit logs capture all actions

---

### **Phase 20: API Documentation & Deployment Guides**
**Priority:** MEDIUM  
**Estimated Time:** 45-60 minutes

**Scope:**
- OpenAPI/Swagger documentation
- Production deployment guides
- Critical bug fixes

#### **20.1: OpenAPI Spec Generation**
- Add `swaggo/swag` annotations to handlers
- Generate `docs/swagger.json`
- Deploy Swagger UI at `/api/docs`

**Files to modify:**
- All `backend/internal/api/*.go` handlers (add comments)
- `backend/cmd/server/main.go` (serve Swagger UI)

**Tools:**
- `github.com/swaggo/swag`
- `github.com/swaggo/http-swagger`

#### **20.2: Fix SOPS Dockerfile Installation**
**Current Issue:** SOPS installed manually in running container (not persistent)

**Fix:**
```dockerfile
# backend/Dockerfile.dev
RUN apk add --no-cache curl && \
    curl -LO https://github.com/getsops/sops/releases/download/v3.8.1/sops-v3.8.1.linux.amd64 && \
    mv sops-v3.8.1.linux.amd64 /usr/bin/sops && \
    chmod +x /usr/bin/sops
```

#### **20.3: Production Deployment Guide**
Create `docs/DEPLOYMENT.md`:
- Kubernetes manifests (Deployment, Service, Ingress)
- Helm chart (optional)
- Azure AD OAuth2 setup
- Azure Key Vault KMS configuration
- PostgreSQL (managed instance)
- FluxCD GitRepository setup
- Monitoring & alerting (Prometheus/Grafana)

#### **20.4: FluxCD Automation Guide**
Create `docs/FLUXCD.md`:
- **Option A:** GitHub/GitLab repo setup
- **Option B:** Local git daemon setup
- **Option C:** Manual sync workflow
- Troubleshooting common issues

**Success Criteria:**
- [ ] Swagger UI accessible at `/api/docs`
- [ ] All 25+ endpoints documented
- [ ] SOPS Dockerfile fixed (rebuild works)
- [ ] Production deployment guide complete
- [ ] FluxCD setup documented

---

## 🔮 Future Enhancements (Post-MVP)

### **Phase 21: Secret Rotation**
- Automatic secret rotation policies
- Integration with cert-manager (TLS certs)
- Webhook to notify dependent services

### **Phase 22: Secret Templates**
- Predefined templates (DB credentials, API keys, TLS certs)
- Variable interpolation
- Environment-based overrides

### **Phase 23: Multi-Cluster Support**
- Manage secrets across multiple K8s clusters
- Cluster groups and federation
- Cross-cluster drift detection

### **Phase 24: Advanced RBAC**
- Field-level permissions (hide sensitive keys)
- Time-based access (temporary access grants)
- Approval workflows for publish

### **Phase 25: Compliance & Security**
- Secret scanning (detect leaked secrets)
- Encryption at rest for PostgreSQL
- SOC2/ISO27001 audit reports
- Integration with Vault/AWS Secrets Manager

---

## 📊 Current System Stats

**Architecture:**
- Backend: Go 1.26 (Chi router, GORM)
- Frontend: Next.js 15 (React 19, Tailwind v3)
- Database: PostgreSQL 15
- Encryption: SOPS + Age keys
- GitOps: FluxCD v2
- Deployment: Docker Compose (dev), Kubernetes (prod)

**Code Stats:**
- Backend handlers: 12 files
- Frontend pages: 8 pages
- API endpoints: 25+
- Database models: 8 tables
- Lines of code: ~8,000+

**Docker Images:**
- `secretmanager-backend`: Go 1.26 Alpine + Air (hot reload)
- `secretmanager-frontend`: Node 25 Alpine + Next.js dev
- `secretmanager-postgres`: PostgreSQL 15

---

## 🚀 Next Session Plan (March 26, 2026)

### **Priority Order:**
1. **Phase 18** - Background Drift Detection (MUST HAVE)
2. **Phase 19** - E2E Tests (QUALITY)
3. **Phase 20** - Documentation + Dockerfile fix (POLISH)

### **Recommended Approach:**

#### **Morning Session (2-3 hours):**
1. Implement Phase 18.1-18.3 (background job + UI widgets)
2. Test drift detection in local K8s
3. Commit + deploy

#### **Afternoon Session (2-3 hours):**
1. Implement Phase 19 (E2E tests)
2. Fix any bugs discovered during testing
3. Commit

#### **Final Polish (1 hour):**
1. Phase 20.1-20.2 (Swagger + Dockerfile)
2. Write deployment guides
3. Final commit + tag v1.0.0

---

## 🐛 Known Issues

### **Critical:**
- ✅ FIXED: Published secrets couldn't be edited
- ✅ FIXED: Published secrets couldn't be re-published
- ✅ FIXED: Tailwind CSS not loading

### **High:**
- ⏳ SOPS installation not persistent in Dockerfile (manual workaround active)
- ⏳ FluxCD automation blocked on Git remote configuration

### **Medium:**
- ⏳ No automatic drift detection (manual only)
- ⏳ Audit log filtering for non-admin users not implemented (TODO comment exists)

### **Low:**
- ⏳ Drift detection base64 encoding issues in mocks (non-blocking)

---

## 📚 Documentation Files

- `README.md` - Project overview
- `ROADMAP.md` - **This file** (phases & next steps)
- `FLUXCD_SETUP.md` - FluxCD status & options
- `IMPLEMENTATION_SUMMARY.md` - Technical implementation details
- `QUICKSTART_TESTING.md` - Testing guide
- `flux-config/README.md` - Flux manifest documentation

---

## 🎯 Success Criteria for v1.0

- [x] Phase 1-17 complete
- [ ] Phase 18 complete (drift alerts)
- [ ] Phase 19 complete (E2E tests)
- [ ] Phase 20 complete (docs + polish)
- [ ] All critical bugs resolved
- [ ] 80%+ test coverage on core workflows
- [ ] Production deployment guide ready
- [ ] FluxCD working (any option)

---

## 📞 Handoff Notes

**For Next Developer:**
1. Review `QUICKSTART_TESTING.md` for manual testing procedures
2. Run `docker-compose up` to start all services
3. Backend: http://localhost:8080 (API)
4. Frontend: http://localhost:3000 (UI)
5. PostgreSQL: localhost:5432 (dev:devpass)
6. FluxCD: Manual sync script at `scripts/sync-secrets-to-k8s.sh`
7. Check `FLUXCD_SETUP.md` for Git remote options

**Environment Variables:**
- Backend: `backend/.env` (DB, JWT, Git, SOPS paths)
- Frontend: `frontend/.env` (API URL)
- Root: `.env` (project-level vars)

**Testing:**
- Login: http://localhost:3000/auth/login
- Default user: admin@example.com (mock OAuth)
- Create secrets in `development` namespace
- Publish → Check `dev-data/secrets-repo/namespaces/development/secrets/`

---

**Ready to continue tomorrow! 🚀**
