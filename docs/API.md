# Secret Manager API Documentation

## Overview

The Secret Manager API provides comprehensive secret management with GitOps integration, drift detection, and audit logging capabilities.

**Base URL**: `http://localhost:8080/api/v1`

## Interactive Documentation

**Swagger UI**: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)

The Swagger UI provides:
- Interactive API explorer with "Try it out" functionality
- Complete request/response schemas
- Authentication testing
- Real-time API testing

## Authentication

All protected endpoints require JWT Bearer token authentication.

### Authentication Flow

1. **Login**: `POST /api/v1/auth/login`
   - Send user email
   - Receive OAuth2 redirect URL
   
2. **Callback**: `GET /api/v1/auth/callback`
   - OAuth2 provider redirects here with authorization code
   - Receive JWT token and user info

3. **Use Token**: Include in all subsequent requests
   ```
   Authorization: Bearer <your-jwt-token>
   ```

### Development Mode

For local development, use the mock authentication endpoint:

```bash
# Direct mock login (bypasses OAuth2)
curl http://localhost:8080/api/v1/auth/mock-callback?email=admin@example.com
```

This redirects to frontend with a JWT token for the specified user.

## API Endpoints

### Authentication

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| POST | `/auth/login` | Initiate OAuth2 login | No |
| GET | `/auth/callback` | OAuth2 callback handler | No |
| POST | `/auth/logout` | Logout (client-side) | No |
| GET | `/auth/mock-callback` | Mock login (dev only) | No |

### Clusters

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| GET | `/clusters` | List all clusters | Yes (read) |
| POST | `/clusters` | Create new cluster | Yes (admin) |
| GET | `/clusters/{id}` | Get cluster by ID | Yes (read) |
| DELETE | `/clusters/{id}` | Delete cluster | Yes (admin) |
| GET | `/clusters/{id}/health` | Check cluster health | Yes (read) |
| GET | `/clusters/{id}/namespaces` | List namespaces in cluster | Yes (read) |

**Note**: Cluster endpoints enable management of multiple Kubernetes clusters. Each cluster requires a kubeconfig file in `K8S_KUBECONFIGS_DIR`.

### Namespaces

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| GET | `/namespaces` | List accessible namespaces (optionally filtered by cluster) | Yes |

### Secrets

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| GET | `/namespaces/{namespace}/secrets` | List secrets in namespace | Yes (read) |
| POST | `/namespaces/{namespace}/secrets` | Create new secret | Yes (write) |
| GET | `/namespaces/{namespace}/secrets/{name}` | Get secret by name | Yes (read) |
| PUT | `/namespaces/{namespace}/secrets/{name}` | Update secret | Yes (write) |
| DELETE | `/namespaces/{namespace}/secrets/{name}` | Delete secret | Yes (delete/admin) |
| POST | `/namespaces/{namespace}/secrets/{name}/publish` | Publish secret to Git | Yes (publish/editor) |
| POST | `/namespaces/{namespace}/secrets/{name}/unpublish` | Unpublish secret from Git | Yes (delete/admin) |

### Kubernetes Secrets (Read-Only)

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| GET | `/namespaces/{namespace}/k8s-secrets` | List K8s secrets | Yes (read) |
| GET | `/namespaces/{namespace}/k8s-secrets/{name}` | Get K8s secret metadata | Yes (read) |

**Note**: These endpoints return metadata only (keys, timestamps, labels) - not actual secret values.

### Drift Detection

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| POST | `/namespaces/{namespace}/drift-check` | Trigger drift detection | Yes (read) |
| GET | `/namespaces/{namespace}/drift-events` | List drift events | Yes (read) |
| POST | `/drift/check-all` | Check all namespaces (admin) | Yes (admin) |
| GET | `/drift-events/{drift_id}/compare` | Get Git vs K8s comparison | Yes (read) |

### Drift Resolution

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| POST | `/drift-events/{drift_id}/sync-from-git` | Sync Git → K8s | Yes (write) |
| POST | `/drift-events/{drift_id}/import-to-git` | Import K8s → Git | Yes (publish) |
| POST | `/drift-events/{drift_id}/mark-resolved` | Mark as resolved | Yes (write) |

### FluxCD Sync Status

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| GET | `/namespaces/{namespace}/sync-status` | Get GitOps sync status | Yes (read) |

### Audit Logs

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| GET | `/audit-logs` | List audit logs (filtered) | Yes |
| GET | `/audit-logs/export` | Export audit logs as CSV | Yes |

### Health Check

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| GET | `/health` | Health check | No |

## Example Workflows

### Register a New Cluster

```bash
# 1. Login and get token
TOKEN="your-jwt-token-here"

# 2. Create cluster (admin only)
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  http://localhost:8080/api/v1/clusters \
  -d '{
    "name": "devops",
    "environment": "production",
    "kubeconfig_ref": "devops.yaml"
  }'

# 3. Verify cluster health
CLUSTER_ID="<cluster-uuid>"
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/clusters/$CLUSTER_ID/health

# 4. List namespaces in cluster
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/clusters/$CLUSTER_ID/namespaces
```

### Create and Publish a Secret

