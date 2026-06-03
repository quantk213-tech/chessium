package main

import (
	"time"

	"github.com/getlantern/systray"
)

func main() {
	appConfig = loadConfig()

	// Apply autostart state from config on first run
	if appConfig.Autostart && !autostartEnabled() {
		setAutostart(true)
	}

	go monitorLoop()

	systray.Run(onReady, onExit)
}

func monitorLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		waWindows := findWhatsAppWindows()
		callActive := isCallActive(waWindows)

		switch {
		case callActive && !isRecording:
			if err := startRecording(appConfig); err != nil {
				logError("startRecording: %v", err)
				showTrayNotification("Recording failed: " + err.Error())
			} else {
				setRecordingState(true)
			}
		case !callActive && isRecording:
			stopRecording()
			setRecordingState(false)
		}
	}
}
