#!/usr/bin/env pwsh
# PowerShell Build Script for GoRAT Client
# Cross-platform compilation for all supported platforms and architectures
# Usage: .\build-clients.ps1 [-Clean] [-Release] [-Verbose]

param(
    [switch]$Clean,
    [switch]$Release,
    [switch]$Verbose
)

# Enable strict error handling
$ErrorActionPreference = "Continue"

# Colors
$Color_Green = [ConsoleColor]::Green
$Color_Yellow = [ConsoleColor]::Yellow
$Color_Blue = [ConsoleColor]::Blue
$Color_Red = [ConsoleColor]::Red
$Color_Reset = [ConsoleColor]::Gray

function Write-Header {
    param([string]$Text)
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor $Color_Blue
    Write-Host "  $Text" -ForegroundColor $Color_Blue
    Write-Host "==========================================" -ForegroundColor $Color_Blue
    Write-Host ""
}

function Write-Section {
    param([string]$Text)
    Write-Host "► $Text" -ForegroundColor $Color_Yellow
}

function Write-Success {
    param([string]$Text)
    Write-Host "✓ $Text" -ForegroundColor $Color_Green
}

function Write-Error {
    param([string]$Text)
    Write-Host "✗ $Text" -ForegroundColor $Color_Red
}

function Write-Info {
    param([string]$Text)
    Write-Host "  $Text" -ForegroundColor $Color_Reset
}

# Start
Write-Header "GoRAT Client Build Script"

# Check if we're in the right directory
if (-not (Test-Path "go.mod")) {
    Write-Error "go.mod not found. Please run this script from the project root."
    exit 1
}

# Check Go installation
Write-Section "Checking Go installation..."
$goVersion = go version 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Error "Go is not installed or not in PATH."
    Write-Info "Please install Go from https://golang.org/dl/"
    exit 1
}
Write-Success "$goVersion"
Write-Host ""

# Create bin directory
if (-not (Test-Path "bin")) {
    Write-Section "Creating bin directory..."
    New-Item -ItemType Directory -Path "bin" | Out-Null
    Write-Success "Created bin directory"
}

# Clean build cache if requested
if ($Clean) {
    Write-Section "Cleaning build cache..."
    go clean -cache 2>$null | Out-Null
    go clean -modcache 2>$null | Out-Null
    Write-Success "Cache cleaned"
    Write-Host ""
}

# Download dependencies
Write-Section "Downloading dependencies..."
go mod download 2>$null
Write-Success "Dependencies downloaded"
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

Write-Header "Building Client Binaries"

$totalTargets = $targets.Count
$successfulBuilds = 0
$failedBuilds = 0
$buildTimes = @{}

# Build for each target
foreach ($target in $targets) {
    $outputPath = "bin/client-$($target.Name)"
    
    Write-Info "Building: $($target.OS)-$($target.Arch)"
    Write-Info "Output: $outputPath"
    
    # Build flags
    $buildFlags = @("-v")
    
    # Add optimization flags for release builds
    if ($Release) {
        $buildFlags += "-ldflags=-s -w"
    }
    
    # Add output flag
    $buildFlags += "-o"
    $buildFlags += $outputPath
    
    # Add source path
    $buildFlags += ".\cmd\client\main.go"
    
    # Set environment variables
    $env:GOOS = $target.OS
    $env:GOARCH = $target.Arch
    
    # Track build time
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    
    # Build
    & go build @buildFlags 2>&1 | Where-Object { $Verbose -or $_ -match "error|warning" } | ForEach-Object { Write-Info $_ }
    
    $stopwatch.Stop()
    $buildTimes["$($target.OS)-$($target.Arch)"] = $stopwatch.ElapsedMilliseconds
    
    if ($LASTEXITCODE -eq 0) {
        # Check file size
        if (Test-Path $outputPath) {
            $fileSize = (Get-Item $outputPath).Length
            $sizeKB = [math]::Round($fileSize / 1024, 2)
            Write-Success "Built successfully in $($stopwatch.ElapsedMilliseconds)ms ($sizeKB KB)"
            $successfulBuilds++
        } else {
            Write-Error "Build completed but output file not found"
            $failedBuilds++
        }
    } else {
        Write-Error "Build failed with exit code $LASTEXITCODE"
        $failedBuilds++
    }
    
    Write-Host ""
}

# Clear environment variables
Remove-Item env:GOOS -ErrorAction SilentlyContinue
Remove-Item env:GOARCH -ErrorAction SilentlyContinue

# Summary
Write-Header "Build Summary"

Write-Info "Total targets:       $totalTargets"
Write-Info "Successful builds:   $(Write-Host $successfulBuilds -ForegroundColor $Color_Green -NoNewline; Write-Host "")"
Write-Info "Failed builds:       $(Write-Host $failedBuilds -ForegroundColor $(if ($failedBuilds -gt 0) { $Color_Red } else { $Color_Green }) -NoNewline; Write-Host "")"
Write-Host ""

if ($failedBuilds -eq 0) {
    Write-Success "All builds completed successfully!"
} else {
    Write-Error "$failedBuilds build(s) failed. Check the output above."
}

Write-Host ""
Write-Section "Built binaries:"
Write-Host ""

Get-ChildItem "bin/client-*" -ErrorAction SilentlyContinue | ForEach-Object {
    $name = $_.Name
    $size = [math]::Round($_.Length / 1024, 2)
    Write-Info "$name ($size KB)"
}

Write-Host ""
Write-Section "Usage examples:"
Write-Host ""

@"
Windows x64 (native):
  .\bin\client-windows-amd64.exe -server wss://your-server/ws

Windows x86:
  .\bin\client-windows-386.exe -server wss://your-server/ws

Linux x64:
  ./bin/client-linux-amd64 -server wss://your-server/ws

Linux ARM (Raspberry Pi):
  ./bin/client-linux-arm -server wss://your-server/ws

Linux ARM64:
  ./bin/client-linux-arm64 -server wss://your-server/ws

macOS Intel:
  ./bin/client-darwin-amd64 -server wss://your-server/ws

macOS Apple Silicon:
  ./bin/client-darwin-arm64 -server wss://your-server/ws

Run as daemon (background service):
  ./bin/client-linux-amd64 -daemon -server wss://your-server/ws

"@ | Write-Host -ForegroundColor $Color_Reset

# Build statistics
if ($buildTimes.Count -gt 0) {
    Write-Host ""
    Write-Section "Build times:"
    Write-Host ""
    foreach ($target in $targets) {
        $key = "$($target.OS)-$($target.Arch)"
        if ($buildTimes.ContainsKey($key)) {
            $time = $buildTimes[$key]
            Write-Info "$key`: $($time)ms"
        }
    }
}

Write-Host ""

# Exit with appropriate code
if ($failedBuilds -eq 0) {
    exit 0
} else {
    exit 1
}
