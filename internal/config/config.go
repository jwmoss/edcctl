package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	AppName        = "edcctl"
	EnvPrefix      = "EDCCTL"
	DefaultBaseURL = "https://app.gostudiopro.com/online"
	// DefaultAccountID is the Dance Studio Pro account ID for
	// Evolution Dance Complex, extracted from the EDC mobile app.
	DefaultAccountID = "30834"
	ConfigFilename   = "config.yaml"
)

type Config struct {
	BaseURL   string `yaml:"base_url"`
	AccountID string `yaml:"account_id"`
	Email     string `yaml:"email"`
	Password  string `yaml:"password"`
}

func Default() Config {
	return Config{
		BaseURL:   DefaultBaseURL,
		AccountID: DefaultAccountID,
	}
}

func DefaultPath() string {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, AppName, ConfigFilename)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ConfigFilename)
	}
	return filepath.Join(home, ".config", AppName, ConfigFilename)
}

func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		path = DefaultPath()
	}
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse config file %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	applyEnv(&cfg)
	normalize(&cfg)
	return &cfg, nil
}

func Save(path string, cfg Config, overwrite bool) error {
	if path == "" {
		path = DefaultPath()
	}
	normalize(&cfg)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refuse non-regular config path %s", path)
		}
		if !overwrite {
			return fmt.Errorf("config already exists at %s; use --force to overwrite: %w", path, os.ErrExist)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect config file: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return fmt.Errorf("create private config: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write config file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close config file: %w", err)
	}
	if overwrite {
		// Replace the directory entry without a write through a raced symlink.
		err = os.Rename(file.Name(), path)
	} else {
		// Publish only if no process has created the destination.
		err = os.Link(file.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("publish config file: %w", err)
	}
	return nil
}

func (c Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("base URL is required; set --base-url or config file base_url")
	}
	if c.AccountID == "" {
		return fmt.Errorf("account ID is required; set --account-id or config file account_id")
	}
	if c.Email == "" {
		return fmt.Errorf("email is required; set EDCCTL_USERNAME or config file email")
	}
	if c.Password == "" {
		return fmt.Errorf("password is required; set EDCCTL_PASSWORD or config file password")
	}
	return nil
}

func (c Config) Redacted() map[string]string {
	return map[string]string{
		"base_url":   c.BaseURL,
		"account_id": c.AccountID,
		"email":      c.Email,
		"password":   redact(c.Password),
	}
}

// applyEnv reads EDCCTL_* variables and the EDC_LOGIN / EDC_PASSWORD
// variables exported in the user's shell.
func applyEnv(cfg *Config) {
	if value := os.Getenv(EnvPrefix + "_BASE_URL"); value != "" {
		cfg.BaseURL = value
	}
	if value := os.Getenv(EnvPrefix + "_ACCOUNT_ID"); value != "" {
		cfg.AccountID = value
	}
	if value := os.Getenv(EnvPrefix + "_USERNAME"); value != "" {
		cfg.Email = value
	} else if value := os.Getenv("EDC_LOGIN"); value != "" {
		cfg.Email = value
	}
	if value := os.Getenv(EnvPrefix + "_PASSWORD"); value != "" {
		cfg.Password = value
	} else if value := os.Getenv("EDC_PASSWORD"); value != "" {
		cfg.Password = value
	}
}

func normalize(cfg *Config) {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.AccountID != "" {
		if id, err := strconv.Atoi(strings.TrimSpace(cfg.AccountID)); err == nil {
			cfg.AccountID = strconv.Itoa(id)
		} else {
			cfg.AccountID = strings.TrimSpace(cfg.AccountID)
		}
	}
	cfg.Email = strings.TrimSpace(cfg.Email)
}

func redact(value string) string {
	if value == "" {
		return ""
	}
	return "redacted"
}
