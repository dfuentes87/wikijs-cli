package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	ErrMissing = errors.New("config file not found")
	ErrInvalid = errors.New("invalid config")
)

type Config struct {
	URL           string       `json:"url"`
	APIToken      string       `json:"apiToken"`
	DefaultEditor string       `json:"defaultEditor"`
	DefaultLocale string       `json:"defaultLocale"`
	AutoSync      AutoSync     `json:"autoSync"`
	Backup        BackupConfig `json:"backup"`
}

type AutoSync struct {
	Path string `json:"path"`
}

type BackupConfig struct {
	Enabled  bool   `json:"enabled"`
	Path     string `json:"path"`
	KeepDays int    `json:"keepDays"`
}

func DefaultPath() string {
	if runtime.GOOS == "windows" {
		base, err := os.UserConfigDir()
		if err == nil && base != "" {
			return filepath.Join(base, "wikijs", "config.json")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".config", "wikijs.json")
	}
	return filepath.Join(home, ".config", "wikijs.json")
}

func Load(path string) (Config, string, error) {
	if envPath := os.Getenv("WIKIJS_CONFIG"); path == "" && envPath != "" {
		path = envPath
	}
	if path == "" {
		path = DefaultPath()
	}
	path = expandPath(path)

	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			applyEnv(&cfg)
			if cfg.URL != "" && cfg.APIToken != "" {
				if err := finalize(&cfg); err != nil {
					return cfg, "environment", err
				}
				return cfg, "environment", nil
			}
			return cfg, path, fmt.Errorf("%w: %s", ErrMissing, path)
		}
		return cfg, path, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, path, fmt.Errorf("%w: parse config: %v", ErrInvalid, err)
	}
	applyEnv(&cfg)
	if err := finalize(&cfg); err != nil {
		return cfg, path, err
	}
	return cfg, path, nil
}

func finalize(cfg *Config) error {
	if cfg.DefaultEditor == "" {
		cfg.DefaultEditor = "markdown"
	}
	if cfg.DefaultLocale == "" {
		cfg.DefaultLocale = "en"
	}
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	if cfg.URL == "" {
		return fmt.Errorf(`%w: missing "url"`, ErrInvalid)
	}
	if cfg.APIToken == "" {
		return fmt.Errorf(`%w: missing "apiToken"`, ErrInvalid)
	}
	if _, err := url.ParseRequestURI(cfg.URL); err != nil {
		return fmt.Errorf("%w: invalid url: %v", ErrInvalid, err)
	}
	cfg.AutoSync.Path = expandPath(cfg.AutoSync.Path)
	cfg.Backup.Path = expandPath(cfg.Backup.Path)
	return nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("WIKIJS_URL"); v != "" {
		cfg.URL = v
	}
	if v := os.Getenv("WIKIJS_API_TOKEN"); v != "" {
		cfg.APIToken = v
	}
	if v := os.Getenv("WIKIJS_DEFAULT_LOCALE"); v != "" {
		cfg.DefaultLocale = v
	}
	if v := os.Getenv("WIKIJS_DEFAULT_EDITOR"); v != "" {
		cfg.DefaultEditor = v
	}
}

func expandPath(path string) string {
	if path == "" || path == "~" {
		if path == "~" {
			if home, err := os.UserHomeDir(); err == nil {
				return home
			}
		}
		return path
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
