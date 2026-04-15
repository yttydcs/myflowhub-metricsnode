# 本脚本承载 MetricsNode 中与 `build_aar` 相关的构建/验证流程。

param(
  [string]$Target = 'android/arm64,android/arm,android/amd64,android/386',
  [string]$JavaPkg = 'com.myflowhub.gomobile',
  [string]$OutFile = 'android/app/libs/myflowhub.aar',
  [int]$AndroidApi = 26
)

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path | Split-Path -Parent
$moduleDir = Join-Path $repoRoot 'nodemobile'
$outPath = Join-Path $repoRoot $OutFile

Write-Host "Build AAR via gomobile" -ForegroundColor Cyan
Write-Host "  RepoRoot  : $repoRoot"
Write-Host "  ModuleDir : $moduleDir"
Write-Host "  Target    : $Target"
Write-Host "  AndroidApi: $AndroidApi"
Write-Host "  JavaPkg   : $JavaPkg"
Write-Host "  OutFile   : $outPath"

if (-not (Test-Path $moduleDir)) {
  throw "nodemobile module not found: $moduleDir"
}

New-Item -ItemType Directory -Force -Path (Split-Path -Parent $outPath) | Out-Null

if (-not (Get-Command gomobile -ErrorAction SilentlyContinue)) {
  Write-Host "gomobile not found, installing..." -ForegroundColor Yellow
  go install golang.org/x/mobile/cmd/gomobile@latest
}

$env:GOWORK = 'off'

Push-Location $moduleDir
try {
  Write-Host "Running: gomobile init" -ForegroundColor Cyan
  gomobile init
  if ($LASTEXITCODE -ne 0) {
    throw "gomobile init failed (exit=$LASTEXITCODE). Please ensure Android SDK/NDK is installed and ANDROID_HOME is set."
  }

  Write-Host "Running: gomobile bind" -ForegroundColor Cyan
  gomobile bind -target $Target -androidapi $AndroidApi -javapkg $JavaPkg -o $outPath .
  if ($LASTEXITCODE -ne 0) {
    throw "gomobile bind failed (exit=$LASTEXITCODE). Please ensure Android SDK/NDK is installed and ANDROID_HOME is set."
  }
} finally {
  Pop-Location
}

if (-not (Test-Path $outPath)) {
  throw "AAR not generated: $outPath"
}

Write-Host "OK: $outPath" -ForegroundColor Green

