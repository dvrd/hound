#+feature global-context
package main

import "core:fmt"
import "core:log"
import "core:os"
import "core:strings"

import models "../lib/models"
import memory "../lib/memory"
import token_cfg "../lib/config"
import commands "./commands"
import output "./output"

// ============================================================================
// Main Entry Point
// ============================================================================

is_verbose :: proc(args: []string) -> bool {
	for arg in args {
		if arg == "--verbose" {
			return true
		}
	}
  return false
}

main :: proc() {
	log_level := log.Level.Warning

  if is_verbose(os.args) {
		log_level = log.Level.Info
  }

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

	// Handle "wallet" command with subcommands
	if first_arg == "wallet" {
		log.debug("Wallet command invoked")

		// Check for subcommand
		subcommand := ""
		if len(os.args) > 2 {
			subcommand = os.args[2]
		}

		// Route to subcommand handlers
		switch subcommand {
		case "help", "--help", "-h", "":
			// Show help if no subcommand or explicit help
			commands.print_wallet_help()
			return .None

		case "import":
			return commands.handle_wallet_import()

		case "update-password":
			return commands.handle_wallet_update_password()

		case "swap":
			// Pass remaining args (from_symbol, to_symbol, amount)
			swap_args: []string
			if len(os.args) > 3 {
				swap_args = os.args[3:]
			} else {
				swap_args = []string{}
			}
			return commands.handle_wallet_swap(swap_args)

		case "list":
			return commands.handle_wallet_list()

		case "status":
			return commands.handle_wallet_status()

		case "switch":
			if len(os.args) < 4 {
				output.print_error("Missing wallet address/label argument")
				fmt.println("Usage: hound wallet switch <address|label>")
				return .MissingArgument
			}
			return commands.handle_wallet_switch(os.args[3])

		case "delete", "remove":
			if len(os.args) < 4 {
				output.print_error("Missing wallet address/label argument")
				fmt.println("Usage: hound wallet delete <address|label>")
				return .MissingArgument
			}
			return commands.handle_wallet_delete(os.args[3])

		case:
			// Unknown subcommand - show help
			output.print_error(fmt.tprintf("Unknown wallet subcommand: %s", subcommand))
			fmt.println("")
			commands.print_wallet_help()
			return .InvalidToken
		}
	}

	// Handle "history" command
	if first_arg == "history" {
		log.debug("History command invoked")
		// Pass remaining args (after "history") to handler for flag parsing
		return commands.handle_history(os.args[2:])
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
