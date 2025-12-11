@echo off
REM Windows Build Script for GoRAT Client
REM Builds the client for all platforms and architectures
REM Usage: build-clients.bat [clean] [release]

setlocal enabledelayedexpansion

REM Check for flags
set "CLEAN_BUILD=0"
set "RELEASE_BUILD=0"

if "%1"=="clean" set "CLEAN_BUILD=1"
if "%1"=="release" set "RELEASE_BUILD=1"
if "%2"=="clean" set "CLEAN_BUILD=1"
if "%2"=="release" set "RELEASE_BUILD=1"

cls
echo.
echo ==========================================
echo     GoRAT Client Build Script
echo ==========================================
echo.

REM Check if we're in the right directory
if not exist "go.mod" (
    echo Error: go.mod not found. Please run this script from the project root.
    echo.
    exit /b 1
)

REM Check Go installation
echo Checking Go installation...
go version >nul 2>&1
if errorlevel 1 (
    echo Error: Go is not installed or not in PATH.
    echo Please install Go from https://golang.org/dl/
    echo.
    exit /b 1
)

for /f "tokens=3" %%A in ('go version') do set "GO_VERSION=%%A"
echo [OK] Found Go %GO_VERSION%
echo.

REM Create bin directory
if not exist "bin" (
    echo Creating bin directory...
    mkdir bin
)

REM Clean build cache if requested
if %CLEAN_BUILD%==1 (
    echo Cleaning build cache...
    go clean -cache
    go clean -modcache
    echo.
)

REM Set build flags
set "BUILD_FLAGS="
if %RELEASE_BUILD%==1 (
    set "BUILD_FLAGS=-ldflags=-s -w"
    echo Building RELEASE binaries (optimized)
) else (
    echo Building DEBUG binaries
)
echo.

REM Initialize counters
set "SUCCESSFUL=0"
set "FAILED=0"

echo ==========================================
echo     Building Client Binaries
echo ==========================================
echo.

REM Windows amd64
call :build_client windows amd64 bin\client-windows-amd64.exe
REM Windows 386
call :build_client windows 386 bin\client-windows-386.exe
REM Linux amd64
call :build_client linux amd64 bin\client-linux-amd64
REM Linux 386
call :build_client linux 386 bin\client-linux-386
REM Linux ARM
call :build_client linux arm bin\client-linux-arm
REM Linux ARM64
call :build_client linux arm64 bin\client-linux-arm64
REM macOS amd64
call :build_client darwin amd64 bin\client-darwin-amd64
REM macOS ARM64
call :build_client darwin arm64 bin\client-darwin-arm64

echo.
echo ==========================================
echo     Build Summary
echo ==========================================
echo Successful: %SUCCESSFUL%
echo Failed:     %FAILED%
echo.

if %FAILED%==0 (
    echo All builds completed successfully!
    echo.
    echo Built binaries:
    dir /b bin\client-*
    echo.
) else (
    echo Some builds failed. Check output above.
    echo.
    exit /b 1
)

endlocal
exit /b 0

REM Build function - takes GOOS, GOARCH, and output path
:build_client
setlocal enabledelayedexpansion
set "GOOS=%1"
set "GOARCH=%2"
set "OUTPUT=%3"

echo Building %GOOS% %GOARCH%...

setlocal
set GOOS=%1
set GOARCH=%2

if "%BUILD_FLAGS%"=="" (
    go build -o "%OUTPUT%" .\cmd\client\main.go
) else (
    go build %BUILD_FLAGS% -o "%OUTPUT%" .\cmd\client\main.go
)

if errorlevel 1 (
    echo   [FAILED]
    set /a FAILED+=1
    endlocal
    endlocal & set "FAILED=%FAILED%"
    exit /b 1
)

echo   [OK]
set /a SUCCESSFUL+=1
endlocal
endlocal & set "SUCCESSFUL=%SUCCESSFUL%"
exit /b 0
