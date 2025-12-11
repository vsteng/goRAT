#!/bin/bash
# Verification script for LanProxy Integration

echo "🔍 LanProxy Integration - Verification Checklist"
echo "=================================================="
echo ""

PASSED=0
FAILED=0

# Check function
check_file() {
    if [ -f "$1" ]; then
        echo "✅ $2"
        ((PASSED++))
    else
        echo "❌ $2 (MISSING)"
        ((FAILED++))
    fi
}

echo "📄 Frontend Components:"
check_file "web/templates/dashboard-new.html" "dashboard-new.html"
check_file "web/templates/client-details.html" "client-details.html"
check_file "web/templates/dashboard.html" "dashboard.html (existing)"
check_file "web/templates/terminal.html" "terminal.html (existing)"
check_file "web/templates/files.html" "files.html (existing)"

echo ""
echo "🔧 Backend Components:"
check_file "server/proxy_handler.go" "proxy_handler.go (NEW)"
check_file "server/handlers.go" "handlers.go (UPDATED)"
check_file "server/web_handlers.go" "web_handlers.go (UPDATED)"

echo ""
echo "📚 Documentation:"
check_file "LANPROXY_INTEGRATION.md" "LANPROXY_INTEGRATION.md"
check_file "LANPROXY_QUICKSTART.md" "LANPROXY_QUICKSTART.md"
check_file "LANPROXY_TECHNICAL.md" "LANPROXY_TECHNICAL.md"
check_file "IMPLEMENTATION_COMPLETE.md" "IMPLEMENTATION_COMPLETE.md"

echo ""
echo "=================================================="
echo "✅ Passed: $PASSED"
echo "❌ Failed: $FAILED"
echo "=================================================="

if [ $FAILED -eq 0 ]; then
    echo ""
    echo "🎉 All files present and ready for deployment!"
    echo ""
    echo "Next steps:"
    echo "1. Build the project: cd server && go build -o ../bin/server main.go"
    echo "2. Start the server: ./bin/server -addr :8080 -web-user admin -web-pass admin"
    echo "3. Open in browser: http://localhost:8080/login"
    echo "4. Read LANPROXY_QUICKSTART.md for usage guide"
else
    echo ""
    echo "⚠️  Some files are missing. Please check the file paths."
    exit 1
fi
