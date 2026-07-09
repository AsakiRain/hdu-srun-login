package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfigReadsSingleAccount(t *testing.T) {
	path := writeTempConfig(t, `
username: "user1"
password: "pass1"
check_interval: "30s"
network_detection: true
log:
  flush: true
`)

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig error = %v", err)
	}
	if cfg.Username != "user1" || cfg.Password != "pass1" {
		t.Fatalf("auth = %q/%q, want user1/pass1", cfg.Username, cfg.Password)
	}
	if cfg.CheckInterval != "30s" {
		t.Fatalf("check interval = %q, want 30s", cfg.CheckInterval)
	}
	if !cfg.NetworkDetection {
		t.Fatal("network detection = false, want true")
	}
	if !cfg.Log.Flush {
		t.Fatal("log flush = false, want true")
	}
}

func TestDefaultConfigDisablesNetworkDetection(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.NetworkDetection {
		t.Fatal("network detection default = true, want false")
	}
}

func TestLoadConfigRequiresSingleAccount(t *testing.T) {
	path := writeTempConfig(t, `
check_interval: "30s"
`)

	if _, err := loadConfig(path); err == nil {
		t.Fatal("loadConfig error = nil, want missing username/password error")
	}
}

func TestLoadConfigRejectsLegacyAccountsOnly(t *testing.T) {
	path := writeTempConfig(t, `
accounts:
  - username: "user1"
    password: "pass1"
`)

	if _, err := loadConfig(path); err == nil {
		t.Fatal("loadConfig error = nil, want legacy accounts-only config to be rejected")
	}
}

func TestRunWithContextLogsInvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "app.log")
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
log:
  log_file: "`+filepath.ToSlash(logPath)+`"
  flush: true
`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := runWithContext(context.Background(), configPath, false, 0, 0, "")
	if err == nil {
		t.Fatal("runWithContext error = nil, want invalid config error")
	}

	content, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read app log: %v", readErr)
	}
	output := string(content)
	if !strings.Contains(output, "invalid config") || !strings.Contains(output, "username and password") {
		t.Fatalf("app log = %q, want invalid config details", output)
	}
}
