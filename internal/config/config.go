package config

import (
	"os"
	"path/filepath"
)

const (
	// DefaultRPCEndpoint is the fallback Solana RPC endpoint if HOUND_RPC_ENDPOINT is not set.
	DefaultRPCEndpoint = "https://api.mainnet-beta.solana.com"

	// ConfigDirName is the name of the configuration directory.
	ConfigDirName = "hound"

	// DatabaseFileName is the name of the SQLite database file.
	DatabaseFileName = "hound.db"
)

// Config holds application configuration.
type Config struct {
	DatabasePath    string
	RPCEndpoint     string
	BackupEndpoints []string
}

// DefaultConfig creates a Config with default values, reading from environment variables.
//
// Environment variables:
//   - HOUND_RPC_ENDPOINT: Solana RPC endpoint (default: Helius mainnet)
func DefaultConfig() Config {
	rpcEndpoint := os.Getenv("HOUND_RPC_ENDPOINT")
	if rpcEndpoint == "" {
		rpcEndpoint = DefaultRPCEndpoint
	}

	return Config{
		DatabasePath:    GetDatabasePath(),
		RPCEndpoint:     rpcEndpoint,
		BackupEndpoints: nil,
	}
}

// GetDatabasePath returns the path to the hound database.
// Uses $HOME/.config/hound/hound.db, falling back to /tmp/hound.db.
func GetDatabasePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), DatabaseFileName)
	}

	return filepath.Join(home, ".config", ConfigDirName, DatabaseFileName)
}

// GetConfigDir returns the path to the hound configuration directory.
func GetConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}

	return filepath.Join(home, ".config", ConfigDirName)
}

// EnsureConfigDir creates the configuration directory if it doesn't exist.
func EnsureConfigDir() error {
	dir := GetConfigDir()
	return os.MkdirAll(dir, 0o700)
}

