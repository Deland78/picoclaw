[Environment]::SetEnvironmentVariable("CLAUDECODE", $null, "Process")
$logDir = "$env:USERPROFILE\PicoAssist\logs"
if (-not (Test-Path $logDir)) { New-Item -ItemType Directory -Path $logDir -Force }
Start-Process -FilePath "C:\Users\david\bin\picoclaw.exe" -ArgumentList "gateway" -RedirectStandardOutput "$logDir\picoclaw_gateway.log" -RedirectStandardError "$logDir\picoclaw_gateway_err.log" -NoNewWindow:$false -WindowStyle Hidden
