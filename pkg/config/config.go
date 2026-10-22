package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

func EncryptSecret(plainText, masterKey string) (string, error) {
	key := []byte(masterKey)
	if len(key) < 32 {
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	} else if len(key) > 32 {
		key = key[:32]
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptSecret(encryptedText, masterKey string) (string, error) {
	key := []byte(masterKey)
	if len(key) < 32 {
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	} else if len(key) > 32 {
		key = key[:32]
	}

	data, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", io.ErrUnexpectedEOF
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plainText, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plainText), nil
}

type ClusterProfile struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	User     string `json:"user"`
}

type AppConfig struct {
	Language        int              `json:"language"`
	Theme           int              `json:"theme"`
	PollIntervalSec int              `json:"poll_interval_sec"`
	SSHUser         string           `json:"ssh_user"`
	SSHPort         int              `json:"ssh_port"`
	SSHKeyPath      string           `json:"ssh_key_path"`
	SSHPassword     string           `json:"ssh_password"`
	Profiles        []ClusterProfile `json:"profiles"`
	ActiveProfile   string           `json:"active_profile"`
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
