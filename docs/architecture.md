# Secret Manager Architecture

## Overview

Secret Manager is a GitOps-based Kubernetes secret management system with **multi-cluster support**. It enables centralized management of secrets across multiple production Kubernetes clusters from a single backend instance.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Frontend (Next.js)                        │
│                     http://localhost:3000                           │
└────────────────────────────────┬────────────────────────────────────┘
                                 │ REST API (JWT Auth)
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       Backend API (Go + Chi)                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐ │
│  │ API Handlers │  │ RBAC Authz   │  │  ClientManager (K8s)     │ │
│  │              │  │              │  │  - Lazy init per cluster │ │
│  └──────────────┘  └──────────────┘  │  - Thread-safe cache     │ │
│         │                 │           │  - Health monitoring     │ │
│         ▼                 ▼           └──────────────────────────┘ │
│  ┌──────────────────────────────┐              │                   │
│  │   PostgreSQL (GORM Models)   │              │                   │
│  │   - Users, Groups, Clusters  │              │                   │
│  │   - Namespaces, Secrets      │              │                   │
│  │   - Audit Logs, Drift Events │              │                   │
│  └──────────────────────────────┘              │                   │
└────────┬─────────────────────────────────────┬─┘
         │ Git Sync (SOPS)                     │ K8s API Calls
         ▼                                     ▼
┌──────────────────────────────┐   ┌───────────────────────────────┐
│   Git Repository (SOPS)      │   │   Multiple K8s Clusters       │
│                              │   │                               │
│  clusters/                   │   │  ┌───────────────────────┐   │
│    devops/                   │   │  │  Cluster: devops      │   │
│      namespaces/             │   │  │  FluxCD → Secrets     │   │
│        development/          │   │  └───────────────────────┘   │
│          secrets/            │   │                               │
│            db-credentials.ya ├───┼─▶┌───────────────────────┐   │
│    integraciones-dev/        │   │  │  Cluster: integr-dev  │   │
│      namespaces/...          │   │  │  FluxCD → Secrets     │   │
│    kyndryl-dev/              │   │  └───────────────────────┘   │
│      namespaces/...          │   │                               │
└──────────────────────────────┘   │  ┌───────────────────────┐   │
                                   │  │  Cluster: kyndryl-pro │   │
                                   │  │  FluxCD → Secrets     │   │
                                   │  └───────────────────────┘   │
                                   └───────────────────────────────┘
```

## Multi-Cluster Architecture

### Design Decisions

Secret Manager uses a **cluster-first Git structure** to manage secrets across multiple environments:

| Component | Design Choice | Rationale |
|-----------|---------------|-----------|
| **Git Structure** | Single repo, cluster-first paths | Simplifies access control, atomic multi-cluster updates |
| **Path Format** | `clusters/{cluster}/namespaces/{ns}/secrets/` | Clear hierarchy, avoids cross-cluster conflicts |
| **Client Pool** | ClientManager with lazy initialization | Unreachable clusters don't block backend startup |
| **Kubeconfig Storage** | Filesystem with K8s Secret mounts | Secure, scalable, supports rotation |
| **Health Monitoring** | Active tracking with periodic checks | Proactive alerting for cluster connectivity issues |

### ClientManager Pattern

The `ClientManager` maintains a **thread-safe pool of Kubernetes clients**, one per cluster:

```go
type ClientManager interface {
    GetClient(clusterID uuid.UUID) (kubernetes.Interface, error)
    RemoveClient(clusterID uuid.UUID) error
    ListClusters() ([]models.Cluster, error)
}
```

**Key Features**:
- **Lazy Initialization**: Clients are created on first access, not at startup
- **Double-Checked Locking**: High-performance concurrent access (27.79 ns/op cached)
- **Graceful Degradation**: Unreachable clusters are marked `is_healthy=false`, don't block operations
- **Security Validation**: Rejects world-readable kubeconfig files (must be 0600/0400)

**Performance**:
- Cached client access: **27.79 ns/op** (357x faster than 10ms SLA)
- Read-heavy workload: **6083 ns/op** (100 concurrent readers)
- Tested with 100+ concurrent calls, zero data races

### Cluster-First Git Structure

Secrets are organized by cluster, then namespace:

```
clusters/
├── devops/
│   └── namespaces/
│       ├── development/
│       │   └── secrets/
│       │       ├── db-credentials.yaml (SOPS encrypted)
│       │       └── api-keys.yaml
│       └── production/
│           └── secrets/
├── integraciones-dev/
│   └── namespaces/
│       └── default/
│           └── secrets/
└── kyndryl-pro/
    └── namespaces/
        └── production/
            └── secrets/
```

**Benefits**:
- **Isolation**: Each cluster has its own directory tree
- **Clarity**: Path explicitly shows cluster → namespace → secret hierarchy
- **FluxCD Integration**: Each cluster's FluxCD instance watches its own subtree
- **Access Control**: Git branch protection can enforce cluster-specific approvals

### Database Model

The multi-cluster data model uses a foreign key relationship:

```
┌─────────────────┐
│    Cluster      │
├─────────────────┤
│ id (PK)         │◀─┐
│ name            │  │
│ environment     │  │
│ kubeconfig_ref  │  │ Foreign Key
│ is_healthy      │  │
└─────────────────┘  │
                     │
