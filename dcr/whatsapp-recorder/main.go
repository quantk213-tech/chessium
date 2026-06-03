package main

import (
	"time"

	"github.com/getlantern/systray"
)

var trayReady = make(chan struct{})

func main() {
	appConfig = loadConfig()
	if appConfig.Autostart && !autostartEnabled() {
		setAutostart(true)
	}
	go monitorLoop()
	systray.Run(onReady, onExit)
}

func monitorLoop() {
	<-trayReady

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		callActive := detectWhatsAppCall()

		switch {
		case callActive && !isRecording:
			if err := startRecording(appConfig); err != nil {
				logError("startRecording: %v", err)
				notify("Ошибка записи", err.Error())
			} else {
				setRecordingState(true)
				notify("Запись началась", "WhatsApp звонок записывается")
			}
		case !callActive && isRecording:
			file := lastRecordingFile
			stopRecording()
			setRecordingState(false)
			notify("Запись остановлена", "Файл сохранён:\n"+file)
		}
	}
}
