package main

import "core:encoding/json"
import "core:fmt"
import "core:os"
import "core:path/filepath"
import "core:strings"

// PoolInfo represents a liquidity pool for a token
PoolInfo :: struct {
	dex:          string, // "raydium"
	pool_address: string, // Pool account address
	quote_token:  string, // "sol", "usdc", etc.
	pool_type:    string, // "amm_v4"
}

// Token represents a single cryptocurrency token configuration
Token :: struct {
	symbol:           string,
	name:             string,
	contract_address: string,
	chain:            string,
	pools:            []PoolInfo, // Liquidity pools for on-chain pricing
	is_quote_token:   bool, // True if this is a quote token (SOL, USDC)
	usd_price:        f64, // USD price for quote tokens
}

// TokenConfig represents the complete token configuration file
TokenConfig :: struct {
	version: string,
	tokens:  []Token,
}

// load_token_config loads the token configuration from ~/.config/hound/tokens.json
// Returns the configuration and an error type
load_token_config :: proc() -> (TokenConfig, ErrorType) {
	// Get home directory
	home, found := os.lookup_env("HOME")
	if !found || len(home) == 0 {
		fmt.eprintln("ERROR: Could not determine home directory")
		return {}, .ConfigNotFound
	}

	// Build config path
	config_path := filepath.join({home, ".config", "hound", "tokens.json"})

	// Check if file exists
	if !os.exists(config_path) {
		fmt.eprintfln("Config file not found: %s", config_path)
		fmt.eprintln("Please create a config file with your token definitions.")
		return {}, .ConfigNotFound
	}

	// Read file
	data, read_ok := os.read_entire_file_from_filename(config_path)
	if !read_ok {
		fmt.eprintln("Failed to read config file")
		return {}, .ConfigNotFound
	}
	defer delete(data)

	// Parse JSON
	config: TokenConfig
	err := json.unmarshal(data, &config)
	if err != nil {
		fmt.eprintfln("Failed to parse config: %v", err)
		return {}, .ConfigParseError
	}

	// Validate config has tokens
	if len(config.tokens) == 0 {
		fmt.eprintln("Config file contains no tokens")
		return {}, .ConfigParseError
	}

	return config, .None
}

// find_token_by_symbol searches for a token by its symbol (case-insensitive)
// Returns the token and true if found, or an empty token and false if not found
find_token_by_symbol :: proc(config: TokenConfig, symbol: string) -> (Token, bool) {
	lower_symbol := strings.to_lower(symbol)

	for token in config.tokens {
		if strings.to_lower(token.symbol) == lower_symbol {
			return token, true
		}
	}

	return {}, false
}

// list_tokens prints all available tokens from the configuration
list_tokens :: proc(config: TokenConfig) {
	fmt.println("Available tokens:")
	fmt.println("")

	for token in config.tokens {
		fmt.printfln("  %s - %s", token.symbol, token.name)
	}
}
