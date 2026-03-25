# Quick Start Guide: Testing the Fixes

## 🚀 Everything is Ready!

All services are running and configured:
- ✅ Backend running on http://localhost:8080
- ✅ Frontend running on http://localhost:3000
- ✅ PostgreSQL database healthy
- ✅ FluxCD installed with SOPS Age key
- ✅ 3 test secrets synced to Kubernetes

---

## 🧪 Test 1: Edit a Published Secret (Backend Fix)

### Via Frontend (Recommended)

1. **Open the app**: http://localhost:3000
2. **Login** (if not already logged in)
3. **Navigate to Secrets** page
4. **Find a published secret** (look for green "published" badge)
5. **Click "Edit ✏️"** button
6. **Modify a value** and save
7. **Verify**:
   - Status changes to "draft" (gray badge)
   - "Re-Publish" button appears
   - No errors shown

### Via API (Alternative)

```bash
# Get your auth token first (login via UI and check browser DevTools > Application > Cookies)
TOKEN="your-token-here"

# Get namespace ID
NAMESPACE_ID=$(curl -s http://localhost:8080/api/v1/namespaces \
  -H "Authorization: Bearer $TOKEN" | jq -r '.[0].id')

# Create a test secret
curl -X POST http://localhost:8080/api/v1/namespaces/$NAMESPACE_ID/secrets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-edit-published",
    "data": {
      "key1": "value1",
      "key2": "value2"
    }
  }'

# Publish it
curl -X POST http://localhost:8080/api/v1/namespaces/$NAMESPACE_ID/secrets/test-edit-published/publish \
  -H "Authorization: Bearer $TOKEN"

# Now edit it (this should succeed and revert to draft)
curl -X PUT http://localhost:8080/api/v1/namespaces/$NAMESPACE_ID/secrets/test-edit-published \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "key1": "updated-value",
      "key2": "value2"
    }
  }'

# Expected: 200 OK with status: "draft"
```

---

## 🧪 Test 2: Frontend Buttons (UI Fix)

### Check Draft Secret
1. Go to http://localhost:3000/secrets
2. Find a secret with **gray "draft" badge**
3. **Verify you see**:
   - [Edit] button (blue)
   - [Publish] button (green)

### Check Published Secret
1. Find a secret with **green "published" badge**
2. **Verify you see**:
   - [Edit ✏️] button (blue with pencil emoji)
   - [Re-Publish] button (green)
3. **Hover over Edit button**
   - Tooltip should say: "Editing will create a new draft version"

### Check Drifted Secret (if you have one)
1. Find a secret with **yellow "drifted" badge**
2. **Verify you see**:
   - [Edit ✏️] button (blue with pencil emoji)
   - [Re-Publish] button (green)

---

## 🧪 Test 3: SOPS Decryption & K8s Sync

### Verify Existing Secrets
```bash
# List all secrets
kubectl get secrets -n development

# Expected output:
# NAME                TYPE     DATA   AGE
# db-credentials      Opaque   6      Xm
# test-secret-e2e     Opaque   3      Xm
# test-secret-final   Opaque   3      Xm
```

### Check Secret Content
```bash
# Get db-credentials secret
kubectl -n development get secret db-credentials -o jsonpath='{.data.DB_HOST}' | base64 -d
# Expected: postgres.dev.svc.cluster.local

kubectl -n development get secret db-credentials -o jsonpath='{.data.DB_USER}' | base64 -d
# Expected: appuser

kubectl -n development get secret db-credentials -o jsonpath='{.data.DB_PASSWORD}' | base64 -d
# Expected: change-me-in-production
```

### Test Manual Sync Script
```bash
# Run the sync script
./scripts/sync-secrets-to-k8s.sh

# Expected output:
# 🔄 Syncing secrets from /home/nk/secret-manager/dev-data/secrets-repo to Kubernetes...
# 📂 Namespace: development
#   📦 Syncing development/db-credentials
#   📦 Syncing development/test-secret-e2e
#   📦 Syncing development/test-secret-final
# 📂 Namespace: production
# 📂 Namespace: staging
# ✅ Sync complete!
```

---

## 🧪 Test 4: Complete Workflow (End-to-End)

### Create → Publish → Edit → Re-Publish

1. **Create a new secret**:
   - Go to http://localhost:3000/secrets
   - Click "+ Create Secret"
   - Name: `workflow-test`
   - Add keys: `username` = `admin`, `password` = `secret123`
   - Click "Create"
   - **Verify**: Status is "draft"

