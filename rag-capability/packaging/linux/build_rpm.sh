#!/usr/bin/env bash
# Automated Linux RPM Build Script for OmniRAG
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/output"

echo "============================================================"
echo " Building OmniRAG Linux RPM Package"
echo "============================================================"

mkdir -p "$OUTPUT_DIR"
RPMBUILD_DIR="$OUTPUT_DIR/rpmbuild"
mkdir -p "$RPMBUILD_DIR"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}

# 1. Build Go Engine for Linux x86_64
echo "[1/4] Compiling Go Connector Engine (Linux amd64)..."
cd "$PROJECT_ROOT/go-engine"
GOOS=linux GOARCH=amd64 go build -o go-engine ./cmd/server/main.go

# 2. Build Frontend React Bundle
echo "[2/4] Compiling Frontend React Bundle..."
cd "$PROJECT_ROOT/frontend-app"
npm run build

# 3. Create Source Archive
echo "[3/4] Preparing RPM Source Tree..."
cd "$PROJECT_ROOT"
tar --exclude='.git' --exclude='node_modules' --exclude='.venv' \
    -czf "$RPMBUILD_DIR/SOURCES/omnirag-1.0.0.tar.gz" .

cp "$SCRIPT_DIR/omnirag.spec" "$RPMBUILD_DIR/SPECS/"

# 4. Build RPM Package
echo "[4/4] Invoking rpmbuild..."
if command -v rpmbuild &>/dev/null; then
    rpmbuild --define "_topdir $RPMBUILD_DIR" -ba "$RPMBUILD_DIR/SPECS/omnirag.spec"
    echo ""
    echo "[SUCCESS] RPM package created at: $RPMBUILD_DIR/RPMS/x86_64/omnirag-1.0.0-1.x86_64.rpm"
else
    echo "[NOTICE] 'rpmbuild' not found on current host. Spec and source tarball are staged at $RPMBUILD_DIR."
    echo "To build on RHEL/Fedora/CentOS/Rocky, run:"
    echo "   rpmbuild --define '_topdir $(pwd)/$RPMBUILD_DIR' -ba $SCRIPT_DIR/omnirag.spec"
fi
