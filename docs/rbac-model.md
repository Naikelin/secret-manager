# RBAC Model Documentation

## Overview

The Secret Manager uses a **Group-Based Access Control** model for authorization. Users are assigned to groups (Azure AD groups or local groups), and groups are granted permissions to specific namespaces with different roles.

## Architecture

```
User → Groups → GroupPermissions (role per namespace) → Namespace
```

### Components

1. **User**: Represents a user authenticated via OAuth2 (Azure AD) or mock auth
2. **Group**: Represents an Azure AD group or local group
3. **GroupPermission**: Junction table linking Group → Namespace with a specific Role
4. **Namespace**: Kubernetes namespace (cluster + environment + name)
5. **Role**: Permission level (viewer, editor, admin)

## Role Hierarchy

Roles follow a hierarchy where higher roles inherit permissions from lower roles:

```
admin (level 3)
  ↓ inherits all from
editor (level 2)
  ↓ inherits all from
viewer (level 1)
```

## Permission Matrix

| Role   | Read Secrets | Write Secrets | Publish to Git | Delete Secrets | Manage Namespace |
|--------|--------------|---------------|----------------|----------------|------------------|
| viewer | ✅           | ❌            | ❌             | ❌             | ❌               |
| editor | ✅           | ✅            | ✅             | ❌             | ❌               |
| admin  | ✅           | ✅            | ✅             | ✅             | ✅               |

### Permission Definitions

#### Read Secrets (`viewer`, `editor`, `admin`)
- View secret drafts in staging area
- View published secrets from Git
- View drift events

#### Write Secrets (`editor`, `admin`)
- Create new secret drafts
- Update existing secret drafts
- Approve/reject draft changes

#### Publish to Git (`editor`, `admin`)
- Commit and push secret changes to Git repository
- Trigger ArgoCD sync

#### Delete Secrets (`admin` only)
- Delete secret drafts
- Delete secrets from Git
- Resolve drift events

#### Manage Namespace (`admin` only)
- Add/remove group permissions for the namespace
- Change role assignments
- Configure namespace settings

## Group-Based Model

### How It Works

1. **User Authentication**: User logs in via Azure AD → JWT contains `user_id` and `groups[]`
2. **Group Membership**: User's groups are stored in `user_groups` (many-to-many)
3. **Permission Assignment**: Each group can have multiple `GroupPermission` entries:
   - One per namespace
   - Each with a specific role

### Example

```
User: alice@company.com
Groups: ["devops-team", "platform-admins"]

GroupPermissions:
- devops-team → prod/app1 → editor
- devops-team → dev/app1 → admin
- platform-admins → prod/* → admin
```

**Result**: Alice has:
- **Editor** access to `prod/app1` (can read, write, publish — but NOT delete)
- **Admin** access to `dev/app1` (full control)
- **Admin** access to all prod namespaces via `platform-admins`

### Highest Role Wins

If a user belongs to multiple groups with different roles in the same namespace, **the highest role applies**:

```
User Groups:
- readonly-group → prod/app1 → viewer
- contributors-group → prod/app1 → editor
- admins-group → prod/app1 → admin

Effective Role: admin (highest)
```

## Implementation

### Permission Evaluation

Permission checks are implemented in `backend/internal/rbac/permissions.go`:

```go
// Check if user can read secrets in a namespace
canRead := rbac.CanReadSecret(userPermissions, namespaceID)

// Check if user can write secrets
canWrite := rbac.CanWriteSecret(userPermissions, namespaceID)

// Check if user can publish to Git
canPublish := rbac.CanPublishSecret(userPermissions, namespaceID)

// Check if user can delete secrets
canDelete := rbac.CanDeleteSecret(userPermissions, namespaceID)

// Check if user can manage namespace permissions
canManage := rbac.CanManageNamespace(userPermissions, namespaceID)
```

### Middleware

RBAC middleware in `backend/internal/middleware/rbac.go` protects API endpoints:

