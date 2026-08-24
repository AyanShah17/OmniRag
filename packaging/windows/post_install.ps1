# OmniRAG Windows Post-Install Script
# Prompts the user to configure credentials and initializes directories

$ProgramDataDir = "$env:ProgramData\OmniRAG"
$ConfigFile = "$ProgramDataDir\omnirag.env"

if (-not (Test-Path $ProgramDataDir)) {
    New-Item -ItemType Directory -Path $ProgramDataDir -Force | Out-Null
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ConfigScript = Join-Path $ScriptDir "omnirag_config.py"

Write-Host "============================================================" -ForegroundColor Cyan
Write-Host " OmniRAG Enterprise Backend Successfully Installed!" -ForegroundColor Green
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Starting the Interactive Configuration Wizard to setup API keys..." -ForegroundColor Yellow
Write-Host ""

Start-Process powershell.exe -ArgumentList "-NoExit", "-Command", "python '$ConfigScript' --config '$ConfigFile'"
