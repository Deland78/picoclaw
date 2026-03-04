# Build PicoClaw from source and install to PATH
# Run from anywhere; derives repo root from script location.
# This script is at: picoclaw\picoassist\scripts\build_picoclaw.ps1

$ErrorActionPreference = "Stop"

$picoassistDir = Split-Path $PSScriptRoot -Parent    # picoclaw\picoassist\
$repo          = Split-Path $picoassistDir -Parent   # picoclaw\ (repo root)
$dest          = "$env:USERPROFILE\bin\picoclaw.exe"

Write-Host "Pulling latest..."
git -C $repo pull

Write-Host "Copying workspace embed..."
Copy-Item -Recurse -Force "$repo\workspace" "$repo\cmd\picoclaw\workspace"

Write-Host "Building..."
& go build -o "$repo\build\picoclaw.exe" "$repo\cmd\picoclaw"

Write-Host "Installing to $dest..."
Copy-Item -Force "$repo\build\picoclaw.exe" $dest

Write-Host "Done."
picoclaw --version
