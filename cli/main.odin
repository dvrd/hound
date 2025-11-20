#+feature global-context
package main

import "core:fmt"
import "core:log"
import "core:os"
import "core:strings"

import models "../core/models"
import memory "../core/memory"
import token_cfg "../src/token_config"
import commands "./commands"
import output "./output"

// ============================================================================
// Main Entry Point
// ============================================================================

main :: proc() {
	log_level := log.Level.Info
	if ODIN_DEBUG {
		log_level = log.Level.Debug
	}

	context.logger = log.create_console_logger(log_level, {.Level, .Terminal_Color})

	log.debug("Hound price fetcher starting")
	log.debugf("Log level: %v", log_level)

	// Initialize memory arenas
	mem_err := memory.memory_init()
	if mem_err != .None {
		log.errorf("Failed to initialize memory system: %v", mem_err)
		output.print_error("Memory initialization failed")
		os.exit(1)
	}

	// Execute command
	err := run()

	// Get token for error messages that need it
	token := ""
	if len(os.args) >= 2 {
		token = os.args[1]
	}

	// Display error if any
	if err != .None {
		output.display_error(err, token)
	}

	// Map error to exit code
	exit_code := output.map_error_to_exit_code(err)

	// Cleanup memory arenas and logger before exit
	memory.memory_shutdown()
	log.destroy_console_logger(context.logger)
	os.exit(exit_code)
}

// ============================================================================
// Command Dispatcher
// ============================================================================

run :: proc() -> models.ErrorType {
	// Check arguments
	if len(os.args) < 2 {
		log.debug("No arguments provided")
		return .MissingArgument
	}

	// Parse --memory-stats flag
	for arg in os.args {
		if arg == "--memory-stats" {
			memory.enable_memory_stats()
			break
		}
	}

	first_arg := os.args[1]
	log.debugf("First argument: %s", first_arg)

	// Handle version command
	if first_arg == "version" {
		return commands.handle_version()
	}

	// Handle "add" command
	if first_arg == "add" {
		if len(os.args) < 5 {
			output.print_error("Missing arguments for add command")
			fmt.eprintln("")
			fmt.eprintln("Usage: hound add <symbol> <name> <contract_address>")
			fmt.eprintln("")
			fmt.eprintln("Example:")
			fmt.eprintln("  hound add AURA \"AURA Memecoin\" DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2")
			return .MissingArgument
		}

		symbol := strings.to_lower(os.args[2])
		name := os.args[3]
		address := os.args[4]

		return commands.handle_add(symbol, name, address)
	}

	// Load token configuration (needed for all other commands)
	log.debug("Loading token configuration")
	config, config_err := token_cfg.load_token_config()
	if config_err != .None {
		log.errorf("Failed to load token config: %v", config_err)
		return config_err
	}
	log.debugf("Loaded %d tokens from configuration", len(config.tokens))

	// Handle "list" command
	if first_arg == "list" {
		return commands.handle_list(config)
	}

	// Handle "fetch" command
	if first_arg == "fetch" {
		if len(os.args) < 3 {
			output.print_error("Missing token symbol for fetch command")
			fmt.eprintln("")
			fmt.eprintln("Usage: hound fetch <symbol> [--refresh]")
			fmt.eprintln("")
			fmt.eprintln("Examples:")
			fmt.eprintln("  hound fetch aura           # Fetch using cached pools")
			fmt.eprintln("  hound fetch sol --refresh  # Force pool rediscovery")
			return .MissingArgument
		}

		symbol := strings.to_lower(os.args[2])
		force_refresh := false

		// Check for --refresh flag
		for arg in os.args[3:] {
			if arg == "--refresh" {
				force_refresh = true
				break
			}
		}

		return commands.handle_fetch(symbol, force_refresh)
	}

	// Default: treat first argument as token symbol (backward compatibility)
	symbol := strings.to_lower(first_arg)
	log.debugf("Treating first arg as token symbol: %s", symbol)

	// Check for --refresh flag in remaining args
	force_refresh := false
	for arg in os.args[2:] {
		if arg == "--refresh" {
			force_refresh = true
			break
		}
	}

	return commands.handle_fetch(symbol, force_refresh)
}
