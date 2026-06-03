# WhatsApp Recorder — установщик
# Запуск: powershell -ExecutionPolicy Bypass -c "irm https://raw.githubusercontent.com/quantk213-tech/chessium/main/dcr/whatsapp-recorder/install.ps1 | iex"

$ErrorActionPreference = "Stop"
$repo   = "quantk213-tech/chessium"
$dest   = "$env:LOCALAPPDATA\WhatsAppRecorder"

Write-Host ""
Write-Host "=== WhatsApp Recorder Installer ===" -ForegroundColor Cyan
Write-Host ""

# Создать папку
New-Item -ItemType Directory -Force $dest | Out-Null

# Скачать файлы из последнего релиза
$base = "https://github.com/$repo/releases/latest/download"

Write-Host "Скачиваю whatsapp-recorder.exe..." -ForegroundColor Yellow
Invoke-WebRequest "$base/whatsapp-recorder.exe" -OutFile "$dest\whatsapp-recorder.exe"

Write-Host "Скачиваю ffmpeg.exe..." -ForegroundColor Yellow
Invoke-WebRequest "$base/ffmpeg.exe" -OutFile "$dest\ffmpeg.exe"

# Автозапуск с Windows
Write-Host "Настраиваю автозапуск..." -ForegroundColor Yellow
$regPath = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
Set-ItemProperty $regPath "WhatsAppRecorder" "$dest\whatsapp-recorder.exe"

# Остановить предыдущий экземпляр если есть
Get-Process "whatsapp-recorder" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

# Запустить
Write-Host "Запускаю..." -ForegroundColor Yellow
Start-Process "$dest\whatsapp-recorder.exe"

Write-Host ""
Write-Host "Готово! Иконка появится в системном трее." -ForegroundColor Green
Write-Host "Записи сохраняются в: $env:USERPROFILE\Documents\WhatsApp Calls" -ForegroundColor Green
Write-Host ""
