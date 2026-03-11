# Start PicoAssist FastAPI services in background windows
# Usage: pwsh scripts\start_services.ps1

$repoRoot = Split-Path $PSScriptRoot -Parent
$envFile = Join-Path $repoRoot ".env"
$logDir = Join-Path $repoRoot "logs"

# Ensure log directory exists
New-Item -ItemType Directory -Force $logDir | Out-Null

# Load .env into the current process environment
if (Test-Path $envFile) {
    Get-Content $envFile | ForEach-Object {
        if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
            [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim(), "Process")
        }
    }
    Write-Host "Loaded environment from $envFile"
}

$mailLog = Join-Path $logDir "mail_worker.log"
$browserLog = Join-Path $logDir "browser_worker.log"
$linkedinLog = Join-Path $logDir "linkedin_worker.log"

# Start mail_worker (port 8001)
Start-Process python `
    -ArgumentList "-m services.mail_worker.app" `
    -WorkingDirectory $repoRoot `
    -NoNewWindow `
    -RedirectStandardOutput $mailLog `
    -RedirectStandardError (Join-Path $logDir "mail_worker_err.log") `
    -PassThru | Out-Null

# Start browser_worker (port 8002)
Start-Process python `
    -ArgumentList "-m services.browser_worker.app" `
    -WorkingDirectory $repoRoot `
    -NoNewWindow `
    -RedirectStandardOutput $browserLog `
    -RedirectStandardError (Join-Path $logDir "browser_worker_err.log") `
    -PassThru | Out-Null

# Start linkedin_worker (port 8003)
Start-Process python `
    -ArgumentList "-m services.linkedin_worker.app" `
    -WorkingDirectory $repoRoot `
    -NoNewWindow `
    -RedirectStandardOutput $linkedinLog `
    -RedirectStandardError (Join-Path $logDir "linkedin_worker_err.log") `
    -PassThru | Out-Null

# Kill any existing picoclaw process before starting fresh
Get-Process picoclaw -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 2

# Start PicoClaw gateway (Telegram + other channels)
$picoExe = "$env:USERPROFILE\bin\picoclaw.exe"
$picoLog = Join-Path $logDir "picoclaw_gateway.log"
Start-Process $picoExe `
    -ArgumentList "gateway" `
    -NoNewWindow `
    -RedirectStandardOutput $picoLog `
    -RedirectStandardError (Join-Path $logDir "picoclaw_gateway_err.log") `
    -PassThru | Out-Null

Write-Host "PicoAssist services starting on ports 8001, 8002, and 8003, PicoClaw gateway starting..."
Write-Host "Logs: $logDir"
Start-Sleep -Seconds 3

Write-Host "`n--- mail_worker health (port 8001) ---"
try {
    Invoke-RestMethod http://localhost:8001/health | ConvertTo-Json
} catch {
    Write-Warning "mail_worker not responding: $_"
}

Write-Host "`n--- browser_worker health (port 8002) ---"
try {
    Invoke-RestMethod http://localhost:8002/health | ConvertTo-Json
} catch {
    Write-Warning "browser_worker not responding: $_"
}

Write-Host "`n--- linkedin_worker health (port 8003) ---"
try {
    Invoke-RestMethod http://localhost:8003/health | ConvertTo-Json
} catch {
    Write-Warning "linkedin_worker not responding: $_"
}
