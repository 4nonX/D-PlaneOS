#!/bin/bash
# Setup git hooks for D-PlaneOS development

set -e

HOOKS_DIR=".git/hooks"

echo "Setting up D-PlaneOS development hooks..."

# Install pre-commit hook
if [ ! -f "$HOOKS_DIR/pre-commit" ]; then
    echo "ERROR: pre-commit hook not found in repository"
    exit 1
fi

# Make hook executable
chmod +x "$HOOKS_DIR/pre-commit"
echo "✓ pre-commit hook installed and executable"

echo ""
echo "Hooks installed successfully!"
echo ""
echo "The following checks will run before each commit:"
echo "  • Nix flake validation (detects duplicate attributes, deprecated APIs)"
echo "  • Credential detection (prevents accidental secret commits)"
echo ""
echo "If a commit is blocked, run: nix flake check --no-build ."
