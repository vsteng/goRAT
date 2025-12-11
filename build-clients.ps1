#!/usr/bin/env pwsh
# PowerShell Build Script for GoRAT Client
# Cross-platform compilation for all supported platforms and architectures
# Usage: .\build-clients.ps1 [-Clean] [-Release]

param(
    [switch]$Clean,
    [switch]$Release
)

$ErrorActionPreference = "Continue"

# Start
Write-Host ""
Write-Host "=========================================="
Write-Host "   GoRAT Client Build Script"
Write-Host "=========================================="
Write-Host ""

# Check if we're in the right directory
if (-not (Test-Path "go.mod")) {
    Write-Host "Error: go.mod not found. Please run this script from the project root."
    Write-Host ""
    exit 1
}

# Check Go installation
Write-Host "Checking Go installation..."
$goVersion = go version 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Go is not installed or not in PATH."
    Write-Host "Please install Go from https://golang.org/dl/"
    Write-Host ""
    exit 1
}
Write-Host "[OK] $goVersion"
Write-Host ""

# Create bin directory
if (-not (Test-Path "bin")) {
    Write-Host "Creating bin directory..."
    New-Item -ItemType Directory -Path "bin" | Out-Null
}

# Clean build cache if requested
if ($Clean) {
    Write-Host "Cleaning build cache..."
    go clean -cache 2>$null | Out-Null
    go clean -modcache 2>$null | Out-Null
    Write-Host ""
}

# Set build flags
if ($Release) {
    Write-Host "Building RELEASE binaries (optimized)"
} else {
    Write-Host "Building DEBUG binaries"
}
Write-Host ""

# Define target platforms
$targets = @(
    @{ OS = "windows"; Arch = "amd64"; Name = "windows-amd64.exe" },
    @{ OS = "windows"; Arch = "386"; Name = "windows-386.exe" },
    @{ OS = "linux"; Arch = "amd64"; Name = "linux-amd64" },
    @{ OS = "linux"; Arch = "386"; Name = "linux-386" },
    @{ OS = "linux"; Arch = "arm"; Name = "linux-arm" },
    @{ OS = "linux"; Arch = "arm64"; Name = "linux-arm64" },
    @{ OS = "darwin"; Arch = "amd64"; Name = "darwin-amd64" },
    @{ OS = "darwin"; Arch = "arm64"; Name = "darwin-arm64" }
)

Write-Host "=========================================="
Write-Host "   Building Client Binaries"
Write-Host "=========================================="
Write-Host ""

$successful = 0
$failed = 0

# Build for each target
foreach ($target in $targets) {
    $outputPath = "bin/client-$($target.Name)"
    
    Write-Host "Building $($target.OS) $($target.Arch)..."
    
    # Build flags
    $buildFlags = @()
    
    if ($Release) {
        $buildFlags += "-ldflags=-s -w"
    }
    
    $buildFlags += "-o", $outputPath, ".\cmd\client\main.go"
    
    # Set environment variables
    $env:GOOS = $target.OS
    $env:GOARCH = $target.Arch
    
    # Build
    & go build @buildFlags 2>&1 | Out-Null
    
    if ($LASTEXITCODE -eq 0) {
        if (Test-Path $outputPath) {
            $fileSize = (Get-Item $outputPath).Length / 1024 / 1024
            Write-Host "  [OK] $(Get-Item $outputPath).Name ($([math]::Round($fileSize, 2))MB)"
            $successful++
        } else {
            Write-Host "  [FAILED] Output file not found"
            $failed++
        }
    } else {
        Write-Host "  [FAILED]"
        $failed++
    }
}

# Clear environment variables
Remove-Item env:GOOS -ErrorAction SilentlyContinue
Remove-Item env:GOARCH -ErrorAction SilentlyContinue

# Summary
Write-Host ""
Write-Host "=========================================="
Write-Host "   Build Summary"
Write-Host "=========================================="
Write-Host ""
Write-Host "Successful: $successful"
Write-Host "Failed:     $failed"
Write-Host ""

if ($failed -eq 0) {
    Write-Host "All builds completed successfully!"
    Write-Host ""
    Write-Host "Built binaries:"
    Get-ChildItem "bin/client-*" -ErrorAction SilentlyContinue | ForEach-Object {
        $size = [math]::Round($_.Length / 1024 / 1024, 2)
        Write-Host "  $($_.Name) ($size MB)"
    }
} else {
    Write-Host "Some builds failed. Check output above."
    exit 1
}

Write-Host ""

exit 0
