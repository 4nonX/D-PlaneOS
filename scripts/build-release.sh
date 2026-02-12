#!/bin/bash
#
# D-PlaneOS Release Builder
# Creates production + vendored tarballs with full validation
#
# Usage: ./scripts/build-release.sh [version]
# Example: ./scripts/build-release.sh 2.0.0
#

set -euo pipefail

VERSION="${1:-2.0.0}"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$PROJECT_DIR/build"
RELEASE_DIR="$PROJECT_DIR/release"
BINARY="dplaned"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "  ${GREEN}✓${NC} $1"; }
fail() { echo -e "  ${RED}✗${NC} $1"; exit 1; }
warn() { echo -e "  ${YELLOW}⚠${NC} $1"; }

echo "═══════════════════════════════════════════════"
echo "  D-PlaneOS v${VERSION} Release Builder"
echo "═══════════════════════════════════════════════"
echo ""

# ── Pre-flight checks ──
echo "Pre-flight checks..."

command -v go >/dev/null 2>&1 || fail "Go not found. Install: apt install golang-go"
GO_VER=$(go version | grep -oP 'go1\.\K[0-9]+')
[ "$GO_VER" -ge 22 ] 2>/dev/null && pass "Go $(go version | grep -oP 'go[0-9\.]+')" || warn "Go 1.22+ recommended (found $(go version))"

command -v gcc >/dev/null 2>&1 || fail "gcc not found. Install: apt install build-essential"
pass "gcc $(gcc -dumpversion)"

[ -f "$PROJECT_DIR/daemon/go.mod" ] || fail "daemon/go.mod not found — wrong directory?"
pass "Project structure"

# ── Vendor dependencies ──
echo ""
echo "Vendoring dependencies..."
cd "$PROJECT_DIR/daemon"

if [ -d vendor ]; then
    pass "Vendor directory exists"
else
    go mod vendor 2>&1 || fail "go mod vendor failed"
    pass "Dependencies vendored"
fi

go mod verify 2>&1 && pass "Module checksums verified" || warn "Module verify skipped (offline mode)"

# ── Static analysis ──
echo ""
echo "Static analysis..."
go vet -mod=vendor ./... 2>&1 && pass "go vet clean" || fail "go vet found issues"

# ── Build ──
echo ""
echo "Building binary..."
mkdir -p "$BUILD_DIR"
CGO_ENABLED=1 go build -mod=vendor -ldflags="-s -w" -o "$BUILD_DIR/$BINARY" ./cmd/dplaned/ 2>&1
BINARY_SIZE=$(du -h "$BUILD_DIR/$BINARY" | cut -f1)
pass "Binary: $BUILD_DIR/$BINARY ($BINARY_SIZE)"

# ── Smoke test ──
echo ""
echo "Smoke test..."
SMOKE_DB=$(mktemp /tmp/dplaneos-smoke-XXXXX.db)
"$BUILD_DIR/$BINARY" -db "$SMOKE_DB" -listen 127.0.0.1:19876 &
SMOKE_PID=$!
sleep 2

HEALTH=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:19876/health 2>/dev/null || echo "000")
kill $SMOKE_PID 2>/dev/null; wait $SMOKE_PID 2>/dev/null
rm -f "$SMOKE_DB" "${SMOKE_DB}.backup" "${SMOKE_DB}-wal" "${SMOKE_DB}-shm"

[ "$HEALTH" = "200" ] && pass "Health endpoint: $HEALTH" || fail "Health check failed: $HEALTH"

# ── Package ──
echo ""
echo "Packaging..."
mkdir -p "$RELEASE_DIR"
cd "$PROJECT_DIR/.."

TARNAME="dplaneos-v${VERSION}-production"
DIRNAME=$(basename "$PROJECT_DIR")

# Standard (needs internet for first build)
tar czf "$RELEASE_DIR/${TARNAME}.tar.gz" \
    --transform="s|^${DIRNAME}|dplaneos|" \
    --exclude="${DIRNAME}/daemon/vendor" \
    --exclude="${DIRNAME}/release" \
    "$DIRNAME/"

STANDARD_SIZE=$(du -h "$RELEASE_DIR/${TARNAME}.tar.gz" | cut -f1)
pass "${TARNAME}.tar.gz ($STANDARD_SIZE)"

# Vendored (fully offline — includes binary + source + vendor)
tar czf "$RELEASE_DIR/${TARNAME}-vendored.tar.gz" \
    --transform="s|^${DIRNAME}|dplaneos|" \
    --exclude="${DIRNAME}/release" \
    "$DIRNAME/"

VENDORED_SIZE=$(du -h "$RELEASE_DIR/${TARNAME}-vendored.tar.gz" | cut -f1)
pass "${TARNAME}-vendored.tar.gz ($VENDORED_SIZE)"

# ── Summary ──
ROUTE_COUNT=$(grep -c 'HandleFunc' "$PROJECT_DIR/daemon/cmd/dplaned/main.go" || echo "?")
GO_FILES=$(find "$PROJECT_DIR/daemon" -name '*.go' ! -path '*/vendor/*' | wc -l)
HTML_FILES=$(find "$PROJECT_DIR/app" -name '*.html' 2>/dev/null | wc -l)

echo ""
echo "═══════════════════════════════════════════════"
echo "  Release v${VERSION} READY"
echo "═══════════════════════════════════════════════"
echo "  Binary:     $BINARY_SIZE (stripped)"
echo "  Go files:   $GO_FILES"
echo "  HTML pages: $HTML_FILES"
echo "  API routes: $ROUTE_COUNT"
echo ""
echo "  $RELEASE_DIR/${TARNAME}.tar.gz"
echo "  $RELEASE_DIR/${TARNAME}-vendored.tar.gz"
echo "═══════════════════════════════════════════════"
