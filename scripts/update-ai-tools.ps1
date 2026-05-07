$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir "..")
$Binary = Join-Path $RepoRoot "dist/update-ai-tools.exe"

if (Test-Path $Binary) {
    & $Binary @args
    exit $LASTEXITCODE
}

$Go = Get-Command go -ErrorAction SilentlyContinue
if ($Go) {
    New-Item -ItemType Directory -Force -Path (Join-Path $RepoRoot "dist") | Out-Null
    if (-not $env:GOCACHE) {
        $env:GOCACHE = Join-Path $RepoRoot ".gocache"
    }
    & go build -o $Binary (Join-Path $RepoRoot "cmd/update-ai-tools")
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
    & $Binary @args
    exit $LASTEXITCODE
}

Write-Error "update-ai-tools binary not found and go is not available."
exit 127
