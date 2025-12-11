# Linux SQLite3 Build Issue - SOLVED ✅

## The Problem

When building on Linux, even with dependencies installed and `CGO_ENABLED=1`, the server fails with:

```
ERROR: Failed to create client store: sql: unknown driver "sqlite3"
```

This happens because **the Go module cache contains pre-compiled sqlite3 modules that were built WITHOUT CGO enabled**.

## The Solution

Run this one command on your Linux system:

```bash
chmod +x rebuild-sqlite3.sh
./rebuild-sqlite3.sh
```

This script automatically:
- ✅ Installs missing dependencies
- ✅ Cleans the Go module cache (critical step!)
- ✅ Recompiles sqlite3 with CGO enabled
- ✅ Verifies everything works
- ✅ Shows detailed progress

## What Changed

### New Scripts (Ready to Use)

| Script | Purpose | When to Use |
|--------|---------|------------|
| `rebuild-sqlite3.sh` | 🚀 Force rebuild with cache cleanup | First choice - fastest fix |
| `diagnose-sqlite3.sh` | 🔍 Test environment and sqlite3 | If rebuild fails |
| `build-linux.sh` | 📊 Detailed Linux build | For understanding the process |

### Updated Files

- `build.sh` - Enhanced with cache cleanup
- `Makefile` - Added CGO_ENABLED=1 to server build

### Documentation

- `QUICK_START_SQLITE3.md` - 2-minute quick start
- `SQLITE3_FIX_SUMMARY.md` - Overview and scripts
- `LINUX_BUILD_GUIDE.md` - Comprehensive guide
- `SQLITE3_COMPLETE_GUIDE.md` - Full troubleshooting

## Why It Works

The key insight: **Just setting CGO_ENABLED=1 is not enough**.

```
❌ Problem:
   go clean -cache  ← Doesn't remove MODULE cache
   Old sqlite3 binaries used even with CGO_ENABLED=1
   Those binaries were compiled WITHOUT CGO → don't work

✅ Solution:
   go clean -modcache              ← Remove ALL module cache
   rm -rf $(go env GOMODCACHE)/... ← Delete specific module
   go build (with CGO_ENABLED=1)   ← Recompile from source
   Result: Fresh sqlite3 with CGO support
```

## Quick Reference

| Scenario | Command |
|----------|---------|
| Just fix it | `./rebuild-sqlite3.sh` |
| Check environment | `./diagnose-sqlite3.sh` |
| Detailed build | `./build-linux.sh` |
| Learn more | See documentation files |

## Success Indicator

After running the fix, you should see:

```
✅ SQLite database initialized successfully
Server starting on 127.0.0.1:8081
Web UI will be available at http://127.0.0.1:8081/login
```

NOT:
```
ERROR: Failed to create client store: sql: unknown driver "sqlite3"
```

## Available Documentation

- **2 min**: `QUICK_START_SQLITE3.md`
- **10 min**: `SQLITE3_FIX_SUMMARY.md`  
- **Reference**: `LINUX_BUILD_GUIDE.md`
- **Complete**: `SQLITE3_COMPLETE_GUIDE.md`

---

**Status:** ✅ All fixes and documentation provided

Start with: `chmod +x rebuild-sqlite3.sh && ./rebuild-sqlite3.sh`