2. **Publish the secret**:
   - Click "Publish" button
   - Confirm the dialog
   - **Verify**: 
     - Status changes to "published" (green badge)
     - Button changes to "Re-Publish"
     - Check Git repo: `ls dev-data/secrets-repo/namespaces/development/secrets/workflow-test.yaml`

3. **Edit the published secret**:
   - Click "Edit ✏️" button (should have pencil emoji)
   - Change password to `newsecret456`
   - Click "Save"
   - **Verify**:
     - Status changes back to "draft" (gray badge)
     - Button changes to "Publish"

4. **Re-publish the updated secret**:
   - Click "Publish" button
   - **Verify**:
     - Status is "published" again
     - Button shows "Re-Publish"
     - Git repo has updated file

5. **Sync to Kubernetes**:
   ```bash
   ./scripts/sync-secrets-to-k8s.sh
   kubectl -n development get secret workflow-test -o yaml
   # Should see updated password (base64 encoded)
   ```

---

## 🧪 Test 5: FluxCD Health Check

```bash
# Check Flux is installed and healthy
flux check

# Expected:
# ✔ Kubernetes 1.34.1 >=1.33.0-0
# ✔ distribution: flux-v2.8.3
# ✔ helm-controller: deployment ready
# ✔ kustomize-controller: deployment ready
# ✔ notification-controller: deployment ready
# ✔ source-controller: deployment ready
# ✔ all checks passed

# Check SOPS Age key is configured
kubectl -n flux-system get secret sops-age

# Expected:
# NAME       TYPE     DATA   AGE
# sops-age   Opaque   1      XXm
```

---

## 🐛 Troubleshooting

### Backend Error: "Cannot update secret with status 'published'"

This means the backend code change didn't apply. Restart backend:
```bash
docker-compose restart backend
docker-compose logs -f backend
```

### Frontend Not Showing New Buttons

Clear browser cache or hard refresh:
- Chrome/Firefox: `Ctrl + Shift + R`
- Or open DevTools > Network tab > check "Disable cache"

### SOPS Decryption Fails

Verify Age key exists:
```bash
ls -la /home/nk/secret-manager/dev-data/age-keys/keys.txt
```

Test manual decryption:
```bash
export SOPS_AGE_KEY_FILE=/home/nk/secret-manager/dev-data/age-keys/keys.txt
sops -d dev-data/secrets-repo/namespaces/development/secrets/db-credentials.yaml
```

### Secrets Not Appearing in K8s

Run manual sync:
```bash
./scripts/sync-secrets-to-k8s.sh
```

Check if namespace exists:
```bash
kubectl get namespace development
```

---

## 📊 Success Criteria Checklist

- [ ] Backend allows editing published secrets (no 409 error)
- [ ] Status automatically changes to "draft" when editing published secret
- [ ] Frontend shows "Edit ✏️" button for published secrets
- [ ] Frontend shows "Re-Publish" button for published/drifted secrets
- [ ] Tooltip appears when hovering Edit button on published secrets
- [ ] FluxCD is installed and healthy (`flux check` passes)
- [ ] SOPS Age key is configured in flux-system namespace
- [ ] Manual SOPS decryption works
- [ ] Manual sync script successfully syncs secrets to K8s
- [ ] All 3 services (backend, frontend, postgres) are running

---

## 🎉 All Tests Pass? You're Done!

If all tests pass, the implementation is complete and working. 

**What's been fixed**:
- ✅ Backend blocks editing published secrets → Now allows with auto-draft
- ✅ Frontend only shows Publish for drafts → Now shows Re-Publish for published
- ✅ No FluxCD setup → Now installed with SOPS decryption ready

**What's next**:
- Choose Git remote option for automated FluxCD sync (see FLUXCD_SETUP.md)
- Or continue using manual sync script for local development

---

## 📚 Additional Documentation

- `IMPLEMENTATION_SUMMARY.md` - Detailed implementation notes
- `FLUXCD_SETUP.md` - FluxCD configuration status and options
- `flux-config/README.md` - FluxCD setup instructions
- `scripts/sync-secrets-to-k8s.sh` - Manual sync script
- `scripts/setup-flux-gitops.sh` - Automated Flux setup (requires Git remote)

---

**Happy testing! 🚀**
