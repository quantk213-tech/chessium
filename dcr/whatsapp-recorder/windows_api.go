package main

import (
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
)

type windowInfo struct {
	title string
	pid   uint32
}

// enumWindowsState передаётся через lParam, чтобы не создавать новый callback каждые 500ms.
type enumWindowsState struct {
	results []windowInfo
}

// cachedCb — создаётся один раз; syscall.NewCallback вызывать в цикле нельзя (пул ~1024).
var cachedCb = syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
	vis, _, _ := procIsWindowVisible.Call(hwnd)
	if vis == 0 {
		return 1
	}
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	buf := make([]uint16, 512)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	title := syscall.UTF16ToString(buf)
	state := (*enumWindowsState)(unsafe.Pointer(lParam))
	state.results = append(state.results, windowInfo{title: title, pid: pid})
	return 1
})

func enumAllWindows() []windowInfo {
	state := &enumWindowsState{}
	procEnumWindows.Call(cachedCb, uintptr(unsafe.Pointer(state)))
	return state.results
}

func findWhatsAppWindows() []windowInfo {
	all := enumAllWindows()
	var wa []windowInfo
	for _, w := range all {
		if isWhatsAppPid(w.pid) {
			wa = append(wa, w)
		}
	}
	if len(wa) > 0 {
		titles := make([]string, len(wa))
		for i, w := range wa {
			titles[i] = `"` + w.title + `"`
		}
		logInfo("WhatsApp windows (%d): %s", len(wa), strings.Join(titles, ", "))
	}
	return wa
}

func isWhatsAppPid(pid uint32) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return false
	}
	path := syscall.UTF16ToString(buf[:size])
	return strings.EqualFold(baseName(path), "WhatsApp.exe")
}

func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '\\' || path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

// detectWhatsAppCall — главная функция определения звонка.
// Использует два метода: окна + микрофон.
func detectWhatsAppCall() bool {
	wins := findWhatsAppWindows()

	// Метод 1: несколько окон WhatsApp
	if len(wins) > 1 {
		logInfo("Call detected: %d windows", len(wins))
		return true
	}
	// Метод 1b: окно с нестандартным заголовком
	for _, w := range wins {
		if w.title != "" && w.title != "WhatsApp" {
			logInfo("Call detected: title=%q", w.title)
			return true
		}
	}

	// Метод 2: WhatsApp захватывает микрофон (работает для overlay-звонков)
	// Проверяем только если WhatsApp вообще запущен
	if len(wins) > 0 && isWhatsAppUsingMic() {
		logInfo("Call detected: mic active")
		return true
	}

	return false
}

// isWhatsAppUsingMic читает реестр Windows Privacy / CapabilityAccessManager.
// LastUsedTimeStop == 0 означает что приложение прямо сейчас использует микрофон.
func isWhatsAppUsingMic() bool {
	bases := []string{
		// Win32 / Electron (не-Store)
		`SOFTWARE\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\microphone\NonPackaged`,
		// Microsoft Store (UWP)
		`SOFTWARE\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\microphone`,
	}
	for _, base := range bases {
		if checkMicInBase(base) {
			return true
		}
	}
	return false
}

func checkMicInBase(base string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, base, registry.READ)
	if err != nil {
		return false
	}
	names, err := k.ReadSubKeyNames(-1)
	k.Close()
	if err != nil {
		return false
	}
	for _, name := range names {
		if !strings.Contains(strings.ToLower(name), "whatsapp") {
			continue
		}
		sub, err := registry.OpenKey(registry.CURRENT_USER, base+`\`+name, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		stop, _, errStop := sub.GetIntegerValue("LastUsedTimeStop")
		start, _, errStart := sub.GetIntegerValue("LastUsedTimeStart")
		sub.Close()
		// stop==0 && start>0 — приложение сейчас держит микрофон открытым
		if errStop == nil && errStart == nil && stop == 0 && start > 0 {
			logInfo("Mic active: %s", name)
			return true
		}
	}
	return false
}
