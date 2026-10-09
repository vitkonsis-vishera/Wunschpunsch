package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AppConfig struct {
	Language        int    `json:"language"`
	Theme           int    `json:"theme"`
	PollIntervalSec int    `json:"poll_interval_sec"`
	SSHUser         string `json:"ssh_user"`
	SSHPort         int    `json:"ssh_port"`
	SSHKeyPath      string `json:"ssh_key_path"`
	SSHPassword     string `json:"ssh_password"`
}

func DefaultConfig() AppConfig {
	return AppConfig{
		Language:        0,
		Theme:           0,
		PollIntervalSec: 2,
		SSHUser:         "postgres",
		SSHPort:         22,
		SSHKeyPath:      "",
		SSHPassword:     "",
	}
}

func GetConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.json"
	}
	dir := filepath.Join(home, ".wunschpunsch")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "config.json")
}

func LoadConfig() AppConfig {
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		cfg := DefaultConfig()
		_ = SaveConfig(cfg)
		return cfg
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig()
	}

	if cfg.SSHUser == "" {
		cfg.SSHUser = "postgres"
	}
	if cfg.SSHPort <= 0 {
		cfg.SSHPort = 22
	}

	return cfg
}

func SaveConfig(cfg AppConfig) error {
	path := GetConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
