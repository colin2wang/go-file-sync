#Requires -Version 5.1
<#
.SYNOPSIS
    go-file-sync 构建脚本
.DESCRIPTION
    提供构建、测试等功能的 PowerShell 脚本
.EXAMPLE
    .\build.ps1 build
    .\build.ps1 test
#>

[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateSet('build', 'build-web', 'test', 'lint', 'clean', 'install', 'build-linux', 'build-darwin', 'build-windows', 'build-all', 'help')]
    [string]$Target = 'help'
)

# 变量定义
$BinaryName = 'go-file-sync'
$BuildDir = 'dist'
$GoFlags = @('-ldflags', '-s -w')

# 辅助函数
function Write-Step {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Green
}

function Write-Error {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Red
}

# 构建目标函数
function Invoke-BuildWeb {
    Write-Step "Building Vue3 frontend..."
    if (Test-Path "web") {
        Push-Location web
        if (-not (Test-Path "node_modules")) {
            Write-Step "Installing pnpm dependencies..."
            pnpm install
        }
        pnpm run build
        Pop-Location
        if ($LASTEXITCODE -eq 0) {
            Write-Success "Frontend build complete"
        } else {
            Write-Error "Frontend build failed!"
            exit 1
        }
    } else {
        Write-Host "web/ directory not found, skipping frontend build" -ForegroundColor Yellow
    }
}

function Invoke-Build {
    Invoke-BuildWeb
    Write-Step "Building $BinaryName..."
    if (-not (Test-Path $BuildDir)) {
        New-Item -ItemType Directory -Path $BuildDir | Out-Null
    }
    go build -o "$BuildDir/$BinaryName" $GoFlags .
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Build complete: $BuildDir/$BinaryName"
    } else {
        Write-Error "Build failed!"
        exit 1
    }
}

function Invoke-Test {
    Write-Step "Running tests..."
    go test ./... -v -count=1
}

function Invoke-Lint {
    Write-Step "Running linter..."
    go vet ./...
    if (Get-Command golangci-lint -ErrorAction SilentlyContinue) {
        golangci-lint run ./...
    } else {
        Write-Host "golangci-lint not installed. Skipping." -ForegroundColor Yellow
    }
}

function Invoke-Clean {
    Write-Step "Cleaning..."
    if (Test-Path $BuildDir) {
        Remove-Item -Recurse -Force $BuildDir
    }
    go clean
    Write-Success "Clean complete"
}

function Invoke-Install {
    Write-Step "Installing $BinaryName..."
    go install .
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Install complete"
    } else {
        Write-Error "Install failed!"
        exit 1
    }
}

function Invoke-BuildLinux {
    Invoke-BuildWeb
    Write-Step "Building for Linux (amd64)..."
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'
    if (-not (Test-Path $BuildDir)) {
        New-Item -ItemType Directory -Path $BuildDir | Out-Null
    }
    go build -o "$BuildDir/$BinaryName-linux-amd64" $GoFlags .
    Remove-Item Env:\GOOS
    Remove-Item Env:\GOARCH
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Linux build complete"
    } else {
        Write-Error "Linux build failed!"
        exit 1
    }
}

function Invoke-BuildDarwin {
    Invoke-BuildWeb
    Write-Step "Building for macOS (amd64)..."
    $env:GOOS = 'darwin'
    $env:GOARCH = 'amd64'
    if (-not (Test-Path $BuildDir)) {
        New-Item -ItemType Directory -Path $BuildDir | Out-Null
    }
    go build -o "$BuildDir/$BinaryName-darwin-amd64" $GoFlags .
    Remove-Item Env:\GOOS
    Remove-Item Env:\GOARCH
    if ($LASTEXITCODE -eq 0) {
        Write-Success "macOS build complete"
    } else {
        Write-Error "macOS build failed!"
        exit 1
    }
}

function Invoke-BuildWindows {
    Invoke-BuildWeb
    Write-Step "Building for Windows (amd64)..."
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    if (-not (Test-Path $BuildDir)) {
        New-Item -ItemType Directory -Path $BuildDir | Out-Null
    }
    go build -o "$BuildDir/$BinaryName-windows-amd64.exe" $GoFlags .
    Remove-Item Env:\GOOS
    Remove-Item Env:\GOARCH
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Windows build complete"
    } else {
        Write-Error "Windows build failed!"
        exit 1
    }
}

function Invoke-BuildAll {
    Invoke-BuildWeb

    if (-not (Test-Path $BuildDir)) {
        New-Item -ItemType Directory -Path $BuildDir | Out-Null
    }

    Write-Step "Building for Linux (amd64)..."
    $env:GOOS = 'linux'; $env:GOARCH = 'amd64'
    go build -o "$BuildDir/$BinaryName-linux-amd64" $GoFlags .
    Remove-Item Env:\GOOS; Remove-Item Env:\GOARCH

    Write-Step "Building for macOS (amd64)..."
    $env:GOOS = 'darwin'; $env:GOARCH = 'amd64'
    go build -o "$BuildDir/$BinaryName-darwin-amd64" $GoFlags .
    Remove-Item Env:\GOOS; Remove-Item Env:\GOARCH

    Write-Step "Building for Windows (amd64)..."
    $env:GOOS = 'windows'; $env:GOARCH = 'amd64'
    go build -o "$BuildDir/$BinaryName-windows-amd64.exe" $GoFlags .
    Remove-Item Env:\GOOS; Remove-Item Env:\GOARCH

    Write-Success "All builds complete"
}

function Show-Help {
    Write-Host "Usage: .\build.ps1 <target>" -ForegroundColor White
    Write-Host ""
    Write-Host "Targets:" -ForegroundColor Yellow
    Write-Host "  build          Build the binary (includes frontend)"
    Write-Host "  build-web      Build only the Vue3 frontend"
    Write-Host "  test           Run all tests"
    Write-Host "  lint           Run linters"
    Write-Host "  clean          Remove build artifacts"
    Write-Host "  install        Install binary to GOPATH"
    Write-Host "  build-linux    Cross-compile for Linux"
    Write-Host "  build-darwin   Cross-compile for macOS"
    Write-Host "  build-windows  Cross-compile for Windows"
    Write-Host "  build-all      Cross-compile for all platforms"
    Write-Host ""
    Write-Host "Examples:" -ForegroundColor Yellow
    Write-Host "  .\build.ps1 build"
    Write-Host "  .\build.ps1 build-web"
    Write-Host "  .\build.ps1 test"
}

# 执行目标
switch ($Target) {
    'build'         { Invoke-Build }
    'build-web'     { Invoke-BuildWeb }
    'test'          { Invoke-Test }
    'lint'          { Invoke-Lint }
    'clean'         { Invoke-Clean }
    'install'       { Invoke-Install }
    'build-linux'   { Invoke-BuildLinux }
    'build-darwin'  { Invoke-BuildDarwin }
    'build-windows' { Invoke-BuildWindows }
    'build-all'     { Invoke-BuildAll }
    'help'          { Show-Help }
    default         { Show-Help }
}