┌─────────────────┐  │
│   Namespace     │  │
├─────────────────┤  │
│ id (PK)         │  │
│ cluster_id (FK) │──┘
│ name            │
└─────────────────┘
         │
         │ FK
         ▼
┌─────────────────┐
│    Secret       │
├─────────────────┤
│ namespace_id    │
│ secret_name     │
│ data (JSONB)    │
│ status          │
└─────────────────┘
```

**Migration Path**:
- Phase 1: Add `Cluster` table with CASCADE delete
- Phase 2: Add `cluster_id` FK to `Namespace` (nullable initially)
- Phase 3: Backfill cluster records and FK references
- Phase 4: Drop deprecated `cluster` TEXT column (blue-green deployment complete)

### Kubeconfig Management

Kubeconfig files are stored on the filesystem and referenced by the `Cluster` model:

```bash
# Environment variable
K8S_KUBECONFIGS_DIR=/etc/kubeconfigs

# Directory structure
/etc/kubeconfigs/
├── devops.yaml          # Referenced by Cluster.kubeconfig_ref
├── integraciones-dev.yaml
└── kyndryl-pro.yaml
```

**Security Measures**:
- File permissions must be **0600** or **0400** (owner read-only)
- World-readable configs are rejected at initialization
- Mounted as Kubernetes Secrets in production deployments
- Database only stores the filename reference, not the content

**Production Deployment**:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cluster-kubeconfigs
  namespace: secret-manager
type: Opaque
data:
  devops.yaml: <base64-encoded-kubeconfig>
  integraciones-dev.yaml: <base64-encoded-kubeconfig>
---
# Mount as volume in backend pod
volumeMounts:
- name: kubeconfigs
  mountPath: /etc/kubeconfigs
  readOnly: true
volumes:
- name: kubeconfigs
  secret:
    secretName: cluster-kubeconfigs
    defaultMode: 0400
```

## API Layer

### Cluster Management Endpoints

New endpoints for multi-cluster operations:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/clusters` | GET | List all registered clusters |
| `/clusters` | POST | Register new cluster (admin) |
| `/clusters/{id}` | GET | Get cluster details + health status |
| `/clusters/{id}` | DELETE | Delete cluster (fails if has namespaces) |
| `/clusters/{id}/health` | GET | Check K8s API connectivity |
| `/clusters/{id}/namespaces` | GET | List namespaces in cluster |

### Namespace Filtering

The `/namespaces` endpoint now supports optional cluster filtering:

```bash
# Get all namespaces across all clusters
GET /api/v1/namespaces

# Get namespaces for specific cluster
GET /api/v1/clusters/{cluster-id}/namespaces
```

**Response includes cluster context**:
```json
{
  "id": "uuid",
  "name": "development",
  "cluster_id": "cluster-uuid",
  "cluster": {
    "id": "cluster-uuid",
    "name": "devops",
    "environment": "production",
    "is_healthy": true
  }
}
```

## Drift Detection

Drift detection now operates **per cluster**:

```go
// Pseudo-code workflow
for each cluster {
    if !cluster.IsHealthy {
        log.Warn("Skipping unhealthy cluster")
        continue
    }
    
    k8sClient = clientManager.GetClient(cluster.ID)
    
    for each namespace in cluster {
        gitSecrets = readFromGit("clusters/{cluster}/namespaces/{ns}/secrets/")
        k8sSecrets = k8sClient.CoreV1().Secrets(namespace).List()
        
        diff = compare(gitSecrets, k8sSecrets)
        if diff {
            createDriftEvent(namespace, diff)
        }
    }
}
```

**Key Changes**:
- Drift detection loops through clusters first, then namespaces
- Unhealthy clusters are skipped with a warning log
- Git paths use cluster-first structure
- Drift events reference the cluster via `namespace.cluster_id`

## RBAC Integration

Permissions can now be scoped to specific clusters:

```sql
-- GroupPermission model
CREATE TABLE group_permissions (
    id UUID PRIMARY KEY,
    group_id UUID NOT NULL REFERENCES groups(id),
    namespace_id UUID REFERENCES namespaces(id),
    cluster_id UUID REFERENCES clusters(id),  -- NEW: Optional cluster scope
    permission_level TEXT NOT NULL,  -- read, write, publish, delete
    created_at TIMESTAMP NOT NULL
);
```

**Permission Hierarchy**:
1. **Global Admin**: No `cluster_id` or `namespace_id` → Access all clusters/namespaces
2. **Cluster Admin**: `cluster_id` set, no `namespace_id` → Access all namespaces in cluster
3. **Namespace Editor**: Both `cluster_id` and `namespace_id` set → Access specific namespace

## References

- **Swagger API Docs**: `/backend/docs/swagger.yaml` (auto-generated)
- **ClientManager Implementation**: `/backend/internal/k8s/manager.go`
- **Database Migrations**: `/backend/migrations/008-010-*.sql`
- **API Handlers**: `/backend/internal/api/clusters.go`
- **Git Sync Logic**: `/backend/internal/gitsync/syncer.go`

## Next Steps

For deployment and operational guides, see:
- [API Documentation](./API.md) - Complete API reference
- [FluxCD Multi-Cluster Setup](./fluxcd-multi-cluster.md) - Production GitOps configuration
- [RBAC Model](./rbac-model.md) - Detailed permission model
