<#
.SYNOPSIS
  Build helper for godisplacementx.

.DESCRIPTION
  Builds the CLI-only binary (pure Go) and/or the full GUI binary (Wails), runs
  tests, regenerates sprites, or cleans build output.

.PARAMETER Target
  One of: all (default), cli, gui, test, lint, sprites, clean.

.EXAMPLE
  .\build.ps1            # cli + gui
  .\build.ps1 gui        # GUI desktop app only
  .\build.ps1 cli        # CLI-only binary (no GUI, cross-compiles cleanly)
  .\build.ps1 test
  .\build.ps1 lint       # golangci-lint, same config CI uses
#>
[CmdletBinding()]
param(
  [ValidateSet('all', 'cli', 'gui', 'test', 'lint', 'sprites', 'clean')]
  [string]$Target = 'all'
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
$binDir = Join-Path $root 'build\bin'

function Write-Step($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }
function Write-Ok($msg) { Write-Host "    $msg" -ForegroundColor Green }

function Require-Cmd($name, $hint) {
  $cmd = Get-Command $name -ErrorAction SilentlyContinue
  if ($null -eq $cmd) {
    throw "'$name' not found on PATH. $hint"
  }
  return $cmd.Source
}

# Resolve the wails CLI, adding GOPATH\bin to PATH if needed.
function Resolve-Wails {
  if (Get-Command wails -ErrorAction SilentlyContinue) { return 'wails' }
  $gopath = (& go env GOPATH).Trim()
  $candidate = Join-Path $gopath 'bin\wails.exe'
  if (Test-Path $candidate) {
    $env:PATH = "$env:PATH;$(Join-Path $gopath 'bin')"
    return $candidate
  }
  throw "'wails' not found. Install it with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
}

function Invoke-Native($file, $arguments, $workdir) {
  Push-Location $workdir
  try {
    & $file @arguments
    if ($LASTEXITCODE -ne 0) {
      throw "$file $($arguments -join ' ') failed (exit $LASTEXITCODE)"
    }
  } finally {
    Pop-Location
  }
}

function Build-CLI {
  Write-Step 'Building CLI-only binary (pure Go, no GUI)'
  Require-Cmd 'go' 'Install Go from https://go.dev/dl/' | Out-Null
  New-Item -ItemType Directory -Force -Path $binDir | Out-Null
  $out = Join-Path $binDir 'godisplacementx-cli.exe'
  Invoke-Native 'go' @('build', '-o', $out, '.') $root
  Write-Ok "built $out"
}

function Build-GUI {
  Write-Step 'Building GUI desktop app (Wails)'
  # The GUI frontend is static HTML + htmx (no Node toolchain), so this is pure Go.
  Require-Cmd 'go' 'Install Go from https://go.dev/dl/' | Out-Null
  $wails = Resolve-Wails
  Invoke-Native $wails @('build') $root
  Write-Ok "built $(Join-Path $binDir 'godisplacementx.exe') (runs the GUI; also works as the CLI)"
}

function Invoke-Tests {
  Write-Step 'Running tests'
  Require-Cmd 'go' 'Install Go from https://go.dev/dl/' | Out-Null
  Invoke-Native 'go' @('test', './...') $root
  Write-Ok 'tests passed'
}

# Resolve the golangci-lint binary, adding GOPATH\bin to PATH if needed.
function Resolve-GolangciLint {
  if (Get-Command golangci-lint -ErrorAction SilentlyContinue) { return 'golangci-lint' }
  $gopath = (& go env GOPATH).Trim()
  $candidate = Join-Path $gopath 'bin\golangci-lint.exe'
  if (Test-Path $candidate) {
    $env:PATH = "$env:PATH;$(Join-Path $gopath 'bin')"
    return $candidate
  }
  throw "'golangci-lint' not found. Install it with: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
}

function Invoke-Lint {
  Write-Step 'Linting (golangci-lint, same config as CI)'
  Require-Cmd 'go' 'Install Go from https://go.dev/dl/' | Out-Null
  $lint = Resolve-GolangciLint
  Invoke-Native $lint @('run', './...') $root
  Write-Ok 'lint passed'
}

function Build-Sprites {
  Write-Step 'Regenerating sprite PNGs from SVGs (resvg)'
  Require-Cmd 'npm' 'Install Node.js from https://nodejs.org/' | Out-Null
  $dir = Join-Path $root 'tools\rasterize-sprites'
  Invoke-Native 'npm' @('install') $dir
  Invoke-Native 'npm' @('run', 'rasterize') $dir
  Write-Ok 'sprites regenerated'
}

function Invoke-Clean {
  Write-Step 'Cleaning build output'
  foreach ($p in @($binDir, (Join-Path $root 'frontend\dist'))) {
    if (Test-Path $p) {
      Remove-Item -Recurse -Force $p
      Write-Ok "removed $p"
    }
  }
}

switch ($Target) {
  'cli'     { Build-CLI }
  'gui'     { Build-GUI }
  'test'    { Invoke-Tests }
  'lint'    { Invoke-Lint }
  'sprites' { Build-Sprites }
  'clean'   { Invoke-Clean }
  'all'     { Build-CLI; Build-GUI }
}

Write-Host ''
Write-Host "Done: $Target" -ForegroundColor Green
