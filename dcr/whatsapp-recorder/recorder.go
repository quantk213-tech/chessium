package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	ffmpegCmd   *exec.Cmd
	ffmpegStdin io.WriteCloser
	isRecording bool
	logFile     *os.File
)

func ffmpegPath() string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "ffmpeg.exe")
}

func detectDevices() (mic, loopback string) {
	ff := ffmpegPath()
	cmd := exec.Command(ff, "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	out, _ := cmd.CombinedOutput()
	lines := strings.Split(string(out), "\n")

	var dshowSection bool
	var wasapiSection bool
	for _, line := range lines {
		if strings.Contains(line, "DirectShow") {
			dshowSection = true
		}
		if strings.Contains(line, "WASAPI") || strings.Contains(line, "wasapi") {
			wasapiSection = true
			dshowSection = false
		}
		if dshowSection && strings.Contains(line, "(audio)") {
			if mic == "" {
				mic = extractDeviceName(line)
			}
		}
		_ = wasapiSection
	}

	// Detect loopback: list wasapi devices
	cmd2 := exec.Command(ff, "-f", "wasapi", "-list_devices", "true", "-i", "dummy")
	out2, _ := cmd2.CombinedOutput()
	lines2 := strings.Split(string(out2), "\n")
	for _, line := range lines2 {
		if strings.Contains(line, "Stereo Mix") || strings.Contains(line, "What U Hear") ||
			strings.Contains(line, "Loopback") || strings.Contains(line, "WASAPI") {
			if loopback == "" {
				loopback = extractDeviceIndex(line)
			}
		}
	}
	return
}

func extractDeviceName(line string) string {
	start := strings.Index(line, `"`)
	if start < 0 {
		return ""
	}
	end := strings.LastIndex(line, `"`)
	if end <= start {
		return ""
	}
	return line[start+1 : end]
}

func extractDeviceIndex(line string) string {
	// For wasapi loopback we use device index like "0"
	// Try to parse a leading number from lines like "  [0] Some Device"
	trimmed := strings.TrimSpace(line)
	if len(trimmed) > 3 && trimmed[0] == '[' {
		end := strings.Index(trimmed, "]")
		if end > 1 {
			return trimmed[1:end]
		}
	}
	return ""
}

func startRecording(cfg Config) error {
	if isRecording {
		return nil
	}

	ff := ffmpegPath()
	if _, err := os.Stat(ff); os.IsNotExist(err) {
		showFfmpegMissingError()
		return fmt.Errorf("ffmpeg.exe not found")
	}

	mic := cfg.MicDevice
	loopback := cfg.LoopbackDevice

	if mic == "" || loopback == "" {
		detectedMic, detectedLoopback := detectDevices()
		if mic == "" {
			mic = detectedMic
		}
		if loopback == "" {
			loopback = detectedLoopback
		}
	}

	if err := os.MkdirAll(cfg.OutputFolder, 0755); err != nil {
		return fmt.Errorf("create output folder: %w", err)
	}

	ts := time.Now().Format("2006-01-02_15-04-05")
	outFile := filepath.Join(cfg.OutputFolder, ts+".mp3")

	var args []string

	if mic != "" {
		args = append(args,
			"-f", "dshow",
			"-i", fmt.Sprintf("audio=%s", mic),
		)
	}

	// loopback capture
	if loopback != "" {
		// numeric index → use wasapi with device index
		args = append(args,
			"-f", "wasapi",
			"-loopback",
			"-i", loopback,
		)
	} else {
		// fallback: wasapi loopback on default device
		args = append(args,
			"-f", "wasapi",
			"-loopback",
			"-i", "default",
		)
	}

	inputCount := 0
	if mic != "" {
		inputCount++
	}
	inputCount++ // loopback always present

	if inputCount == 2 {
		args = append(args,
			"-filter_complex", "amix=inputs=2:duration=first",
		)
	}

	args = append(args,
		"-ac", "1",
		"-ar", "44100",
		outFile,
	)

	ffmpegCmd = exec.Command(ff, args...)
	var err error
	ffmpegStdin, err = ffmpegCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	ffmpegCmd.Stdout = getLogWriter()
	ffmpegCmd.Stderr = getLogWriter()

	if err := ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}

	isRecording = true
	logInfo("Recording started: %s", outFile)
	return nil
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
	logInfo("Recording stopped")
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
	bw := bufio.NewWriter(w)
	fmt.Fprintf(bw, "[%s] INFO  "+format+"\n", append([]interface{}{time.Now().Format("2006-01-02 15:04:05")}, args...)...)
	bw.Flush()
}

func logError(format string, args ...interface{}) {
	w := getLogWriter()
	bw := bufio.NewWriter(w)
	fmt.Fprintf(bw, "[%s] ERROR "+format+"\n", append([]interface{}{time.Now().Format("2006-01-02 15:04:05")}, args...)...)
	bw.Flush()
}