```bash
# 1. Login and get token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com"}' | jq -r '.redirect_url')

# 2. List namespaces
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/namespaces

# 3. Create secret
NAMESPACE_ID="<namespace-uuid>"
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  http://localhost:8080/api/v1/namespaces/$NAMESPACE_ID/secrets \
  -d '{
    "name": "my-secret",
    "data": {
      "username": "admin",
      "password": "secure-password"
    }
  }'

# 4. Publish to Git
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/namespaces/$NAMESPACE_ID/secrets/my-secret/publish
```

### Check for Drift

```bash
# Trigger drift detection
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/namespaces/$NAMESPACE_ID/drift-check

# List drift events
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/namespaces/$NAMESPACE_ID/drift-events?status=active"
```

### Resolve Drift

```bash
DRIFT_ID="<drift-event-uuid>"

# Option 1: Sync from Git (Git is source of truth)
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/drift-events/$DRIFT_ID/sync-from-git

# Option 2: Import to Git (K8s is source of truth)
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/drift-events/$DRIFT_ID/import-to-git

# Option 3: Mark as resolved (manual resolution)
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/drift-events/$DRIFT_ID/mark-resolved
```

### Export Audit Logs

```bash
# Export all logs to CSV
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs/export" > audit-logs.csv

# Filter by date range
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs/export?start_date=2026-01-01T00:00:00Z&end_date=2026-03-31T23:59:59Z" \
  > audit-logs-q1.csv

# Filter by namespace
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs/export?namespace_id=$NAMESPACE_ID" \
  > namespace-audit.csv
```

## Request/Response Examples

### Create Secret Request

```json
{
  "name": "database-credentials",
  "data": {
    "DB_HOST": "postgres.example.com",
    "DB_PORT": "5432",
    "DB_USER": "app_user",
    "DB_PASSWORD": "super_secret_password",
    "DB_NAME": "production"
  }
}
```

### Secret Response

```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "secret_name": "database-credentials",
  "namespace_id": "789e4567-e89b-12d3-a456-426614174000",
  "data": {
    "DB_HOST": "postgres.example.com",
    "DB_PORT": "5432",
    "DB_USER": "app_user",
    "DB_PASSWORD": "super_secret_password",
    "DB_NAME": "production"
  },
  "status": "draft",
  "edited_by": "456e4567-e89b-12d3-a456-426614174000",
  "edited_at": "2026-03-26T00:00:00Z",
  "created_at": "2026-03-26T00:00:00Z",
  "updated_at": "2026-03-26T00:00:00Z"
}
```

### Drift Event Response

```json
{
  "namespace": "production",
  "checked": 15,
  "drifted": 2,
  "events": [
    {
      "id": "321e4567-e89b-12d3-a456-426614174000",
      "secret_name": "app-config",
      "detected_at": "2026-03-26T00:00:00Z",
      "diff_summary": "2 differences detected"
    }
  ]
}
```

## Error Responses

All error responses follow this format:

```json
{
  "error": "Error message description"
}
```

### Common HTTP Status Codes

- **200 OK**: Successful request
- **201 Created**: Resource created successfully
- **204 No Content**: Successful deletion
- **400 Bad Request**: Invalid request parameters
- **401 Unauthorized**: Missing or invalid authentication
- **403 Forbidden**: Insufficient permissions
- **404 Not Found**: Resource not found
- **409 Conflict**: Resource conflict (e.g., duplicate name, unresolved drift)
- **500 Internal Server Error**: Server error
- **503 Service Unavailable**: External service unavailable (Git, K8s, FluxCD)

## Permissions

The API uses role-based access control (RBAC) with the following permission levels:

| Permission | Description | Allowed Actions |
|------------|-------------|-----------------|
| **read** | Read-only access | List, get secrets and drift events |
| **write** | Create and modify | Create, update secrets; trigger drift checks |
| **publish** | Publish to Git | Publish secrets to Git (editors) |
| **delete** | Delete resources | Delete secrets, unpublish (admins only) |

Permissions are assigned to groups, and users inherit permissions from their group memberships.

## Rate Limiting

Currently, no rate limiting is enforced. For production deployments, consider implementing rate limiting at the API gateway or load balancer level.

## Development

### Regenerate Swagger Documentation

After making changes to API handlers or adding new endpoints:

```bash
cd backend
swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal
```

### Environment Variables

Key configuration variables:

- `JWT_SECRET`: Secret key for JWT token signing
- `GIT_REPO_URL`: Git repository URL for GitOps
- `GIT_REPO_PATH`: Local path for Git repository
- `SOPS_ENABLED`: Enable SOPS encryption (true/false)
- `K8S_KUBECONFIGS_DIR`: Directory containing cluster kubeconfig files (multi-cluster)
- `DRIFT_CHECK_INTERVAL`: Interval for automatic drift checks (e.g., "5m")
- `DRIFT_WEBHOOK_URL`: Webhook URL for drift notifications

**Multi-Cluster Configuration**: The backend reads kubeconfig files from `K8S_KUBECONFIGS_DIR` and matches them to clusters registered in the database via the `kubeconfig_ref` field. Kubeconfigs should be mounted as Kubernetes Secrets in production deployments.

## Support

For issues, feature requests, or questions:
- GitHub Issues: https://github.com/yourorg/secret-manager/issues
- Email: support@example.com

## License

MIT License - See LICENSE file for details
