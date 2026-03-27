#!/bin/bash
# Migration script: Convert Git structure from flat to cluster-first
# From: namespaces/{ns}/secrets/
# To:   clusters/{cluster}/namespaces/{ns}/secrets/
#
# Strategy: Blue-green migration with git mv to preserve history
#
# Usage: ./migrate-git-structure.sh <git-repo-path> <database-url> [default-cluster-name]
#
# Prerequisites:
# - psql client installed (for database queries)
# - Git repository cloned locally
# - Database accessible via DATABASE_URL

set -e
set -o pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Parse arguments
if [ $# -lt 2 ]; then
    log_error "Usage: $0 <git-repo-path> <database-url> [default-cluster-name]"
    echo "Example: $0 /data/secrets-repo postgresql://user:pass@localhost:5432/secretmanager devops"
    exit 1
fi

GIT_REPO_PATH="$1"
DATABASE_URL="$2"
DEFAULT_CLUSTER="${3:-devops}"

log_info "Git Migration Script - Multi-cluster support"
log_info "============================================="
log_info "Repository: $GIT_REPO_PATH"
log_info "Database: ${DATABASE_URL%%@*}@***"  # Hide credentials
log_info "Default cluster: $DEFAULT_CLUSTER"
echo ""

# Validate Git repository exists
if [ ! -d "$GIT_REPO_PATH/.git" ]; then
    log_error "Git repository not found at: $GIT_REPO_PATH"
    exit 1
fi

cd "$GIT_REPO_PATH"

# Check for uncommitted changes
if ! git diff-index --quiet HEAD --; then
    log_error "Git repository has uncommitted changes. Please commit or stash them first."
    git status
    exit 1
fi

log_info "Git repository is clean"

# Check if old namespaces/ directory exists
if [ ! -d "namespaces" ]; then
    log_warn "No namespaces/ directory found - nothing to migrate"
    exit 0
fi

# Check if clusters/ directory already exists
if [ -d "clusters" ]; then
    log_warn "clusters/ directory already exists - migration may have already run"
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Migration aborted"
        exit 0
    fi
fi

# Query database for cluster-namespace mappings
log_info "Querying database for cluster-namespace mappings..."

# Create temp SQL file
TEMP_SQL=$(mktemp)
cat > "$TEMP_SQL" <<'EOSQL'
SELECT 
    n.name AS namespace_name,
    COALESCE(c.name, n.cluster) AS cluster_name
FROM namespaces n
LEFT JOIN clusters c ON n.cluster_id = c.id
WHERE n.name != ''
ORDER BY cluster_name, namespace_name;
EOSQL

# Execute query and save results
MAPPINGS=$(psql "$DATABASE_URL" -t -F'|' -f "$TEMP_SQL" 2>&1)
QUERY_EXIT_CODE=$?

rm -f "$TEMP_SQL"

if [ $QUERY_EXIT_CODE -ne 0 ]; then
    log_error "Failed to query database:"
    echo "$MAPPINGS"
    log_warn "Falling back to single-cluster migration with cluster: $DEFAULT_CLUSTER"
    
    # Build mappings manually from filesystem
    MAPPINGS=""
    for ns_dir in namespaces/*/; do
        if [ -d "$ns_dir" ]; then
            ns_name=$(basename "$ns_dir")
            MAPPINGS="$MAPPINGS
 $ns_name|$DEFAULT_CLUSTER"
        fi
    done
fi

# Count namespaces to migrate
NAMESPACE_COUNT=$(echo "$MAPPINGS" | grep -v '^$' | wc -l)
log_info "Found $NAMESPACE_COUNT namespace(s) to migrate"

if [ "$NAMESPACE_COUNT" -eq 0 ]; then
    log_warn "No namespaces found - nothing to migrate"
    exit 0
fi

echo ""
log_info "Migration Plan:"
echo "---------------"
echo "$MAPPINGS" | grep -v '^$' | while IFS='|' read -r namespace cluster; do
    namespace=$(echo "$namespace" | xargs)  # Trim whitespace
    cluster=$(echo "$cluster" | xargs)
    echo "  $namespace -> clusters/$cluster/namespaces/$namespace"
done
echo ""

read -p "Proceed with migration? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    log_info "Migration aborted"
    exit 0
fi

# Create clusters directory structure
log_info "Creating cluster directory structure..."
echo "$MAPPINGS" | grep -v '^$' | while IFS='|' read -r namespace cluster; do
    namespace=$(echo "$namespace" | xargs)
    cluster=$(echo "$cluster" | xargs)
    
    if [ -z "$namespace" ] || [ -z "$cluster" ]; then
        continue
    fi
    
    cluster_dir="clusters/$cluster"
    mkdir -p "$cluster_dir"
done

# Perform git mv operations
log_info "Moving namespaces to cluster directories (preserving Git history)..."
MIGRATED_COUNT=0
FAILED_COUNT=0

echo "$MAPPINGS" | grep -v '^$' | while IFS='|' read -r namespace cluster; do
    namespace=$(echo "$namespace" | xargs)
    cluster=$(echo "$cluster" | xargs)
    
    if [ -z "$namespace" ] || [ -z "$cluster" ]; then
        continue
    fi
    
    old_path="namespaces/$namespace"
    new_path="clusters/$cluster/namespaces/$namespace"
    
    if [ ! -d "$old_path" ]; then
        log_warn "Namespace directory not found: $old_path (skipping)"
        continue
    fi
    
    if [ -d "$new_path" ]; then
        log_warn "Target directory already exists: $new_path (skipping)"
        continue
    fi
    
    log_info "Moving: $old_path -> $new_path"
    
    # Ensure parent directory exists
    mkdir -p "$(dirname "$new_path")"
    
    # Use git mv to preserve history
    if git mv "$old_path" "$new_path" 2>&1; then
        ((MIGRATED_COUNT++)) || true
    else
        log_error "Failed to move $old_path"
        ((FAILED_COUNT++)) || true
    fi
done

# Check if old namespaces directory is empty
if [ -d "namespaces" ]; then
    REMAINING=$(find namespaces -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)
    if [ "$REMAINING" -eq 0 ]; then
        log_info "Removing empty namespaces/ directory"
        rmdir namespaces 2>/dev/null || log_warn "Could not remove namespaces/ directory"
    else
        log_warn "namespaces/ directory still contains $REMAINING subdirectories - not removing"
    fi
fi

# Commit changes
log_info "Committing migration..."
git add -A

if git diff --cached --quiet; then
    log_warn "No changes to commit - migration may have already been applied"
else
    COMMIT_MSG="chore(git): migrate to cluster-first structure

- Migrated $MIGRATED_COUNT namespace(s) from flat structure to cluster-first
- Old path: namespaces/{ns}/secrets/
- New path: clusters/{cluster}/namespaces/{ns}/secrets/
- Git history preserved via 'git mv'

Related to multi-cluster support implementation."

    git commit -m "$COMMIT_MSG"
    log_info "Migration committed successfully"
fi

# Summary
echo ""
log_info "Migration Summary:"
log_info "=================="
log_info "Migrated: $MIGRATED_COUNT namespace(s)"
[ "$FAILED_COUNT" -gt 0 ] && log_error "Failed: $FAILED_COUNT namespace(s)"
echo ""

log_info "Git history preserved. You can verify with:"
echo "  git log --follow clusters/{cluster}/namespaces/{ns}/secrets/"
echo ""

log_warn "IMPORTANT: Do NOT push to remote yet!"
log_warn "1. Review the changes: git log -p"
log_warn "2. Test the backend with new paths"
log_warn "3. Update FluxCD Kustomizations to point to cluster-specific paths"
log_warn "4. Then push to remote: git push"
echo ""

log_info "To rollback, run: ./rollback-git-migration.sh $GIT_REPO_PATH"
