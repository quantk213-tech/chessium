package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/getlantern/systray"
	"golang.org/x/sys/windows/registry"
)

const registryValueName = "WhatsAppRecorder"

var iconIdle = buildIco(0x80, 0x80, 0x80)
var iconRecording = buildIco(0xCC, 0x00, 0x00)

func buildIco(r, g, b byte) []byte {
	const (
		w, h    = 16, 16
		bpp     = 32
		dibSize = 40
		pixSize = w * h * 4
		andSize = w * h / 8
	)
	bmpSize := dibSize + pixSize + andSize
	ico := make([]byte, 0, 6+16+bmpSize)
	ico = append(ico, 0, 0, 1, 0, 1, 0)
	ico = append(ico, w, h, 0, 0, 1, 0, bpp, 0)
	ico = appendU32(ico, uint32(bmpSize))
	ico = appendU32(ico, 22)
	ico = appendU32(ico, dibSize)
	ico = appendI32(ico, w)
	ico = appendI32(ico, h*2)
	ico = appendU16(ico, 1)
	ico = appendU16(ico, bpp)
	ico = appendU32(ico, 0)
	ico = appendU32(ico, uint32(pixSize+andSize))
	ico = appendI32(ico, 0)
	ico = appendI32(ico, 0)
	ico = appendU32(ico, 0)
	ico = appendU32(ico, 0)
	for row := 0; row < h; row++ {
		for col := 0; col < w; col++ {
			ico = append(ico, b, g, r, 0xFF)
		}
	}
	for i := 0; i < andSize; i++ {
		ico = append(ico, 0)
	}
	return ico
}

func appendU16(b []byte, v uint16) []byte { return append(b, byte(v), byte(v>>8)) }
func appendU32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
func appendI32(b []byte, v int) []byte { return appendU32(b, uint32(int32(v))) }

var (
	mStatus    *systray.MenuItem
	mOpen      *systray.MenuItem
	mSettings  *systray.MenuItem
	mAutostart *systray.MenuItem
	mExit      *systray.MenuItem
	appConfig  Config
)

func onReady() {
	systray.SetIcon(iconIdle)
	systray.SetTooltip("WhatsApp Recorder — ожидание")

	mStatus = systray.AddMenuItem("○ Ожидание", "Текущий статус")
	mStatus.Disable()

	systray.AddSeparator()
	mOpen = systray.AddMenuItem("Открыть папку с записями", "")
	mSettings = systray.AddMenuItem("Настройки...", "Открыть config.json")
	systray.AddSeparator()
	mAutostart = systray.AddMenuItem("Запускать с Windows", "")
	refreshAutostartCheck()
	systray.AddSeparator()
	mExit = systray.AddMenuItem("Выход", "")

	close(trayReady) // сигнал — трей готов, можно запускать монитор

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
	if mStatus == nil {
		return
	}
	if recording {
		systray.SetIcon(iconRecording)
		systray.SetTooltip("WhatsApp Recorder — запись идёт")
		mStatus.SetTitle("● Запись...")
		mStatus.Enable()
	} else {
		systray.SetIcon(iconIdle)
		systray.SetTooltip("WhatsApp Recorder — ожидание")
		mStatus.SetTitle("○ Ожидание")
		mStatus.Disable()
	}
}

// notify показывает всплывающее уведомление Windows через PowerShell
func notify(title, message string) {
	// Экранируем одиночные кавычки для PowerShell
	t := strings.ReplaceAll(title, "'", "''")
	m := strings.ReplaceAll(message, "'", "''")
	script := fmt.Sprintf(`
$ErrorActionPreference='SilentlyContinue'
[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime]|Out-Null
$xml=[Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent(
    [Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$nodes=$xml.GetElementsByTagName('text')
$nodes[0].AppendChild($xml.CreateTextNode('%s'))|Out-Null
$nodes[1].AppendChild($xml.CreateTextNode('%s'))|Out-Null
$toast=[Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('WhatsApp Recorder').Show($toast)
`, t, m)
	exec.Command("powershell", "-WindowStyle", "Hidden", "-NonInteractive", "-Command", script).Start()
}

func openFolder(path string) {
	os.MkdirAll(path, 0755)
	exec.Command("explorer", path).Start()
}

func openEditor(path string) {
	exec.Command("cmd", "/c", "start", "", path).Start()
}

func autostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(registryValueName)
	return err == nil
}

func setAutostart(enable bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		logError("autostart open key: %v", err)
		return
	}
	defer k.Close()
	if enable {
		exe, _ := os.Executable()
		k.SetStringValue(registryValueName, exe)
	} else {
		k.DeleteValue(registryValueName)
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

func showFfmpegMissingError() {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	exec.Command("powershell", "-WindowStyle", "Hidden", "-Command",
		fmt.Sprintf(`[System.Windows.MessageBox]::Show('ffmpeg.exe не найден в папке %s. Сейчас откроется страница загрузки.','WhatsApp Recorder')`, dir)).Start()
	exec.Command("cmd", "/c", "start", "", "https://www.gyan.dev/ffmpeg/builds/").Start()
}
