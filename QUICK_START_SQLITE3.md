# Quick Start - Fix SQLite3 on Linux

## TL;DR - Just Run This

```bash
chmod +x rebuild-sqlite3.sh
./rebuild-sqlite3.sh
```

Then test:
```bash
./bin/server -addr 127.0.0.1:8081
```

---

## What This Does

The `rebuild-sqlite3.sh` script automatically:

1. ✅ Checks if you have CGO dependencies (gcc, sqlite3-dev, pkg-config)
2. ✅ Installs missing dependencies with `sudo` if needed
3. ✅ **Cleans the Go module cache** (this is the key fix!)
4. ✅ Forces recompilation of sqlite3 with CGO enabled
5. ✅ Verifies the result
6. ✅ Shows detailed output

---

## If Something Goes Wrong

### Step 1: Diagnose

```bash
chmod +x diagnose-sqlite3.sh
./diagnose-sqlite3.sh
```

This will show you:
- ✓ What's installed correctly
- ✗ What's missing
- How to fix it

### Step 2: Install Dependencies

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install -y build-essential libsqlite3-dev pkg-config
```

**CentOS/RHEL:**
```bash
sudo yum groupinstall -y 'Development Tools'
sudo yum install -y sqlite-devel pkgconfig
```

**Alpine:**
```bash
apk add build-base sqlite-dev pkgconfig
```

### Step 3: Try Again

```bash
./rebuild-sqlite3.sh
```

---

## Success Indicators

When it works, you should see:

```
2025/12/11 10:00:00 ✅ SQLite database initialized successfully
2025/12/11 10:00:00 Server starting on 127.0.0.1:8081
2025/12/11 10:00:00 Web UI will be available at http://127.0.0.1:8081/login
```

NOT this error:
```
ERROR: Failed to create client store: sql: unknown driver "sqlite3"
```

---

## Available Documentation

| Document | Purpose |
|----------|---------|
| `SQLITE3_FIX_SUMMARY.md` | Overview of the problem and solution |
| `LINUX_BUILD_GUIDE.md` | Detailed Linux-specific instructions |
| `SQLITE3_COMPLETE_GUIDE.md` | Complete troubleshooting guide |

---

## Scripts Available

| Script | Purpose |
|--------|---------|
| `rebuild-sqlite3.sh` | 🚀 Fastest fix (recommended) |
| `diagnose-sqlite3.sh` | 🔍 Check your environment |
| `build-linux.sh` | 📊 Detailed build with diagnostics |
| `build.sh` | 🔧 General build (auto-detects OS) |

---

## Need More Help?

See **SQLITE3_COMPLETE_GUIDE.md** for:
- Step-by-step explanations
- Troubleshooting each issue
- Docker examples
- Advanced topics

---

## The Fix is Simple!

The problem: Go module cache holds sqlite3 builds without CGO

The solution: Clean cache + rebuild with CGO enabled

The script: `./rebuild-sqlite3.sh` ✨
