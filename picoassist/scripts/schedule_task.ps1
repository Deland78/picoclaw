param(
    [string]$Time = "06:00",
    [string]$TaskName = "PicoAssist-DailyDigest"
)

$scriptPath = Join-Path $PSScriptRoot "run_daily_digest.ps1"
$action = New-ScheduledTaskAction -Execute "pwsh.exe" `
    -Argument "-NonInteractive -File `"$scriptPath`""
$trigger = New-ScheduledTaskTrigger -Daily -At $Time
Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger `
    -Description "PicoAssist daily digest" -Force
Write-Host "Scheduled task '$TaskName' registered to run daily at $Time"
