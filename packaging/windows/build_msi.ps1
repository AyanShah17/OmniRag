# OmniRAG Windows MSI Automated Build Script

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Resolve-Path (Join-Path $ScriptDir "..\..")

Write-Host "============================================================" -ForegroundColor Cyan
Write-Host " Building OmniRAG Windows MSI Installer Package" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan

# 1. Build Go Engine Binary
Write-Host "`n[1/4] Compiling Go Connector Engine (Windows x64)..." -ForegroundColor Yellow
Push-Location (Join-Path $ProjectRoot "go-engine")
go build -o go-engine.exe ./cmd/server/main.go
Pop-Location
Write-Host "      Go binary compiled: go-engine\go-engine.exe" -ForegroundColor Green

# 2. Build Frontend React Bundle
Write-Host "`n[2/4] Compiling React + Shadcn Frontend Bundle..." -ForegroundColor Yellow
Push-Location (Join-Path $ProjectRoot "frontend-app")
npm run build
Pop-Location
Write-Host "      Frontend bundle built: frontend-app\dist" -ForegroundColor Green

# 3. Check for WiX Toolset
Write-Host "`n[3/4] Checking WiX Toolset Compiler..." -ForegroundColor Yellow
$Candle = Get-Command "candle.exe" -ErrorAction SilentlyContinue
$Light = Get-Command "light.exe" -ErrorAction SilentlyContinue
$WixExe = Get-Command "wix.exe" -ErrorAction SilentlyContinue

$OutputDir = Join-Path $ScriptDir "output"
if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

if ($Candle -and $Light) {
    Write-Host "      Found WiX v3 toolset. Compiling MSI package..." -ForegroundColor Green
    & candle.exe -nologo -out "$OutputDir\omnirag.wixobj" (Join-Path $ScriptDir "omnirag.wxs")
    & light.exe -nologo -ext WixUIExtension -out "$OutputDir\OmniRAG-1.0.0-x64.msi" "$OutputDir\omnirag.wixobj"
    Write-Host "`n[SUCCESS] Windows MSI Installer generated at: $OutputDir\OmniRAG-1.0.0-x64.msi" -ForegroundColor Green
} elseif ($WixExe) {
    Write-Host "      Found WiX v4 toolset. Building MSI package..." -ForegroundColor Green
    & wix.exe build (Join-Path $ScriptDir "omnirag.wxs") -o "$OutputDir\OmniRAG-1.0.0-x64.msi"
    Write-Host "`n[SUCCESS] Windows MSI Installer generated at: $OutputDir\OmniRAG-1.0.0-x64.msi" -ForegroundColor Green
} else {
    Write-Host "      WiX toolset (candle/light/wix.exe) not found in system PATH." -ForegroundColor Yellow
    Write-Host "      Generating portable Windows installation bundle instead..." -ForegroundColor Yellow
    
    $BundleDir = Join-Path $OutputDir "OmniRAG-Windows-x64-Setup"
    if (Test-Path $BundleDir) { Remove-Item -Recurse -Force $BundleDir }
    New-Item -ItemType Directory -Path $BundleDir -Force | Out-Null
    
    Copy-Item (Join-Path $ProjectRoot "go-engine\go-engine.exe") -Destination $BundleDir
    Copy-Item (Join-Path $ScriptDir "omnirag-service.bat") -Destination $BundleDir
    Copy-Item (Join-Path $ScriptDir "post_install.ps1") -Destination $BundleDir
    Copy-Item (Join-Path $ProjectRoot "packaging\config_manager\omnirag_config.py") -Destination $BundleDir
    Copy-Item (Join-Path $ProjectRoot ".env.example") -Destination (Join-Path $BundleDir "omnirag.env.example")
    Copy-Item -Recurse (Join-Path $ProjectRoot "python-rag") -Destination (Join-Path $BundleDir "python-rag")
    
    Compress-Archive -Path "$BundleDir\*" -DestinationPath "$OutputDir\OmniRAG-Windows-x64.zip" -Force
    Write-Host "`n[SUCCESS] Portable Windows Setup Bundle generated at: $OutputDir\OmniRAG-Windows-x64.zip" -ForegroundColor Green
}
