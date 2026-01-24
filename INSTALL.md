# Installation Guide

## 🚀 Quick Install (Binary Download)

### Linux & macOS

**Install:**
Copy and paste this into your terminal to download and install `scrum` to `/usr/local/bin`:

```bash
# Detect OS and Architecture, download, and install
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
sudo curl -L "https://github.com/atharshah1/scrum/releases/latest/download/scrum-$OS-$ARCH" -o /usr/local/bin/scrum
sudo chmod +x /usr/local/bin/scrum
echo "Installation complete! Run 'scrum --help' to get started."
```

**Uninstall:**
```bash
sudo rm /usr/local/bin/scrum
```

### Windows (PowerShell)

**Install:**
Open PowerShell and paste the following to install `scrum` and add it to your PATH automatically:

```powershell
$installDir = "$env:LOCALAPPDATA\ScrumCLI"
if (!(Test-Path $installDir)) { New-Item -ItemType Directory -Force -Path $installDir }
$url = "https://github.com/atharshah1/scrum/releases/latest/download/scrum-windows-amd64.exe"
Invoke-WebRequest -Uri $url -OutFile "$installDir\scrum.exe"
$currentPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
if ($currentPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$currentPath;$installDir", [EnvironmentVariableTarget]::User)
    Write-Host "Added to PATH. Please restart your terminal to use the 'scrum' command."
} else {
    Write-Host "Installation complete. You can run 'scrum' now."
}
```

**Uninstall:**
```powershell
$installDir = "$env:LOCALAPPDATA\ScrumCLI"
Remove-Item -Recurse -Force $installDir
$currentPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
$newPath = ($currentPath -split ';' | Where-Object { $_ -ne $installDir }) -join ';'
[Environment]::SetEnvironmentVariable("Path", $newPath, [EnvironmentVariableTarget]::User)
Write-Host "Uninstalled successfully."
```