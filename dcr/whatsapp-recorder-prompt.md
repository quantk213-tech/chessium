# Claude Code Prompt — WhatsApp Call Recorder

Paste this entire prompt into Claude Code.

---

Build a Windows tray application in Go that automatically records WhatsApp Desktop calls.

## Requirements

### Core behavior
- Runs as a background process, sits in system tray (no main window)
- Every 500ms enumerate all windows via EnumWindows (user32.dll)
- Find all windows belonging to WhatsApp.exe process
- Call detection logic:
  - Main WhatsApp window title = "WhatsApp"
  - A call is active when ANY window belonging to WhatsApp has a title that differs from "WhatsApp" (e.g. contact name, "Voice call", "Video call", etc.)
  - OR when more than one WhatsApp window exists simultaneously
- On call start → launch ffmpeg.exe recording
- On call end → gracefully stop ffmpeg (write "q\n" to its stdin)

### Audio recording
Use ffmpeg with both audio sources mixed:
```
ffmpeg.exe -f dshow -i audio="<mic_device>" -f wasapi -loopback -i <loopback> -filter_complex amix=inputs=2:duration=first -ac 1 -ar 44100 "output_path\YYYY-MM-DD_HH-MM-SS.mp3"
```
- Mic device and loopback device names should be auto-detected at startup using `ffmpeg -list_devices true -f dshow -i dummy`
- If auto-detection fails, fall back to values from config.json
- Output folder default: `%USERPROFILE%\Documents\WhatsApp Calls`

### System tray
- Use github.com/getlantern/systray
- Icon: two states
  - Idle: grey microphone icon (embed as ICO bytes)
  - Recording: red circle icon (embed as ICO bytes)
  - Use simple hardcoded 16x16 ICO bytes if no icon files present — generate minimal valid ICO programmatically
- Tooltip: "WhatsApp Recorder — Idle" / "WhatsApp Recorder — Recording"
- Right-click menu:
  - "● Recording..." (grayed out, shown only when recording) OR "○ Idle" (grayed out)
  - Separator
  - "Open Recordings Folder"
  - "Settings..." → opens a small native dialog (or just opens config.json in default editor via os.StartProcess)
  - Separator
  - "Start with Windows" (checkmark reflects current state, toggles autostart)
  - Separator
  - "Exit"

### Autostart (Windows Registry)
- Registry key: `HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run`
- Value name: `WhatsAppRecorder`
- Value: full path to the exe
- "Start with Windows" menu item reads current state and toggles it
- Use golang.org/x/sys/windows/registry

### Config file
Save `config.json` next to the exe:
```json
{
  "output_folder": "C:\\Users\\username\\Documents\\WhatsApp Calls",
  "mic_device": "",
  "loopback_device": "",
  "autostart": true
}
```
Load on startup, save on any change.

### Error handling
- If ffmpeg.exe not found next to the exe → show a Windows MessageBox error and open a browser to https://www.gyan.dev/ffmpeg/builds/ (ffmpeg download page)
- If recording fails to start → log to `recorder.log` next to exe, show tray notification
- Never crash silently — all errors to log file

### Project structure
```
whatsapp-recorder/
├── go.mod
├── main.go
├── tray.go
├── recorder.go
├── config.go
├── windows_api.go
└── README.md
```

### Build
- `go build -ldflags="-H windowsgui" -o whatsapp-recorder.exe .`
- Target: Windows 10/11 x64
- No CGO
- Dependencies: github.com/getlantern/systray, golang.org/x/sys

### README.md must include
1. How to install: place whatsapp-recorder.exe and ffmpeg.exe in the same folder, run exe
2. Where recordings are saved
3. How to change output folder (tray → Settings)
4. How to uninstall (Exit → delete folder, optionally disable autostart first)
