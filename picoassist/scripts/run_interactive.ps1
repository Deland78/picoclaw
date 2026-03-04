param(
    [Parameter(Mandatory)][string]$ClientId,
    [Parameter(Mandatory)][ValidateSet("jira","ado")][string]$App
)

# Load environment
$envFile = Join-Path $PSScriptRoot ".." ".env"
if (Test-Path $envFile) {
    Get-Content $envFile | ForEach-Object {
        if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
            [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim(), "Process")
        }
    }
}

$browserUrl = "http://localhost:$($env:BROWSER_WORKER_PORT ?? '8002')"

Write-Host "Starting browser_worker HTTP API..."
$worker = Start-Process -FilePath "python" -ArgumentList "-m services.browser_worker.app" `
    -WorkingDirectory (Split-Path $PSScriptRoot -Parent) -PassThru -NoNewWindow
Start-Sleep -Seconds 3

try {
    # Start session
    $session = Invoke-RestMethod -Uri "$browserUrl/browser/start_session" -Method POST `
        -ContentType "application/json" -Body (@{client_id=$ClientId; app=$App} | ConvertTo-Json)

    Write-Host "Session started: $($session.session_id)"
    Write-Host "Browser is open. Log in manually if needed, then press Enter..."
    Read-Host

    # Interactive loop
    while ($true) {
        $action = Read-Host "Action name (or 'quit')"
        if ($action -eq 'quit') { break }
        $url = Read-Host "URL to navigate"
        $body = @{
            session_id = $session.session_id
            action_spec = @{action = $action; params = @{url = $url}}
        } | ConvertTo-Json -Depth 5
        $result = Invoke-RestMethod -Uri "$browserUrl/browser/do" -Method POST `
            -ContentType "application/json" -Body $body
        $result | ConvertTo-Json -Depth 5
    }

    # Stop session
    Invoke-RestMethod -Uri "$browserUrl/browser/stop_session" -Method POST `
        -ContentType "application/json" -Body (@{session_id=$session.session_id} | ConvertTo-Json)
    Write-Host "Session stopped."
}
finally {
    Stop-Process -Id $worker.Id -Force -ErrorAction SilentlyContinue
}
