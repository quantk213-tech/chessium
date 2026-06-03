package main

import (
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
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

func enumWindows() []windowInfo {
	var results []windowInfo
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		vis, _, _ := procIsWindowVisible.Call(hwnd)
		if vis == 0 {
			return 1
		}
		var pid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		buf := make([]uint16, 512)
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		title := syscall.UTF16ToString(buf)
		results = append(results, windowInfo{title: title, pid: pid})
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return results
}

func findWhatsAppWindows() []windowInfo {
	all := enumWindows()
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
		logInfo("WhatsApp windows: %s", strings.Join(titles, ", "))
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

func isCallActive(wins []windowInfo) bool {
	if len(wins) > 1 {
		return true
	}
	for _, w := range wins {
		if w.title != "" && w.title != "WhatsApp" {
			return true
		}
	}
	return false
}
