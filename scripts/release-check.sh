#!/usr/bin/env bash
# scripts/release-check.sh
# Run this before tagging a release to ensure the repository is in a valid state.

set -e

# 1. Check for uncommitted changes
if [ -n "$(git status --porcelain)" ]; then
    echo "❌ ERROR: Working directory is not clean. Commit or stash your changes first."
    git status --short
    exit 1
fi

# 2. Check current version
VERSION=$(cat VERSION | tr -d '[:space:]')
echo "🔍 Current version in file: $VERSION"

# 3. Check if version matches the latest tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "none")
if [ "v$VERSION" == "$LATEST_TAG" ]; then
    echo "⚠️ WARNING: VERSION file ($VERSION) matches the latest tag ($LATEST_TAG). Did you forget to bump the version?"
fi

# 4. Check for Syncthing conflict files
CONFLICTS=$(find . -name '.sync-conflict-*.go' 2>/dev/null | head -5 || true)
if [ -n "$CONFLICTS" ]; then
    echo "❌ ERROR: Syncthing conflict files found:"
    echo "$CONFLICTS"
    exit 1
fi

# 5. Verify CHANGELOG has an entry for this version
if ! grep -q "## v$VERSION" docs/reference/CHANGELOG.md; then
    echo "❌ ERROR: docs/reference/CHANGELOG.md is missing an entry for v$VERSION"
    exit 1
fi

# 6. Check for submodule drifts
SUBMODULE_STATUS=$(git submodule status)
if echo "$SUBMODULE_STATUS" | grep -q "^+"; then
    echo "⚠️ WARNING: Submodules have uncommitted changes/pointers."
    echo "$SUBMODULE_STATUS"
fi

# 7. Verify documentation was updated alongside code changes
PREV_TAG=$(git tag --sort=-v:refname | sed -n '2p' 2>/dev/null || echo "none")
if [ "$PREV_TAG" != "none" ]; then
    # If HA code changed, the HA guide must also change.
    HA_CODE_CHANGED=$(git diff "$PREV_TAG"..HEAD -- daemon/internal/ha/ nixos/ha.nix | wc -l)
    HA_DOC_CHANGED=$(git diff  "$PREV_TAG"..HEAD -- docs/admin/HIGH-AVAILABILITY.md | wc -l)
    if [ "$HA_CODE_CHANGED" -gt 0 ] && [ "$HA_DOC_CHANGED" -eq 0 ]; then
        echo "❌ ERROR: HA code changed since $PREV_TAG but docs/admin/HIGH-AVAILABILITY.md was not updated."
        exit 1
    fi

    # If gitops code changed, the GitOps reference must also change.
    GITOPS_CODE_CHANGED=$(git diff "$PREV_TAG"..HEAD -- daemon/internal/gitops/ | wc -l)
    GITOPS_DOC_CHANGED=$(git diff  "$PREV_TAG"..HEAD -- docs/admin/GITOPS-DRIVEN-NAS.md docs/reference/GITOPS-REFERENCE.md | wc -l)
    if [ "$GITOPS_CODE_CHANGED" -gt 0 ] && [ "$GITOPS_DOC_CHANGED" -eq 0 ]; then
        echo "❌ ERROR: GitOps code changed since $PREV_TAG but docs/admin/GITOPS-DRIVEN-NAS.md was not updated."
        exit 1
    fi

    # General: at least one .md in docs/ must be modified (CHANGELOG alone satisfies this).
    DOCS_CHANGED=$(git diff "$PREV_TAG"..HEAD -- docs/ | wc -l)
    if [ "$DOCS_CHANGED" -eq 0 ]; then
        echo "❌ ERROR: No files in docs/ were modified since $PREV_TAG. Update documentation before releasing."
        exit 1
    fi

    echo "✅ Documentation checks passed (HA doc: ${HA_DOC_CHANGED} lines changed, total docs: ${DOCS_CHANGED} lines changed)"
else
    echo "⚠️ WARNING: No previous tag found - skipping documentation diff check"
fi

echo "✅ All checks passed! You are safe to tag: git tag v$VERSION"
