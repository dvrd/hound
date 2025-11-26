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
		log.error("Memory initialization failed")
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

	// Handle "tokens" command with subcommands
	if first_arg == "tokens" {
		log.debug("Tokens command invoked")

		// Pass remaining args to tokens router (subcommand + args)
		tokens_args: []string
		if len(os.args) > 2 {
			tokens_args = os.args[2:]
		} else {
			tokens_args = []string{}
		}

		return commands.handle_tokens(tokens_args)
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
			// Pass remaining args for flag parsing (e.g., --compact)
			list_args: []string
			if len(os.args) > 3 {
				list_args = os.args[3:]
			} else {
				list_args = []string{}
			}
			return commands.handle_wallet_list(list_args)

		case "status":
			return commands.handle_wallet_status()

		case "switch":
			if len(os.args) < 4 {
				log.error("Missing wallet address/label argument")
				fmt.println("\nUsage: hound wallet switch <address|label>")
				return .MissingArgument
			}
			return commands.handle_wallet_switch(os.args[3])

		case "delete", "remove":
			if len(os.args) < 4 {
				log.error("Missing wallet address/label argument")
				fmt.println("\nUsage: hound wallet delete <address|label>")
				return .MissingArgument
			}
			return commands.handle_wallet_delete(os.args[3])

		case:
			// Unknown subcommand - show help
			log.error(fmt.tprintf("Unknown wallet subcommand: %s", subcommand))
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

	// Unknown command - show help
	log.errorf("Unknown command: %s", first_arg)
	return .MissingArgument
}
