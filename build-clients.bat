@echo off
REM Windows Build Script for GoRAT Client
REM Builds the client for all platforms and architectures
REM Usage: build-clients.bat [clean|release]

setlocal enabledelayedexpansion

REM Color codes
set "COLOR_RESET=[0m"
set "COLOR_GREEN=[32m"
set "COLOR_YELLOW=[33m"
set "COLOR_BLUE=[34m"
set "COLOR_RED=[31m"

REM Check for clean flag
set "CLEAN_BUILD=0"
set "RELEASE_BUILD=0"

if "%1"=="clean" set "CLEAN_BUILD=1"
if "%1"=="release" set "RELEASE_BUILD=1"

echo.
echo [34m==========================================
echo          GoRAT Client Build Script
echo ==========================================
echo.

REM Check if we're in the right directory
if not exist "go.mod" (
    echo [31mError: go.mod not found. Please run this script from the project root.
    exit /b 1
)

REM Check Go installation
echo [34mChecking Go installation...
go version >nul 2>&1
if errorlevel 1 (
    echo [31mError: Go is not installed or not in PATH.
    echo [31mPlease install Go from https://golang.org/dl/
    exit /b 1
)
for /f "tokens=3" %%A in ('go version') do set "GO_VERSION=%%A"
echo [32m✓ Found Go %GO_VERSION%
echo.

REM Create bin directory
if not exist "bin" (
    echo [34mCreating bin directory...
    mkdir bin
)

REM Clean build cache if requested
if %CLEAN_BUILD% equ 1 (
    echo [34mCleaning build cache...
    go clean -cache 2>nul
    go clean -modcache 2>nul
    echo [32m✓ Cache cleaned
    echo.
)

REM Download dependencies
echo [34mDownloading dependencies...
go mod download 2>nul
echo [32m✓ Dependencies downloaded
echo.

REM Define target platforms and architectures
REM Format: GOOS:GOARCH
set "targets="
set "targets=!targets! windows:amd64"
set "targets=!targets! windows:386"
set "targets=!targets! linux:amd64"
set "targets=!targets! linux:386"
set "targets=!targets! linux:arm"
set "targets=!targets! linux:arm64"
set "targets=!targets! darwin:amd64"
set "targets=!targets! darwin:arm64"

REM Define output names
set "output_windows_amd64=bin\client-windows-amd64.exe"
set "output_windows_386=bin\client-windows-386.exe"
set "output_linux_amd64=bin\client-linux-amd64"
set "output_linux_386=bin\client-linux-386"
set "output_linux_arm=bin\client-linux-arm"
set "output_linux_arm64=bin\client-linux-arm64"
set "output_darwin_amd64=bin\client-darwin-amd64"
set "output_darwin_arm64=bin\client-darwin-arm64"

REM Define version info (optional)
set "VERSION=1.0.0"
set "TIMESTAMP=%date% %time%"

REM Count builds
set "TOTAL_TARGETS=0"
set "SUCCESSFUL_BUILDS=0"
set "FAILED_BUILDS=0"

echo [34m==========================================
echo          Building Client Binaries
echo ==========================================
echo.

REM Build for each target
for %%T in (%targets%) do (
    for /f "tokens=1,2 delims=:" %%A in ("%%T") do (
        set "GOOS=%%A"
        set "GOARCH=%%B"
        set /a TOTAL_TARGETS+=1
        
        REM Get output variable name
        set "OUTPUT_VAR=output_!GOOS!_!GOARCH!"
        for /f "delims=" %%O in ('echo !OUTPUT_VAR!') do set "OUTPUT=!!%%O!!"
        
        REM Build flags
        set "BUILD_FLAGS=-v"
        
        REM Add optimization flags for release builds
        if %RELEASE_BUILD% equ 1 (
            set "BUILD_FLAGS=!BUILD_FLAGS! -ldflags=-s -w"
        )
        
        REM Set output binary name
        set "BUILD_FLAGS=!BUILD_FLAGS! -o !OUTPUT!"
        
        REM Display build info
        echo [33m► Building: !GOOS!-!GOARCH!
        echo   Output: !OUTPUT!
        
        REM Build the client
        set "GOOS=!GOOS!"
        set "GOARCH=!GOARCH!"
        
        setlocal enabledelayedexpansion
        go build !BUILD_FLAGS! .\cmd\client\main.go
        endlocal & set "BUILD_RESULT=%errorlevel%"
        
        if %BUILD_RESULT% equ 0 (
            echo [32m  ✓ Built successfully
            set /a SUCCESSFUL_BUILDS+=1
            
            REM Show file size
            for %%F in ("!OUTPUT!") do set "FILE_SIZE=%%~zF"
            echo   Size: !FILE_SIZE! bytes
        ) else (
            echo [31m  ✗ Build failed with error code %BUILD_RESULT%
            set /a FAILED_BUILDS+=1
        )
        echo.
    )
)

REM Summary
echo [34m==========================================
echo          Build Summary
echo ==========================================
echo [0m
echo Total targets:       %TOTAL_TARGETS%
echo Successful builds:   [32m%SUCCESSFUL_BUILDS%[0m
echo Failed builds:       [31m%FAILED_BUILDS%[0m
echo.

if %FAILED_BUILDS% equ 0 (
    echo [32m✓ All builds completed successfully!
) else (
    echo [31m⚠ Some builds failed. Check the output above.
)

echo.
echo [34mBuilt binaries:[0m
echo.
for %%F in (bin\client-*) do (
    for %%Z in (%%F) do (
        echo   %%~nF
    )
)

echo.
echo [34mUsage examples:[0m
echo.
echo   Windows x64:
echo     .\bin\client-windows-amd64.exe -server wss://your-server/ws
echo.
echo   Windows x86:
echo     .\bin\client-windows-386.exe -server wss://your-server/ws
echo.
echo   Linux x64:
echo     ./bin/client-linux-amd64 -server wss://your-server/ws
echo.
echo   macOS Intel:
echo     ./bin/client-darwin-amd64 -server wss://your-server/ws
echo.
echo   macOS Apple Silicon:
echo     ./bin/client-darwin-arm64 -server wss://your-server/ws
echo.

REM Exit with appropriate code
if %FAILED_BUILDS% equ 0 (
    exit /b 0
) else (
    exit /b 1
)
