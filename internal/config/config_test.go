package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesEnvOverridesAndDefaults(t *testing.T) {
	t.Setenv("WIKIJS_URL", "https://env.example.test/")
	t.Setenv("WIKIJS_API_TOKEN", "env-token")
	t.Setenv("WIKIJS_DEFAULT_LOCALE", "fr")
	t.Setenv("WIKIJS_DEFAULT_EDITOR", "ckeditor")

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"url":"https://file.example.test","apiToken":"file-token"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, usedPath, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if usedPath != path {
		t.Fatalf("used path = %q, want %q", usedPath, path)
	}
	if cfg.URL != "https://env.example.test" {
		t.Fatalf("URL = %q", cfg.URL)
	}
	if cfg.APIToken != "env-token" || cfg.DefaultLocale != "fr" || cfg.DefaultEditor != "ckeditor" {
		t.Fatalf("env overrides not applied: %+v", cfg)
	}
}

func TestLoadFromEnvironmentWithoutConfigFile(t *testing.T) {
	t.Setenv("WIKIJS_URL", "https://env-only.example.test/")
	t.Setenv("WIKIJS_API_TOKEN", "env-token")

	cfg, usedPath, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if usedPath != "environment" {
		t.Fatalf("used path = %q, want environment", usedPath)
	}
	if cfg.URL != "https://env-only.example.test" || cfg.APIToken != "env-token" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.DefaultLocale != "en" || cfg.DefaultEditor != "markdown" {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
}

func TestLoadMissingConfig(t *testing.T) {
	_, _, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadInvalidConfigMissingField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"apiToken":"token"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := Load(path)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}
