// List command implementation
// Displays all configured tokens with pool statistics
package commands

import "core:log"
import models "../../lib/models"
import db "../../lib/database"
import memory "../../lib/memory"
import token_cfg "../../lib/config"
import output "../output"

// ============================================================================
// List Command
// ============================================================================

// handle_list implements the "hound list" workflow
//
// Workflow:
// 1. Load token configuration
// 2. Open database for pool statistics
// 3. Display tokens with pool stats using formatter
// 4. Fallback to basic list if database unavailable
//
// Returns: ErrorType for consistent error handling
handle_list :: proc(config: models.TokenConfig) -> models.ErrorType {
	log.debug("Listing all configured tokens with statistics")

	// Try to open database for stats
	db_path := token_cfg.get_database_path()
	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.warnf("Database unavailable, falling back to basic list")
		output.format_basic_token_list(config.tokens)

		// Reset command arena and log stats
		memory.reset_command_arena()
		memory.log_memory_stats()

		return .None
	}
	defer db.database_close(database)

	// Display tokens with comprehensive stats
	output.format_token_list(config.tokens, database)

	// Reset command arena and log stats
	memory.reset_command_arena()
	memory.log_memory_stats()

	return .None
}
