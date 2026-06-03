package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	OutputFolder   string `json:"output_folder"`
	MicDevice      string `json:"mic_device"`
	LoopbackDevice string `json:"loopback_device"`
	Autostart      bool   `json:"autostart"`
}

var configPath string

func defaultOutputFolder() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "WhatsApp Calls"
	}
	return filepath.Join(home, "Documents", "WhatsApp Calls")
}

func loadConfig() Config {
	exe, err := os.Executable()
	if err != nil {
		exe = "."
	}
	configPath = filepath.Join(filepath.Dir(exe), "config.json")

	cfg := Config{
		OutputFolder: defaultOutputFolder(),
		Autostart:    true,
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		saveConfig(cfg)
		return cfg
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg
	}
	return cfg
}

func saveConfig(cfg Config) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		logError("saveConfig marshal: %v", err)
		return
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		logError("saveConfig write: %v", err)
	}
}
