$AppName = "simple-invoice"
$BinName = "$AppName-bin.exe"
$InstallDir = "$env:USERPROFILE\.simple-invoice"
$BinPath = "$env:USERPROFILE\bin"
$Launcher = "$BinPath\$AppName.cmd"

Write-Host "Installing simple-invoice into $InstallDir..."
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item -Recurse -Force . $InstallDir

Push-Location $InstallDir

Write-Host "Building Go binary..."
& go build -o $BinName cmd/server/main.go

Write-Host "Setting up Python virtual environment..."
Set-Location tools
python -m venv venv
& "$InstallDir\tools\venv\Scripts\pip.exe" install -r requirements.txt
Pop-Location

Write-Host "Creating launcher script at $Launcher..."
New-Item -ItemType Directory -Force -Path $BinPath | Out-Null
Set-Content -Path $Launcher -Value "@echo off`ncd /d $InstallDir`n$BinName %*"

# Ensure $BinPath is in PATH
$envPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
if (-not ($envPath -split ";" | Where-Object { $_ -eq $BinPath })) {
    Write-Host "🔧 Adding $BinPath to PATH..."
    [System.Environment]::SetEnvironmentVariable("Path", "$envPath;$BinPath", "User")
    Write-Host "⚠️ Restart your terminal or log out and in again to apply PATH changes."
}

Write-Host "✅ Done! Run 'simple-invoice' from any terminal."

