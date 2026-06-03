package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/getlantern/systray"
	"golang.org/x/sys/windows/registry"
)

const registryValueName = "WhatsAppRecorder"

// Minimal valid 16x16 ICO — grey mic (idle)
var iconIdle = buildIco(0x80, 0x80, 0x80)

// Minimal valid 16x16 ICO — red circle (recording)
var iconRecording = buildIco(0xFF, 0x00, 0x00)

// buildIco creates a minimal 16x16 1-image ICO filled with one RGB color.
func buildIco(r, g, b byte) []byte {
	// BMP DIB header for 16x16 32bpp (BITMAPINFOHEADER = 40 bytes)
	// Pixel data: 16*16*4 = 1024 bytes, XOR mask + AND mask
	const (
		w, h    = 16, 16
		bpp     = 32
		dibSize = 40
		pixSize = w * h * 4
		andSize = w * h / 8 // AND mask (1 bit per pixel, aligned to DWORD per row)
	)

	// Total BMP size (no palette for 32bpp)
	bmpSize := dibSize + pixSize + andSize

	ico := make([]byte, 0, 6+16+bmpSize)

	// ICONDIR
	ico = append(ico, 0, 0) // reserved
	ico = append(ico, 1, 0) // type=1 (icon)
	ico = append(ico, 1, 0) // count=1

	// ICONDIRENTRY
	ico = append(ico, w)        // width
	ico = append(ico, h)        // height
	ico = append(ico, 0)        // color count (0 = no palette)
	ico = append(ico, 0)        // reserved
	ico = append(ico, 1, 0)     // planes
	ico = append(ico, bpp, 0)   // bits per pixel
	sz := uint32(bmpSize)
	ico = appendU32(ico, sz)    // size of image data
	ico = appendU32(ico, 22)    // offset to image data (6 + 16)

	// BITMAPINFOHEADER
	ico = appendU32(ico, dibSize)       // biSize
	ico = appendI32(ico, w)             // biWidth
	ico = appendI32(ico, h*2)           // biHeight (XOR+AND stacked)
	ico = appendU16(ico, 1)             // biPlanes
	ico = appendU16(ico, bpp)           // biBitCount
	ico = appendU32(ico, 0)             // biCompression = BI_RGB
	ico = appendU32(ico, uint32(pixSize+andSize)) // biSizeImage
	ico = appendI32(ico, 0)             // biXPelsPerMeter
	ico = appendI32(ico, 0)             // biYPelsPerMeter
	ico = appendU32(ico, 0)             // biClrUsed
	ico = appendU32(ico, 0)             // biClrImportant

	// XOR pixel data (BGRA, bottom-up rows)
	for row := 0; row < h; row++ {
		for col := 0; col < w; col++ {
			ico = append(ico, b, g, r, 0xFF) // BGRA, fully opaque
		}
	}

	// AND mask (all zeros = fully visible)
	for i := 0; i < andSize; i++ {
		ico = append(ico, 0)
	}

	return ico
}

func appendU16(b []byte, v uint16) []byte {
	return append(b, byte(v), byte(v>>8))
}
func appendU32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
func appendI32(b []byte, v int) []byte {
	return appendU32(b, uint32(int32(v)))
}

// ---- tray state ----

var (
	mStatus   *systray.MenuItem
	mOpen     *systray.MenuItem
	mSettings *systray.MenuItem
	mAutostart *systray.MenuItem
	mExit     *systray.MenuItem

	appConfig Config
)

func onReady() {
	systray.SetIcon(iconIdle)
	systray.SetTooltip("WhatsApp Recorder — Idle")

	mStatus = systray.AddMenuItem("○ Idle", "Current state")
	mStatus.Disable()

	systray.AddSeparator()

	mOpen = systray.AddMenuItem("Open Recordings Folder", "Open the recordings folder")
	mSettings = systray.AddMenuItem("Settings...", "Open config.json in editor")

	systray.AddSeparator()

	mAutostart = systray.AddMenuItem("Start with Windows", "Toggle autostart")
	refreshAutostartCheck()

	systray.AddSeparator()

	mExit = systray.AddMenuItem("Exit", "Quit WhatsApp Recorder")

	go handleMenu()
}

func handleMenu() {
	for {
		select {
		case <-mOpen.ClickedCh:
			openFolder(appConfig.OutputFolder)
		case <-mSettings.ClickedCh:
			openEditor(configPath)
		case <-mAutostart.ClickedCh:
			toggleAutostart()
		case <-mExit.ClickedCh:
			stopRecording()
			systray.Quit()
		}
	}
}

func onExit() {}

func setRecordingState(recording bool) {
	if recording {
		systray.SetIcon(iconRecording)
		systray.SetTooltip("WhatsApp Recorder — Recording")
		mStatus.SetTitle("● Recording...")
	} else {
		systray.SetIcon(iconIdle)
		systray.SetTooltip("WhatsApp Recorder — Idle")
		mStatus.SetTitle("○ Idle")
	}
}

func openFolder(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		logError("openFolder mkdir: %v", err)
		return
	}
	exec.Command("explorer", path).Start()
}

func openEditor(path string) {
	exec.Command("cmd", "/c", "start", "", path).Start()
}

// ---- autostart ----

func autostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(registryValueName)
	return err == nil
}

func setAutostart(enable bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.SET_VALUE)
	if err != nil {
		logError("autostart open key: %v", err)
		return
	}
	defer k.Close()

	if enable {
		exe, _ := os.Executable()
		if err := k.SetStringValue(registryValueName, exe); err != nil {
			logError("autostart set value: %v", err)
		}
	} else {
		_ = k.DeleteValue(registryValueName)
	}
}

func refreshAutostartCheck() {
	if autostartEnabled() {
		mAutostart.Check()
	} else {
		mAutostart.Uncheck()
	}
}

func toggleAutostart() {
	enabled := autostartEnabled()
	setAutostart(!enabled)
	appConfig.Autostart = !enabled
	saveConfig(appConfig)
	refreshAutostartCheck()
}

// ---- ffmpeg missing ----

func showFfmpegMissingError() {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	msg := "ffmpeg.exe not found next to " + dir + "\n\nDownloading ffmpeg..."
	exec.Command("cmd", "/c", "msg", "*", msg).Start()
	exec.Command("cmd", "/c", "start", "", "https://www.gyan.dev/ffmpeg/builds/").Start()
}

// ---- tray notification ----

func showTrayNotification(msg string) {
	logError("notification: %s", msg)
}
