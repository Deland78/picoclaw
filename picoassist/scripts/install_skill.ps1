# Install or update the picoassist skill into ~/.picoclaw workspace
# Run this once after cloning, and again whenever you edit picoassist\skill\SKILL.md
$src  = Join-Path $PSScriptRoot "..\skill"
$dest = "$env:USERPROFILE\.picoclaw\workspace\skills\picoassist"
New-Item -ItemType Directory -Force $dest | Out-Null
Copy-Item -Recurse -Force "$src\*" $dest
Write-Host "Skill installed to $dest"
