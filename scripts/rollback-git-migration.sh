#!/bin/bash
# Rollback script: Revert cluster-first Git structure back to flat
# From: clusters/{cluster}/namespaces/{ns}/secrets/
# To:   namespaces/{ns}/secrets/
#
# Usage: ./rollback-git-migration.sh <git-repo-path>
#
# WARNING: This will overwrite any changes made to the cluster-first structure!

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
if [ $# -lt 1 ]; then
    log_error "Usage: $0 <git-repo-path>"
    echo "Example: $0 /data/secrets-repo"
    exit 1
fi

GIT_REPO_PATH="$1"

log_warn "Git Rollback Script - Revert to flat structure"
log_warn "================================================"
log_warn "Repository: $GIT_REPO_PATH"
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

# Check if clusters/ directory exists
if [ ! -d "clusters" ]; then
    log_error "No clusters/ directory found - nothing to rollback"
    exit 1
fi

# Check if namespaces/ directory already exists
if [ -d "namespaces" ]; then
    log_warn "namespaces/ directory already exists - rollback may have conflicts"
fi

# Count namespaces to rollback
NAMESPACE_PATHS=$(find clusters -type d -path "*/namespaces/*" -mindepth 3 -maxdepth 3 2>/dev/null || echo "")
NAMESPACE_COUNT=$(echo "$NAMESPACE_PATHS" | grep -c "secrets$" || echo "0")

log_info "Found $NAMESPACE_COUNT namespace(s) to rollback"

if [ "$NAMESPACE_COUNT" -eq 0 ]; then
    log_warn "No namespaces found in clusters/ - nothing to rollback"
    exit 0
fi

echo ""
log_warn "Rollback Plan:"
echo "---------------"
find clusters -type d -name "secrets" -path "*/namespaces/*/secrets" 2>/dev/null | while read -r secrets_dir; do
    # Extract namespace and cluster from path
    # clusters/devops/namespaces/development/secrets -> development
    namespace=$(echo "$secrets_dir" | sed -E 's|^clusters/[^/]+/namespaces/([^/]+)/secrets$|\1|')
    cluster=$(echo "$secrets_dir" | sed -E 's|^clusters/([^/]+)/namespaces/.*$|\1|')
    old_path=$(dirname "$secrets_dir")  # clusters/devops/namespaces/development
    new_path="namespaces/$namespace"
    echo "  $old_path -> $new_path (from cluster: $cluster)"
done
echo ""

log_warn "WARNING: This will OVERWRITE the namespaces/ directory if it exists!"
log_warn "All Git history of the cluster-first structure will be preserved."
echo ""

read -p "Are you sure you want to proceed with rollback? (yes/NO) " -r
echo
if [[ ! $REPLY =~ ^yes$ ]]; then
    log_info "Rollback aborted"
    exit 0
fi

# Create namespaces directory
mkdir -p namespaces

# Perform rollback git mv operations
log_info "Moving namespaces back to flat structure (preserving Git history)..."
ROLLED_BACK_COUNT=0
FAILED_COUNT=0

find clusters -type d -name "secrets" -path "*/namespaces/*/secrets" 2>/dev/null | while read -r secrets_dir; do
    # Extract namespace from path
    namespace=$(echo "$secrets_dir" | sed -E 's|^clusters/[^/]+/namespaces/([^/]+)/secrets$|\1|')
    cluster=$(echo "$secrets_dir" | sed -E 's|^clusters/([^/]+)/namespaces/.*$|\1|')
    
    old_path=$(dirname "$secrets_dir")  # clusters/devops/namespaces/development
    new_path="namespaces/$namespace"
    
    if [ "$old_path" == "$new_path" ]; then
        continue  # Skip if already in flat structure
    fi
    
    log_info "Moving: $old_path -> $new_path (from cluster: $cluster)"
    
    # If target exists, remove it first (force overwrite)
    if [ -d "$new_path" ]; then
        log_warn "Target exists, removing: $new_path"
        git rm -rf "$new_path" 2>&1 || rm -rf "$new_path"
    fi
    
    # Use git mv to preserve history
    if git mv "$old_path" "$new_path" 2>&1; then
        ((ROLLED_BACK_COUNT++)) || true
    else
        log_error "Failed to move $old_path"
        ((FAILED_COUNT++)) || true
    fi
done

# Remove clusters directory if empty
if [ -d "clusters" ]; then
    log_info "Checking if clusters/ directory is empty..."
    REMAINING=$(find clusters -mindepth 1 -type d 2>/dev/null | wc -l)
    if [ "$REMAINING" -eq 0 ]; then
        log_info "Removing empty clusters/ directory"
        rmdir clusters 2>/dev/null || log_warn "Could not remove clusters/ directory"
    else
        log_warn "clusters/ directory still contains $REMAINING subdirectories - not removing"
        log_warn "You may need to manually clean up: rm -rf clusters/"
    fi
fi

# Commit changes
log_info "Committing rollback..."
git add -A

if git diff --cached --quiet; then
    log_warn "No changes to commit - rollback may have already been applied"
else
    COMMIT_MSG="chore(git): rollback to flat namespace structure

- Rolled back $ROLLED_BACK_COUNT namespace(s) from cluster-first to flat structure
- Old path: clusters/{cluster}/namespaces/{ns}/secrets/
- New path: namespaces/{ns}/secrets/
- Git history preserved via 'git mv'

This reverts the multi-cluster Git migration."

    git commit -m "$COMMIT_MSG"
    log_info "Rollback committed successfully"
fi

# Summary
echo ""
log_info "Rollback Summary:"
log_info "=================="
log_info "Rolled back: $ROLLED_BACK_COUNT namespace(s)"
[ "$FAILED_COUNT" -gt 0 ] && log_error "Failed: $FAILED_COUNT namespace(s)"
echo ""

log_warn "IMPORTANT: Do NOT push to remote yet!"
log_warn "1. Review the changes: git log -p"
log_warn "2. Test the backend with flat paths"
log_warn "3. Revert FluxCD Kustomizations to old paths"
log_warn "4. Then push to remote: git push"
echo ""

log_info "Git history preserved. You can verify with:"
echo "  git log --follow namespaces/{ns}/secrets/"
