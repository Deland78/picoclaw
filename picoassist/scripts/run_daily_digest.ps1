# Load environment from .env
$envFile = Join-Path $PSScriptRoot ".." ".env"
if (Test-Path $envFile) {
    Get-Content $envFile | ForEach-Object {
        if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
            [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim(), "Process")
        }
    }
}

$date = Get-Date -Format "yyyy-MM-dd"
$runsDir = Join-Path $PSScriptRoot ".." "data" "runs" $date

# Ensure output directory exists
New-Item -ItemType Directory -Force -Path $runsDir | Out-Null

# Run digest
$repoRoot = Split-Path $PSScriptRoot -Parent
python (Join-Path $repoRoot "digest_runner.py")

Write-Host "Digest complete. Output: $runsDir"
