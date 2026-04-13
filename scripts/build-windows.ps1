# Context: This file belongs to the MetricsNode application layer around build-windows.

[CmdletBinding()]
param(
  [string]$WindowsDir = "windows",
  [switch]$SkipCleanBindings,
  [switch]$SkipGenerateBindings,
  [switch]$KeepGoWork
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$windowsPath = Join-Path $repoRoot $WindowsDir
$frontendPath = Join-Path $windowsPath "frontend"
$frontendDistPath = Join-Path $frontendPath "dist"
$bindingsPath = Join-Path $frontendPath "wailsjs/go"
$appBindingsPath = Join-Path $bindingsPath "main/App.d.ts"
$binaryPath = Join-Path $windowsPath "build/bin/windows.exe"

if (-not (Test-Path $windowsPath)) {
  throw "windows directory not found: $windowsPath"
}

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
  throw "wails command not found. Please install Wails CLI first: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
}

function Assert-MetricsNodeBindings {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Path
  )

  if (-not (Test-Path $Path)) {
    throw "generated Wails bindings not found: $Path"
  }

  $content = Get-Content -Raw $Path
  $requiredExports = @(
    "BootstrapGet",
    "BootstrapSet",
    "Status",
    "MetricsSettingsGet",
    "MetricsSettingsSet",
    "StartReporting"
  )
  $missing = @($requiredExports | Where-Object { $content -notmatch ("export function {0}\(" -f [regex]::Escape($_)) })
  if ($missing.Count -gt 0) {
    throw "generated Wails bindings do not match MyFlowHub-MetricsNode exports. Missing: $($missing -join ', ')"
  }

  $foreignExports = @(
    "AboutState",
    "FlowProjectsState",
    "SaveHomeState"
  )
  $unexpected = @($foreignExports | Where-Object { $content -match ("export function {0}\(" -f [regex]::Escape($_)) })
  if ($unexpected.Count -gt 0) {
    throw "generated Wails bindings appear to belong to another app. Unexpected exports: $($unexpected -join ', ')"
  }
}

Write-Host "Build Windows app via Wails" -ForegroundColor Cyan
Write-Host "  RepoRoot            : $repoRoot"
Write-Host "  WindowsDir          : $windowsPath"
Write-Host "  FrontendDir         : $frontendPath"
Write-Host "  CleanBindings       : $(-not $SkipCleanBindings)"
Write-Host "  GenerateBindings    : $(-not $SkipGenerateBindings)"
Write-Host "  KeepGoWork          : $KeepGoWork"

$oldGoWork = $env:GOWORK
if (-not $KeepGoWork) {
  $env:GOWORK = "off"
}

if (-not (Test-Path $frontendDistPath)) {
  Write-Host "Creating frontend dist directory for go:embed: $frontendDistPath" -ForegroundColor Yellow
  New-Item -ItemType Directory -Force -Path $frontendDistPath | Out-Null
}
$keepFile = Join-Path $frontendDistPath ".keep"
if (-not (Test-Path $keepFile)) {
  New-Item -ItemType File -Force -Path $keepFile | Out-Null
}

Push-Location $windowsPath
try {
  if (-not $SkipCleanBindings -and (Test-Path $bindingsPath)) {
    Write-Host "Cleaning stale Wails bindings: $bindingsPath" -ForegroundColor Yellow
    Remove-Item -Recurse -Force $bindingsPath
  }

  if (-not $SkipGenerateBindings) {
    if (Test-Path $frontendDistPath) {
      Write-Host "Running: wails generate module" -ForegroundColor Cyan
      wails generate module
      if ($LASTEXITCODE -ne 0) {
        throw "wails generate module failed (exit=$LASTEXITCODE)"
      }
    }
    else {
      Write-Host "Skip: wails generate module (frontend/dist not found). Wails build will regenerate bindings." -ForegroundColor Yellow
    }
  }

  Write-Host "Validating generated Wails bindings: $appBindingsPath" -ForegroundColor Cyan
  Assert-MetricsNodeBindings -Path $appBindingsPath

  Write-Host "Running: wails build" -ForegroundColor Cyan
  wails build
  if ($LASTEXITCODE -ne 0) {
    throw "wails build failed (exit=$LASTEXITCODE)"
  }
}
finally {
  Pop-Location
  if (-not $KeepGoWork) {
    if ([string]::IsNullOrWhiteSpace($oldGoWork)) {
      Remove-Item Env:GOWORK -ErrorAction SilentlyContinue
    }
    else {
      $env:GOWORK = $oldGoWork
    }
  }
}

if (-not (Test-Path $binaryPath)) {
  throw "build output not found: $binaryPath"
}

Write-Host "OK: $binaryPath" -ForegroundColor Green
