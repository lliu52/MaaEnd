package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	configFilename = "telegram-notifier.json"

	envBotToken            = "MAAEND_TELEGRAM_BOT_TOKEN"
	envChatID              = "MAAEND_TELEGRAM_CHAT_ID"
	envMessageThreadID     = "MAAEND_TELEGRAM_MESSAGE_THREAD_ID"
	envDisableNotification = "MAAEND_TELEGRAM_DISABLE_NOTIFICATION"
	envConfigPath          = "MAAEND_TELEGRAM_CONFIG"
)

// Config contains the Telegram Bot API settings used by the task notifier.
type Config struct {
	BotToken            string `json:"bot_token"`
	ChatID              string `json:"chat_id"`
	MessageThreadID     int64  `json:"message_thread_id"`
	DisableNotification bool   `json:"disable_notification"`
}

func loadConfig() (Config, string, bool, error) {
	if configPath := strings.TrimSpace(os.Getenv(envConfigPath)); configPath != "" {
		cfg, err := loadConfigFile(configPath)
		if err != nil {
			return Config{}, configPath, false, err
		}
		applyEnvironmentOverrides(&cfg)
		return cfg, configPath, true, validateConfig(cfg)
	}

	if hasTelegramEnvironment() {
		cfg := Config{}
		applyEnvironmentOverrides(&cfg)
		return cfg, "environment", true, validateConfig(cfg)
	}

	for _, path := range configCandidates() {
		cfg, err := loadConfigFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Config{}, path, false, err
		}
		applyEnvironmentOverrides(&cfg)
		return cfg, path, true, validateConfig(cfg)
	}

	return Config{}, "", false, nil
}

func loadConfigFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read Telegram notifier config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse Telegram notifier config: %w", err)
	}
	return cfg, nil
}

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.BotToken) == "" {
		return errors.New("Telegram notifier bot_token is required")
	}
	if strings.TrimSpace(cfg.ChatID) == "" {
		return errors.New("Telegram notifier chat_id is required")
	}
	if cfg.MessageThreadID < 0 {
		return errors.New("Telegram notifier message_thread_id cannot be negative")
	}
	return nil
}

func hasTelegramEnvironment() bool {
	return strings.TrimSpace(os.Getenv(envBotToken)) != "" || strings.TrimSpace(os.Getenv(envChatID)) != ""
}

func applyEnvironmentOverrides(cfg *Config) {
	if value := strings.TrimSpace(os.Getenv(envBotToken)); value != "" {
		cfg.BotToken = value
	}
	if value := strings.TrimSpace(os.Getenv(envChatID)); value != "" {
		cfg.ChatID = value
	}
	if value := strings.TrimSpace(os.Getenv(envMessageThreadID)); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			cfg.MessageThreadID = parsed
		}
	}
	if value := strings.TrimSpace(os.Getenv(envDisableNotification)); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			cfg.DisableNotification = parsed
		}
	}
}

func configCandidates() []string {
	seen := make(map[string]struct{})
	paths := make([]string, 0, 3)
	appendPath := func(path string) {
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	if cwd, err := os.Getwd(); err == nil {
		appendPath(filepath.Join(cwd, configFilename))
	}
	if executable, err := os.Executable(); err == nil {
		agentDir := filepath.Dir(executable)
		appendPath(filepath.Join(agentDir, configFilename))
		appendPath(filepath.Join(filepath.Dir(agentDir), configFilename))
	}

	return paths
}
