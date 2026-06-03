package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	ffmpegCmd         *exec.Cmd
	ffmpegStdin       io.WriteCloser
	isRecording       bool
	lastRecordingFile string
	logFile           *os.File
)

func ffmpegPath() string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "ffmpeg.exe")
}

// detectMic returns the first DirectShow audio input device name.
func detectMic() string {
	ff := ffmpegPath()
	cmd := exec.Command(ff, "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	out, _ := cmd.CombinedOutput()

	inAudioSection := false
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "DirectShow audio devices") {
			inAudioSection = true
			continue
		}
		if inAudioSection && strings.Contains(line, `"`) {
			name := extractQuoted(line)
			if name != "" {
				logInfo("Detected mic: %s", name)
				return name
			}
		}
	}
	logInfo("No mic device detected via dshow")
	return ""
}

func extractQuoted(line string) string {
	s := strings.Index(line, `"`)
	if s < 0 {
		return ""
	}
	e := strings.LastIndex(line, `"`)
	if e <= s {
		return ""
	}
	return line[s+1 : e]
}

func startRecording(cfg Config) error {
	if isRecording {
		return nil
	}

	ff := ffmpegPath()
	if _, err := os.Stat(ff); os.IsNotExist(err) {
		showFfmpegMissingError()
		return fmt.Errorf("ffmpeg.exe не найден рядом с программой")
	}

	mic := cfg.MicDevice
	if mic == "" {
		mic = detectMic()
	}

	if err := os.MkdirAll(cfg.OutputFolder, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку: %w", err)
	}

	ts := time.Now().Format("2006-01-02_15-04-05")
	outFile := filepath.Join(cfg.OutputFolder, ts+".mp3")
	lastRecordingFile = outFile

	args := buildArgs(mic, outFile)
	logInfo("ffmpeg args: %v", args)

	ffmpegCmd = exec.Command(ff, args...)
	var err error
	ffmpegStdin, err = ffmpegCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	lw := getLogWriter()
	ffmpegCmd.Stdout = lw
	ffmpegCmd.Stderr = lw

	if err := ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg не запустился: %w", err)
	}

	isRecording = true
	logInfo("Recording started: %s", outFile)
	return nil
}

func buildArgs(mic, outFile string) []string {
	var args []string
	inputs := 0

	if mic != "" {
		args = append(args, "-f", "dshow", "-i", "audio="+mic)
		inputs++
	}

	// WASAPI loopback — захват системного звука с дефолтного выхода
	args = append(args, "-f", "wasapi", "-loopback", "-i", "default")
	inputs++

	if inputs == 2 {
		args = append(args, "-filter_complex", "amix=inputs=2:duration=first")
	}

	args = append(args, "-ac", "1", "-ar", "44100", "-y", outFile)
	return args
}

func stopRecording() {
	if !isRecording || ffmpegStdin == nil {
		return
	}
	_, _ = fmt.Fprint(ffmpegStdin, "q\n")
	_ = ffmpegStdin.Close()
	_ = ffmpegCmd.Wait()
	ffmpegCmd = nil
	ffmpegStdin = nil
	isRecording = false
	logInfo("Recording stopped: %s", lastRecordingFile)
}

func getLogWriter() io.Writer {
	if logFile != nil {
		return logFile
	}
	exe, _ := os.Executable()
	logPath := filepath.Join(filepath.Dir(exe), "recorder.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return io.Discard
	}
	logFile = f
	return logFile
}

func logInfo(format string, args ...interface{}) {
	w := getLogWriter()
	fmt.Fprintf(w, "[%s] INFO  "+format+"\n",
		append([]interface{}{time.Now().Format("2006-01-02 15:04:05")}, args...)...)
}

func logError(format string, args ...interface{}) {
	w := getLogWriter()
	fmt.Fprintf(w, "[%s] ERROR "+format+"\n",
		append([]interface{}{time.Now().Format("2006-01-02 15:04:05")}, args...)...)
}