```go
// Require write permission for creating/updating secrets
r.Post("/namespaces/{namespaceID}/secrets", 
    middleware.RequireWrite(db, getNamespaceFromParam), 
    handlers.CreateSecret)

// Require publish permission for publishing to Git
r.Post("/secrets/{id}/publish", 
    middleware.RequirePublish(db, getNamespaceFromParam), 
    handlers.PublishSecret)

// Require admin permission for deleting secrets
r.Delete("/secrets/{id}", 
    middleware.RequireDelete(db, getNamespaceFromParam), 
    handlers.DeleteSecret)

// Require admin permission for managing namespace
r.Put("/namespaces/{namespaceID}/permissions", 
    middleware.RequireAdmin(db, getNamespaceFromParam), 
    handlers.UpdateNamespacePermissions)
```

### Loading User Permissions

```go
// Load all permissions for a user (via their groups)
permissions, err := rbac.GetUserPermissions(db, userID)
if err != nil {
    // Handle error
}

// Check permission
if rbac.CanWriteSecret(permissions, namespaceID) {
    // Allow operation
} else {
    // Deny with 403 Forbidden
}
```

## Database Schema

### Tables

**users**
- `id` (UUID, PK)
- `email` (unique)
- `name`
- `azure_ad_oid` (Azure AD Object ID)

**groups**
- `id` (UUID, PK)
- `name` (unique)
- `azure_ad_gid` (Azure AD Group ID)

**user_groups** (many-to-many)
- `user_id` (FK → users)
- `group_id` (FK → groups)

**namespaces**
- `id` (UUID, PK)
- `name`
- `cluster`
- `environment` (dev/staging/prod)

**group_permissions**
- `id` (UUID, PK)
- `group_id` (FK → groups)
- `namespace_id` (FK → namespaces)
- `role` (enum: viewer, editor, admin)

### Constraints

- Role check constraint: `role IN ('viewer', 'editor', 'admin')`
- Unique constraint on `(group_id, namespace_id)` to prevent duplicate permissions

## Adding New Roles or Permissions

To add a new role or permission type:

1. **Add role to hierarchy** in `rbac/permissions.go`:
   ```go
   const RoleNewRole Role = "newrole"
   
   var roleHierarchy = map[Role]int{
       RoleViewer: 1,
       RoleEditor: 2,
       RoleNewRole: 2.5,  // Between editor and admin
       RoleAdmin:  3,
   }
   ```

2. **Add permission check function**:
   ```go
   func CanDoNewAction(userGroups []models.GroupPermission, namespaceID uuid.UUID) bool {
       return hasRole(userGroups, namespaceID, RoleNewRole)
   }
   ```

3. **Add middleware** (if needed):
   ```go
   func RequireNewAction(db *gorm.DB, getNamespaceID func(r *http.Request) (uuid.UUID, error)) func(http.Handler) http.Handler {
       // Similar to other middleware
   }
   ```

4. **Update database constraint**:
   ```sql
   ALTER TABLE group_permissions 
   DROP CONSTRAINT group_permissions_role_check,
   ADD CONSTRAINT group_permissions_role_check 
   CHECK (role IN ('viewer', 'editor', 'newrole', 'admin'));
   ```

5. **Update tests** in `rbac/permissions_test.go`

6. **Update this documentation** with the new permission matrix

## Security Considerations

### Denial Logging

All permission denials are logged with structured logging (slog):

```go
slog.Warn("Permission denied",
    "user_id", userID,
    "email", email,
    "namespace_id", namespaceID,
    "action", "write",
)
```

These logs can be ingested into a SIEM for security monitoring.

### Principle of Least Privilege

- **Default deny**: No permissions by default
- **Explicit grants**: Users must be explicitly added to groups with permissions
- **Namespace isolation**: Permissions are scoped to specific namespaces

### Audit Trail

All permission checks and denials should be logged to the `audit_logs` table (to be implemented in Phase 6):

```go
auditLog := models.AuditLog{
    UserID:       &userID,
    ActionType:   "permission_denied",
    ResourceType: "secret",
    ResourceName: secretName,
    NamespaceID:  &namespaceID,
    Metadata:     json.Marshal(map[string]string{"required_role": "editor"}),
}
db.Create(&auditLog)
```

## Future Enhancements

1. **Fine-grained permissions**: Per-secret permissions (not just namespace-level)
2. **Temporary permissions**: Time-limited role grants
3. **Permission delegation**: Allow admins to delegate specific permissions
4. **Resource-based policies**: AWS IAM-style policy documents
5. **Approval workflows**: Multi-person approval for sensitive operations
