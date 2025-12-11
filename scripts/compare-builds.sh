#!/bin/bash
# Comparison script to demonstrate debug vs release builds

echo "======================================"
echo "Client Build Version Comparison"
echo "======================================"
echo ""

echo "1. Debug Version Behavior:"
echo "   Default: daemon=false, logging=enabled"
echo ""
echo "   Running: ./bin/client-debug -h"
echo "   ----------------------------------------"
./bin/client-debug -h 2>&1 | grep -A2 "build mode" | head -3
./bin/client-debug -h 2>&1 | grep -E "(daemon|autostart)" | grep "default"
echo ""

echo "2. Release Version Behavior:"
echo "   Default: daemon=true, logging=disabled"
echo ""
echo "   Running: CLIENT_ENABLE_LOG=1 ./bin/client-release -h"
echo "   ----------------------------------------"
CLIENT_ENABLE_LOG=1 ./bin/client-release -h 2>&1 | grep "build mode" || echo "   (No build mode log - logging disabled by default)"
CLIENT_ENABLE_LOG=1 ./bin/client-release -h 2>&1 | grep -E "(daemon|autostart)" | grep "default"
echo ""

echo "3. Log Output Comparison:"
echo "   ----------------------------------------"
echo "   Debug: status command (with logs):"
./bin/client-debug status 2>&1 | head -2
echo ""
echo "   Release: status command (no logs):"
./bin/client-release status 2>&1 | head -2
echo ""

echo "======================================"
echo "Build files created:"
echo "======================================"
ls -lh bin/client-debug bin/client-release 2>/dev/null || echo "Build files not found"
echo ""
echo "For more details, see BUILD_VERSIONS.md"
