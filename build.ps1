# requires PowerShell 5.1+
# FileSync build script:  .\build.ps1 [-Clean] [-Package]
param(
    [switch]$Clean,
    [switch]$Package
)
$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
Set-Location $root

if ($Clean) {
    Write-Host "== Clean =="
    Remove-Item -Recurse -Force "$root\frontend\dist", "$root\frontend\node_modules", "$root\build\bin" -ErrorAction SilentlyContinue
}

if (-not (Get-Command pnpm -ErrorAction SilentlyContinue)) {
    throw "pnpm not found. Install it first: npm install -g pnpm"
}

if (-not (Test-Path "$root\frontend\node_modules")) {
    Write-Host "== pnpm approve-builds =="
    Push-Location "$root\frontend"; pnpm approve-builds; Pop-Location

    Write-Host "== pnpm install =="
    $lockfileArgs = @()
    if (Test-Path "$root\frontend\pnpm-lock.yaml") { $lockfileArgs = @("--frozen-lockfile") }
    Push-Location "$root\frontend"; pnpm install @lockfileArgs; Pop-Location
    if ($LASTEXITCODE -ne 0) { throw "pnpm install failed" }
}

Write-Host "== frontend build (vue-tsc + vite) =="
Push-Location "$root\frontend"; pnpm run build; Pop-Location
if ($LASTEXITCODE -ne 0) { throw "frontend build failed" }

Write-Host "== embed icon (go-winres -> .syso) =="
go run github.com/tc-hib/go-winres@v0.3.3 simply `
    --icon "$root\build\windows\icon.ico" `
    --product-name "FileSync" `
    --product-version "1.0.0" `
    --file-description "FileSync 文件同步备份工具"
if ($LASTEXITCODE -ne 0) { throw "generate syso failed" }

Write-Host "== cross compile windows/amd64 =="
$env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
New-Item -ItemType Directory -Force -Path "$root\build\bin" | Out-Null
go build -tags "desktop,production" -ldflags "-w -s -H windowsgui" -o "$root\build\bin\FileSync.exe" .
Remove-Item "$root\rsrc_windows_amd64.syso" -ErrorAction SilentlyContinue
if ($LASTEXITCODE -ne 0) { throw "go build failed" }
Write-Host "OK: build\bin\FileSync.exe"

if ($Package) {
    Write-Host "== package zip =="
    Compress-Archive -Path "$root\build\bin\FileSync.exe", "$root\build\使用说明.md" -DestinationPath "$root\build\FileSync_文件同步备份工具.zip" -Force
    Write-Host "OK: build\FileSync_文件同步备份工具.zip"
}
