package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvrd/hound/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	// Clear env to test defaults
	os.Unsetenv("HOUND_RPC_ENDPOINT")

	cfg := config.DefaultConfig()

	if cfg.RPCEndpoint != config.DefaultRPCEndpoint {
		t.Errorf("RPCEndpoint = %q, want %q", cfg.RPCEndpoint, config.DefaultRPCEndpoint)
	}

	if cfg.DatabasePath == "" {
		t.Error("DatabasePath should not be empty")
	}

	if !strings.HasSuffix(cfg.DatabasePath, "hound.db") {
		t.Errorf("DatabasePath = %q, should end with hound.db", cfg.DatabasePath)
	}
}

func TestDefaultConfigEnvOverride(t *testing.T) {
	customEndpoint := "https://my-custom-rpc.example.com"
	os.Setenv("HOUND_RPC_ENDPOINT", customEndpoint)
	defer os.Unsetenv("HOUND_RPC_ENDPOINT")

	cfg := config.DefaultConfig()

	if cfg.RPCEndpoint != customEndpoint {
		t.Errorf("RPCEndpoint = %q, want %q", cfg.RPCEndpoint, customEndpoint)
	}
}

func TestGetDatabasePath(t *testing.T) {
	path := config.GetDatabasePath()

	if path == "" {
		t.Error("database path should not be empty")
	}

	// Should contain .config/hound/hound.db or be in tmp
	if !strings.Contains(path, filepath.Join(".config", "hound", "hound.db")) &&
		!strings.Contains(path, "hound.db") {
		t.Errorf("unexpected database path: %q", path)
	}
}

func TestGetConfigDir(t *testing.T) {
	dir := config.GetConfigDir()

	if dir == "" {
		t.Error("config dir should not be empty")
	}

	if !strings.HasSuffix(dir, "hound") {
		t.Errorf("config dir = %q, should end with 'hound'", dir)
	}
}

func TestEnsureConfigDir(t *testing.T) {
	// Use a temp directory to avoid polluting the real config
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test-hound-config")

	// Temporarily override HOME to control the path
	// We can't easily do this without changing the function, so just verify
	// the real function doesn't error on an existing directory
	err := config.EnsureConfigDir()
	if err != nil {
		t.Errorf("EnsureConfigDir() error = %v", err)
	}

	// Verify the directory exists
	dir := config.GetConfigDir()
	info, err := os.Stat(dir)
	if err != nil {
		t.Errorf("config dir should exist after EnsureConfigDir: %v", err)
	}
	if !info.IsDir() {
		t.Error("config dir should be a directory")
	}

	_ = testDir // suppress unused
}

func TestConstants(t *testing.T) {
	if config.DefaultRPCEndpoint != "https://api.mainnet-beta.solana.com" {
		t.Errorf("DefaultRPCEndpoint = %q", config.DefaultRPCEndpoint)
	}
	if config.ConfigDirName != "hound" {
		t.Errorf("ConfigDirName = %q", config.ConfigDirName)
	}
	if config.DatabaseFileName != "hound.db" {
		t.Errorf("DatabaseFileName = %q", config.DatabaseFileName)
	}
}
