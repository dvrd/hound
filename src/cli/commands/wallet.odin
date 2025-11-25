// Wallet command implementation
// Manages Solana wallets and displays cryptocurrency holdings
package commands

import "core:fmt"
import "core:log"
import "core:strconv"
import "core:strings"
import "core:slice"
import "core:encoding/json"
import "core:time"
import "core:os"
import models "../../lib/models"
import db "../../lib/database"
import wallet "../../lib/wallet"
import memory "../../lib/memory"
import token_cfg "../../lib/config"
import keystore_svc "../../lib/services"
import services "../../lib/services"
import keystore "../../lib/keystore"
import output "../output"
import utils "../../lib/utils"

// ============================================================================
// Phase 3: Flag Parsing and Configuration
// ============================================================================

// WalletFlags contains all command-line flags for wallet command
WalletFlags :: struct {
	show_all:      bool,   // --all: Show zero-balance tokens
	json_output:   bool,   // --json: Output JSON format
	wallet_addr:   string, // --wallet: Specific wallet address/label
	sort_by:       string, // --sort: Sort field (value, symbol, balance)
	force_refresh: bool,   // --refresh: Force price refresh
}

// parse_wallet_flags extracts flags from os.args
//
// Parses: --all, --json, --wallet <addr>, --sort <field>, --refresh
//
// Returns: WalletFlags struct with parsed values
parse_wallet_flags :: proc(args: []string) -> (flags: WalletFlags, err: models.ErrorType) {
	log.debugf("Parsing wallet flags from %d args", len(args))

	// Initialize defaults
	flags = WalletFlags{
		show_all      = false,
		json_output   = false,
		wallet_addr   = "",
		sort_by       = "value",  // Default sort
		force_refresh = false,
	}

	// Skip program name and "wallet" command (os.args[0] and os.args[1])
	for i := 2; i < len(args); i += 1 {
		arg := args[i]

		if arg == "--all" {
			flags.show_all = true
			log.debug("Flag: --all (show zero balances)")
		}
		else if arg == "--json" {
			flags.json_output = true
			log.debug("Flag: --json (JSON output)")
		}
		else if arg == "--refresh" {
			flags.force_refresh = true
			log.debug("Flag: --refresh (force refresh)")
		}
		else if arg == "--wallet" {
			// Next arg is the wallet identifier
			if i + 1 >= len(args) {
				log.error("--wallet flag requires an address/label argument")
				fmt.println("")
				fmt.println("Usage: hound wallet --wallet <address|label>")
				return {}, .MissingArgument
			}
			i += 1  // Consume next arg
			flags.wallet_addr = args[i]
			log.debugf("Flag: --wallet %s", flags.wallet_addr)
		}
		else if arg == "--sort" {
			// Next arg is the sort field
			if i + 1 >= len(args) {
				log.error("--sort flag requires a field argument")
				fmt.println("")
				fmt.println("Usage: hound wallet --sort <value|symbol|balance>")
				return {}, .MissingArgument
			}
			i += 1  // Consume next arg
			sort_field := strings.to_lower(args[i])

			// Validate sort field
			if sort_field != "value" && sort_field != "symbol" && sort_field != "balance" {
				log.errorf("Invalid sort field: %s", sort_field)
				fmt.println("Valid fields: value, symbol, balance")
				return {}, .InvalidToken
			}

			flags.sort_by = sort_field
			log.debugf("Flag: --sort %s", sort_field)
		}
		else if strings.has_prefix(arg, "--") {
			// Unknown flag
			log.warnf("Unknown flag: %s", arg)
			fmt.printfln("Warning: Unknown flag '%s' (ignored)", arg)
		}
		// Ignore other args (not flags)
	}

	log.infof("Parsed flags: all=%v json=%v wallet=%s sort=%s refresh=%v",
		flags.show_all, flags.json_output, flags.wallet_addr, flags.sort_by, flags.force_refresh)

	return flags, .None
}

// resolve_target_wallet determines which wallet to use
//
// Priority:
// 1. If wallet_identifier provided: lookup by full address, partial address, or label
// 2. Otherwise: use primary wallet
//
// Returns: Wallet and error status
resolve_target_wallet :: proc(
	database: ^db.Database,
	wallet_identifier: string,
) -> (wallet: models.Wallet, err: models.ErrorType) {
	assert(database != nil, "Database cannot be nil")

	// Case 1: No identifier - use primary wallet
	if len(wallet_identifier) == 0 {
		log.debug("Using primary wallet (no identifier provided)")
		return db.get_primary_wallet(database)
	}

	log.debugf("Resolving wallet identifier: %s", wallet_identifier)

	// Get all wallets for searching
	all_wallets, wallets_err := db.get_all_wallets(database)
	if wallets_err != .None {
		log.errorf("Failed to get wallets: %v", wallets_err)
		return {}, wallets_err
	}

	if len(all_wallets) == 0 {
		log.warn("No wallets configured")
		return {}, .ConfigNotFound
	}

	// Case 2: Try full address match (exact)
	for w in all_wallets {
		if w.address == wallet_identifier {
			log.infof("Matched by full address: %s", w.label)
			return w, .None
		}
	}

	// Case 3: Try partial address match (first 8 characters)
	if len(wallet_identifier) >= 8 {
		partial := wallet_identifier[:8]
		matches := make([dynamic]models.Wallet, memory.command_allocator())

		for w in all_wallets {
			if strings.has_prefix(w.address, partial) {
				append(&matches, w)
			}
		}

		if len(matches) == 1 {
			// Unique match
			log.infof("Matched by partial address: %s", matches[0].label)
			return matches[0], .None
		} else if len(matches) > 1 {
			// Ambiguous - multiple matches
			log.warnf("Partial address '%s' matches %d wallets (ambiguous)", partial, len(matches))
			fmt.printfln("Partial address '%s' matches multiple wallets:", partial)
			for m in matches {
				fmt.printfln("  - %s (%s)", m.label, m.address[:12])
			}
			fmt.println("Please use a more specific address or full address.")
			return {}, .InvalidToken
		}
	}

	// Case 4: Try label match (case-insensitive)
	identifier_lower := strings.to_lower(wallet_identifier, context.temp_allocator)
	for w in all_wallets {
		label_lower := strings.to_lower(w.label, context.temp_allocator)
		if label_lower == identifier_lower {
			log.infof("Matched by label: %s", w.label)
			return w, .None
		}
	}

	// No match found
	log.warnf("No wallet found for identifier: %s", wallet_identifier)
	fmt.println("")
	fmt.printfln("No wallet found for '%s'", wallet_identifier)
	fmt.println("")
	fmt.println("Available wallets:")
	for w in all_wallets {
		primary_marker := w.is_primary ? " (primary)" : ""
		fmt.printfln("  - %s: %s%s", w.label, w.address[:16], primary_marker)
	}

	return {}, .ConfigNotFound
}

// sort_token_balances sorts balances by specified field
//
// Modifies: balances slice (sorted in-place)
sort_token_balances :: proc(balances: []wallet.TokenBalance, sort_field: string) {
	assert(balances != nil, "Balances slice cannot be nil")

	log.debugf("Sorting %d balances by %s", len(balances), sort_field)

	switch sort_field {
	case "value":
		// Sort by USD value (descending - highest first)
		slice.sort_by(balances, proc(a, b: wallet.TokenBalance) -> bool {
			return a.usd_value > b.usd_value
		})

	case "symbol":
		// Sort by symbol (alphabetical)
		slice.sort_by(balances, proc(a, b: wallet.TokenBalance) -> bool {
			return a.symbol < b.symbol
		})

	case "balance":
		// Sort by balance amount (descending - highest first)
		slice.sort_by(balances, proc(a, b: wallet.TokenBalance) -> bool {
			return a.amount > b.amount
		})

	case:
		log.warnf("Unknown sort field '%s', defaulting to value", sort_field)
		// Default: sort by value
		slice.sort_by(balances, proc(a, b: wallet.TokenBalance) -> bool {
			return a.usd_value > b.usd_value
		})
	}

	log.debug("Sorting complete")
}

// ============================================================================
// Wallet Command Handler
// ============================================================================

// handle_wallet implements "hound wallet [flags]" workflow
//
// Phase 3 enhancements:
// - Flag parsing (--all, --json, --wallet, --sort, --refresh)
// - JSON output support
// - Flexible sorting
// - Advanced wallet selection
//
// Returns: ErrorType for consistent error handling
handle_wallet :: proc(flags: WalletFlags) -> models.ErrorType {
	log.debugf("Wallet command: flags=%#v", flags)

	// Step 1: Load token configuration
	config, config_err := token_cfg.load_token_config()
	if config_err != .None {
		log.errorf("Failed to load token config: %v", config_err)
		return config_err
	}

	// Step 2: Open database
	db_path := token_cfg.get_database_path()
	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		return .DatabaseError
	}
	defer db.database_close(database)

	// Step 3: Resolve target wallet (primary or specified)
	target_wallet, wallet_err := resolve_target_wallet(database, flags.wallet_addr)
	if wallet_err != .None {
		return wallet_err
	}

	log.infof("Target wallet: %s (%s)", target_wallet.label, target_wallet.address)

	// Step 4: Initialize RPC client and balance fetcher
	// Check for custom RPC endpoint from environment variable
	rpc_endpoint := os.get_env("HOUND_RPC_ENDPOINT", context.temp_allocator)
	if len(rpc_endpoint) == 0 {
		// Default to public RPC (slow but free)
		rpc_endpoint = "https://api.mainnet-beta.solana.com"
		log.debug("Using default public RPC endpoint")
	} else {
		log.infof("Using custom RPC endpoint: %s", rpc_endpoint)
	}

	backup_endpoints: []string = nil
	rpc_client := wallet.init_rpc_client(rpc_endpoint, backup_endpoints)

	price_fetcher := wallet.PriceFetcher{}
	balance_fetcher := wallet.init_balance_fetcher(&rpc_client, &price_fetcher)

	// Step 5: Fetch portfolio
	if flags.force_refresh {
		output.print_progress("Refreshing wallet balances...")
	} else {
		output.print_progress("Fetching wallet balances...")
	}

	portfolio, portfolio_err := wallet.fetch_portfolio_balance(
		&balance_fetcher,
		target_wallet.address,
		&config,
		database,
	)

	if portfolio_err != .None {
		log.errorf("Failed to fetch portfolio: %v", portfolio_err)

		// Try to show cached balances from database
		log.warn("Failed to fetch fresh balances, trying cached data...")
		cached_portfolio, cached_err := get_cached_portfolio(database, target_wallet.address)

		if cached_err == .None && len(cached_portfolio.token_balances) > 0 {
			portfolio = cached_portfolio
			log.warn("Using cached balances (prices may be stale)")
		} else {
			log.error("Could not fetch wallet balances")
			return portfolio_err
		}
	}

	log.infof("Portfolio fetched: $%.2f total", portfolio.total_usd)

	// Step 6: Build combined balances list (SOL + tokens)
	all_balances := make([dynamic]wallet.TokenBalance, memory.command_allocator())

	// Add SOL balance (always show if non-zero, or if --all flag)
	if portfolio.sol_balance.amount > 0 || flags.show_all {
		append(&all_balances, portfolio.sol_balance)
	}

	// Add token balances
	for token_balance in portfolio.token_balances {
		// Filter zero balances unless --all flag
		if token_balance.amount > 0 || flags.show_all {
			// Also filter out tokens with no price (failed to fetch price)
			// These are unknown tokens that show "TokenNotFound" error
			if token_balance.usd_price > 0 {
				append(&all_balances, token_balance)
			}
		}
	}

	log.debugf("Total balances to display: %d (show_all=%v)", len(all_balances), flags.show_all)

	// Step 7: Sort balances by specified field
	// CRITICAL: In Odin, empty [dynamic] array converted to slice can become nil!
	// Use fixed array slice for empty case to ensure non-nil pointer.
	balances_slice: []wallet.TokenBalance
	if len(all_balances) > 0 {
		balances_slice = all_balances[:]
	} else {
		empty: [0]wallet.TokenBalance
		balances_slice = empty[:]
	}
	sort_token_balances(balances_slice, flags.sort_by)

	// Step 8: Display output in requested format
	if flags.json_output {
		// JSON output
		output.format_wallet_json(target_wallet, portfolio, balances_slice)
	} else {
		// Table output (default)
		output.format_wallet_table(target_wallet, portfolio, balances_slice)
	}

	// Step 9: Reset arena and log stats
	memory.reset_command_arena()
	memory.log_memory_stats()

	return .None
}

// ============================================================================
// Swap Subcommand (Phase 2: Quote & Dry-Run)
// ============================================================================

// handle_wallet_swap implements swap quote fetching and display
//
// Phase 2 Scope: Quote fetching + dry-run + confirmation prompt
// Phase 3 Will Add: Transaction execution
//
// Command: hound wallet swap <from> <to> <amount> [flags]
// Flags: --dry-run, --slippage <bps>, --wallet <addr>
//
// Parameters:
//   - args: Command arguments after "wallet swap"
//
// Returns: ErrorType for error handling
handle_wallet_swap :: proc(args: []string) -> models.ErrorType {
	log.debugf("Wallet swap subcommand: %d args", len(args))

	// Validate argument count
	if len(args) < 3 {
		fmt.println("")
		fmt.println("Usage: hound wallet swap <from> <to> <amount> [flags]")
		fmt.println("")
		fmt.println("Examples:")
		fmt.println("  hound wallet swap sol usdc 1.0")
		fmt.println("  hound wallet swap sol usdc 1.0 --dry-run")
		fmt.println("  hound wallet swap sol usdc 1.0 --slippage 100  # 1%")
		fmt.println("")
		fmt.println("Flags:")
		fmt.println("  --dry-run         Simulation mode (no execution)")
		fmt.println("  --slippage <bps>  Custom slippage (default: 50 = 0.5%)")
		fmt.println("  --wallet <addr>   Use specific wallet")
		return .MissingArgument
	}

	// Extract positional args
	from_symbol := strings.to_lower(args[0])
	to_symbol := strings.to_lower(args[1])
	amount_str := args[2]

	// Parse flags from remaining args
	flags := parse_swap_flags(args[3:])

	log.infof("Swap: %s → %s, amount: %s, dry_run: %v",
		from_symbol, to_symbol, amount_str, flags.dry_run)

	// Open database
	db_path := token_cfg.get_database_path()
	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		return .DatabaseError
	}
	defer db.database_close(database)

	// Resolve target wallet (uses existing function!)
	target_wallet, wallet_err := resolve_target_wallet(database, flags.wallet_addr)
	if wallet_err != .None {
		return wallet_err
	}

	log.infof("Using wallet: %s (%s)", target_wallet.label, target_wallet.address)

	// Load token config (for mint resolution)
	config, config_err := token_cfg.load_token_config()
	if config_err != .None {
		log.errorf("Failed to load tokens: %v", config_err)
		return config_err
	}

	// Resolve token symbols to mints
	from_token, found_from := models.get_token_by_symbol(&config, from_symbol)
	if !found_from {
		fmt.println("")
		fmt.println("Run 'hound list' to see available tokens.")
		return .TokenNotFound
	}

	to_token, found_to := models.get_token_by_symbol(&config, to_symbol)
	if !found_to {
		fmt.println("")
		fmt.println("Run 'hound list' to see available tokens.")
		return .TokenNotFound
	}

	log.debugf("Resolved: %s → %s, %s → %s",
		from_symbol, from_token.contract_address,
		to_symbol, to_token.contract_address)

	// Parse amount (handle decimals)
	amount_f64, amount_ok := strconv.parse_f64(amount_str)
	if !amount_ok || amount_f64 <= 0 {
		return .InvalidToken
	}

	// Convert to lamports (use token decimals or default to 9)
	decimals := models.get_token_decimals(from_token)
	multiplier := f64(1)
	for i := 0; i < decimals; i += 1 {
		multiplier *= 10
	}
	amount_lamports := u64(amount_f64 * multiplier)

	log.debugf("Amount: %.6f %s = %d lamports (decimals=%d)",
		amount_f64, from_symbol, amount_lamports, decimals)

	// NOTE: Balance validation skipped in Phase 2
	// Jupiter API will fail if balance is insufficient
	// TODO Phase 3: Add proper balance validation before quote

	// Fetch swap quote from Jupiter
	output.print_progress("Fetching quote from Jupiter...")

	quote, quote_err := services.fetch_swap_quote(
		from_token.contract_address,
		to_token.contract_address,
		amount_lamports,
		target_wallet.address,  // Pass wallet address as taker
		flags.slippage_bps,
	)
	if quote_err != .None {
		fmt.println("")
		fmt.println("Possible reasons:")
		fmt.println("  - No liquidity available")
		fmt.println("  - Invalid token pair")
		fmt.println("  - Jupiter API unavailable")
		return quote_err
	}

	// Convert lamports to human-readable amounts
	quote.input_symbol = from_symbol
	quote.input_amount = amount_f64
	quote.output_symbol = to_symbol

	// Use output token decimals for conversion
	out_decimals := models.get_token_decimals(to_token)
	out_multiplier := f64(1)
	for i := 0; i < out_decimals; i += 1 {
		out_multiplier *= 10
	}
	quote.output_amount = f64(quote.output_lamports) / out_multiplier
	quote.minimum_out = f64(quote.output_lamports) / out_multiplier * (1.0 - f64(flags.slippage_bps) / 10000.0)

	// Recalculate rate using human-readable amounts (not lamports)
	if quote.input_amount > 0 {
		quote.rate = quote.output_amount / quote.input_amount
	}

	// Display formatted quote
	output.format_swap_quote(quote, from_symbol, to_symbol)

	// Show route details if multi-hop
	if len(quote.route_plan) > 1 {
		output.format_route_steps(quote.route_plan)
	}

	// Dry-run mode: show simulation message and exit
	if flags.dry_run {
		fmt.println("🔍 DRY-RUN MODE: No transaction will be executed.")
		fmt.println("This is a simulation only.")
		fmt.println("")
		return .None
	}

	// Prompt user confirmation
	confirmed := output.prompt_swap_confirmation()
	if !confirmed {
		fmt.println("Swap cancelled.")
		return .None
	}

	// Phase 3: Execute swap transaction
	fmt.println("")
	output.print_progress("Preparing transaction...")

	// Step 1: Prompt for password
	password, password_ok := utils.read_password_secure()
	defer utils.zero_string(&password)

	if !password_ok {
		return .NetworkError
	}

	if len(password) == 0 {
		return .InvalidToken
	}

	// Step 2: Retrieve and decrypt keypair (database already opened earlier in function)
	output.print_progress("Decrypting keypair...")
	keypair, decrypt_err := keystore_svc.unlock_keypair(database, target_wallet.address, password)

	if decrypt_err != .None {
		if decrypt_err == .KeypairNotFound {
		} else {
		}
		return decrypt_err
	}

	log.debug("Keypair decrypted successfully")

	// Step 3: Execute transaction
	output.print_progress("Signing and submitting transaction...")
	result, exec_err := services.execute_swap_transaction(
		quote,
		keypair,
		from_symbol,
		to_symbol,
	)

	if exec_err != .None {
		// Provide specific error messages for common issues
		if exec_err == .InsufficientBalance {
			fmt.println("")
			fmt.println("Your wallet doesn't have enough SOL to complete this swap.")
			fmt.println("Please deposit SOL to your wallet and try again.")
			fmt.println("")
			fmt.printfln("Wallet address: %s", target_wallet.address)
		} else {
		}
		return exec_err
	}

	// Step 4: Save to swap history
	log.debug("Recording swap to history")
	history_err := db.insert_swap_history(database, target_wallet.address, result, flags.slippage_bps)
	if history_err != .None {
		log.warnf("Failed to save swap history: %v", history_err)
		// Don't fail the command - transaction already succeeded
	}

	// Step 5: Display result
	output.format_swap_result(result)

	return .None
}

// parse_swap_flags extracts swap-specific flags from args
//
// Flags: --dry-run, --slippage <bps>, --wallet <addr>
//
// Returns: SwapFlags struct with parsed values
parse_swap_flags :: proc(args: []string) -> models.SwapFlags {
	flags := models.SwapFlags{
		dry_run      = false,
		slippage_bps = 50,  // Default: 0.5%
		wallet_addr  = "",
	}

	for i := 0; i < len(args); i += 1 {
		arg := args[i]

		if arg == "--dry-run" {
			flags.dry_run = true
			log.debug("Flag: --dry-run")
		}
		else if arg == "--slippage" {
			if i + 1 >= len(args) {
				log.warn("--slippage requires value (using default 50)")
				continue
			}
			i += 1
			slippage_val, ok := strconv.parse_int(args[i])
			if ok && slippage_val >= 1 && slippage_val <= 1000 {
				flags.slippage_bps = int(slippage_val)
				log.debugf("Flag: --slippage %d bps", slippage_val)
			} else {
				log.warnf("Invalid slippage value: %s (using default 50)", args[i])
			}
		}
		else if arg == "--wallet" {
			if i + 1 >= len(args) {
				log.warn("--wallet requires address/label")
				continue
			}
			i += 1
			flags.wallet_addr = args[i]
			log.debugf("Flag: --wallet %s", flags.wallet_addr)
		}
	}

	return flags
}

// NOTE: validate_swap_balance removed in Phase 2
// Balance validation will be added in Phase 3 with proper RPC integration

// ============================================================================
// Helper Functions
// ============================================================================

// get_cached_portfolio retrieves portfolio from database cache
//
// Falls back to database when network fetch fails
get_cached_portfolio :: proc(
	database: ^db.Database,
	wallet_address: string,
) -> (portfolio: wallet.PortfolioBalance, err: models.ErrorType) {
	// Initialize portfolio early to ensure non-nil slices (satisfies assertions)
	portfolio.wallet_address = wallet_address
	portfolio.total_usd = 0
	// CRITICAL: In Odin, make([]T, 0) can return nil! Use fixed array slice instead.
	// Fixed arrays always have valid stack addresses, even when empty.
	empty_array: [0]wallet.TokenBalance
	portfolio.token_balances = empty_array[:]

	// Get cached balances from database
	balances_map, balances_err := db.get_balances_for_wallet(database, wallet_address)
	if balances_err != .None {
		return portfolio, balances_err  // Return portfolio with initialized slice, not {}
	}

	// Convert map to token balances slice
	token_balances := make([dynamic]wallet.TokenBalance, memory.command_allocator())

	for mint, data in balances_map {
		balance := wallet.TokenBalance{
			mint      = mint,
			symbol    = "",  // Will be filled if we can match
			amount    = data[0],
			decimals  = 9,  // Default, actual value may vary
			usd_price = data[1],
			usd_value = data[2],
		}

		append(&token_balances, balance)
		portfolio.total_usd += data[2]  // Add usd_value
	}

	portfolio.token_balances = token_balances[:]

	return portfolio, .None
}

// ============================================================================
// Wallet Import Subcommand (Phase 1: Secure Keystore)
// ============================================================================

// handle_wallet_import implements "hound wallet import" workflow
//
// Imports a Solana keypair from BIP39 seed phrase with password encryption
//
// Returns: ErrorType for error handling
handle_wallet_import :: proc() -> models.ErrorType {
	log.info("Starting wallet import workflow")

	// Step 1: Prompt for seed phrase (12 or 24 words)
	seed_phrase, seed_err := prompt_seed_phrase()
	if seed_err != .None {
		return seed_err
	}
	defer {
		// Zero seed phrase memory (no delete needed - using command arena)
		for word in seed_phrase {
			if len(word) > 0 {
				mem_word := transmute([]byte)word
				for i := 0; i < len(mem_word); i += 1 {
					mem_word[i] = 0
				}
			}
		}
	}

	// Step 2: Prompt for wallet type selection
	wallet_type, type_err := prompt_wallet_type()
	if type_err != .None {
		return type_err
	}

	// Step 3: Prompt for account index (for multi-account wallets)
	account_index, index_err := prompt_account_index()
	if index_err != .None {
		return index_err
	}

	// Step 4: Derive address for preview (before password entry)
	derivation_path := models.get_derivation_path(wallet_type, account_index)

	// Temporarily derive keypair to show address
	preview_keypair: keystore.Keypair
	preview_err: models.ErrorType

	if wallet_type == .Legacy {
		preview_keypair, preview_err = keystore.derive_keypair_from_seed(seed_phrase)
	} else {
		preview_keypair, preview_err = keystore.derive_keypair_from_seed_bip44(seed_phrase, "", u32(account_index), 0)
	}

	if preview_err != .None {
		log.errorf("Failed to derive preview address: %v", preview_err)
		return preview_err
	}
	defer keystore.zero_keypair(&preview_keypair)

	preview_address := keystore.keypair_to_address(&preview_keypair)

	// Step 5: Display address and confirm
	confirmed := display_address_confirmation(preview_address, wallet_type)
	if !confirmed {
		fmt.println("If the address doesn't match, try selecting a different wallet type.")
		return .None  // Not an error, user chose to cancel
	}

	// Step 6: Prompt for password (with confirmation)
	password, password_err := prompt_password_with_confirmation()
	if password_err != .None {
		return password_err
	}
	defer {
		// Zero password memory (no delete needed - using command arena)
		if len(password) > 0 {
			mem_password := transmute([]byte)password
			for i := 0; i < len(mem_password); i += 1 {
				mem_password[i] = 0
			}
		}
	}

	// Step 3: Prompt for wallet label
	fmt.print("Enter wallet label (e.g., 'Main Wallet'): ")
	label_buffer: [256]byte
	n, read_err := os.read(os.stdin, label_buffer[:])
	if read_err != nil {
		log.errorf("Failed to read label: %v", read_err)
		return .NetworkError
	}
	label := strings.trim_space(string(label_buffer[:n]))

	if len(label) == 0 {
		label = "Imported Wallet"
	}

	// Step 4: Open database
	db_path := token_cfg.get_database_path()
	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		return .DatabaseError
	}
	defer db.database_close(database)

	// Step 9: Import keypair (encrypts and stores)
	output.print_progress("Encrypting and storing wallet...")
	address, import_err := keystore_svc.import_keypair(database, seed_phrase, password, label, true, wallet_type, account_index)
	if import_err != .None {
		log.errorf("Failed to import keypair: %v", import_err)
		return import_err
	}

	// Step 10: Display success with wallet type information
	output.print_success(fmt.tprintf("Wallet imported successfully!"))
	fmt.println("")
	fmt.printfln("Label:   %s", label)
	fmt.printfln("Address: %s", address)
	fmt.printfln("Type:    %v", wallet_type)
	if len(derivation_path) > 0 {
		fmt.printfln("Path:    %s", derivation_path)
	}
	fmt.println("")
	fmt.println("Your wallet is now encrypted and stored securely.")
	fmt.println("Use the password to unlock your wallet for transactions.")

	return .None
}

// handle_wallet_update_password implements "hound wallet update-password" workflow
//
// Updates wallet password by providing the seed phrase again
//
// Returns: ErrorType for error handling
handle_wallet_update_password :: proc() -> models.ErrorType {
	log.info("Starting wallet password update workflow")

	// Step 1: Prompt for seed phrase (12 or 24 words)
	fmt.println("To update your wallet password, you need to provide your seed phrase.")
	fmt.println("")
	seed_phrase, seed_err := prompt_seed_phrase()
	if seed_err != .None {
		return seed_err
	}
	defer {
		// Zero seed phrase memory
		for word in seed_phrase {
			if len(word) > 0 {
				mem_word := transmute([]byte)word
				for i := 0; i < len(mem_word); i += 1 {
					mem_word[i] = 0
				}
			}
		}
	}

	// Step 2: Prompt for new password (with confirmation)
	fmt.println("")
	fmt.println("Now create your new password.")
	new_password, password_err := prompt_password_with_confirmation()
	if password_err != .None {
		return password_err
	}
	defer {
		// Zero password memory
		if len(new_password) > 0 {
			mem_password := transmute([]byte)new_password
			for i := 0; i < len(mem_password); i += 1 {
				mem_password[i] = 0
			}
		}
	}

	// Step 3: Open database
	db_path := token_cfg.get_database_path()
	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		return .DatabaseError
	}
	defer db.database_close(database)

	// Step 4: Update keypair password
	output.print_progress("Updating wallet password...")
	address, update_err := keystore_svc.update_keypair_password(database, seed_phrase, "", new_password)
	if update_err != .None {
		log.errorf("Failed to update password: %v", update_err)

		// Provide helpful error messages
		if update_err == .KeypairNotFound {
			fmt.println("")
			fmt.println("The seed phrase you provided doesn't match any imported wallet.")
			fmt.println("Please check your seed phrase and try again.")
		} else {
		}
		return update_err
	}

	// Step 5: Display success
	output.print_success("Password updated successfully!")
	fmt.println("")
	fmt.printfln("Wallet: %s", address)
	fmt.println("")
	fmt.println("Your wallet password has been updated.")
	fmt.println("Use the new password to unlock your wallet for transactions.")

	return .None
}

// prompt_seed_phrase prompts user to enter seed phrase and validates format
//
// Returns: Slice of seed words and error status
prompt_seed_phrase :: proc() -> (words: []string, err: models.ErrorType) {
	fmt.println("Enter your seed phrase (12 or 24 words).")
	fmt.println("Words should be separated by spaces.")
	fmt.print("Seed phrase: ")

	// Read seed phrase from stdin
	buffer: [1024]byte
	n, read_err := os.read(os.stdin, buffer[:])
	if read_err != nil {
		log.errorf("Failed to read seed phrase: %v", read_err)
		return nil, .NetworkError
	}

	// Parse words
	phrase := strings.trim_space(string(buffer[:n]))
	words_dynamic := strings.split(phrase, " ")

	// Filter out empty strings
	words_filtered := make([dynamic]string, memory.command_allocator())
	for word in words_dynamic {
		trimmed := strings.trim_space(word)
		if len(trimmed) > 0 {
			// Use command arena allocator (no manual cleanup needed)
			append(&words_filtered, strings.clone(trimmed, memory.command_allocator()))
		}
	}

	words = words_filtered[:]

	// Validate word count
	if len(words) != 12 && len(words) != 24 {
		log.errorf("Invalid seed phrase length: %d words", len(words))
		return nil, .InvalidSeedPhrase
	}

	log.infof("Seed phrase parsed: %d words", len(words))
	return words, .None
}

// prompt_wallet_type displays interactive wallet type selection menu
//
// Returns: Selected Wallet_Type and error status
prompt_wallet_type :: proc() -> (wallet_type: models.Wallet_Type, err: models.ErrorType) {
	// Display menu
	fmt.println("")
	fmt.println("Select wallet derivation standard:")
	fmt.println("")
	fmt.println("  1. BIP44 Standard (Phantom/Solflare/Ledger) [Recommended]")
	fmt.println("     Derivation: m/44'/501'/0'/0'")
	fmt.println("     Use this for wallets from Phantom, Solflare, or Ledger")
	fmt.println("")
	fmt.println("  2. BIP44-Change (Alternative standard)")
	fmt.println("     Derivation: m/44'/501'/0'")
	fmt.println("     Use if your wallet uses this alternative path")
	fmt.println("")
	fmt.println("  3. Solana CLI (solana-keygen)")
	fmt.println("     Derivation: m/44'/501'")
	fmt.println("     Use if wallet was created with solana-keygen tool")
	fmt.println("")
	fmt.println("  4. Legacy (Hound original method)")
	fmt.println("     Derivation: SHA-256 seed → Ed25519")
	fmt.println("     Only for wallets previously imported in old Hound versions")
	fmt.println("")
	fmt.print("Enter selection (1-4) [1]: ")

	// Read input
	buffer: [256]byte
	n, read_err := os.read(os.stdin, buffer[:])
	if read_err != nil {
		log.error("Failed to read wallet type selection")
		return .BIP44_Standard, .NetworkError  // Default to recommended
	}

	selection := strings.trim_space(string(buffer[:n]))

	// Handle default (empty input)
	if len(selection) == 0 {
		log.info("Using default wallet type: BIP44_Standard")
		return .BIP44_Standard, .None
	}

	// Parse selection
	selection_int, parse_ok := strconv.parse_int(selection)
	if !parse_ok || selection_int < 1 || selection_int > 4 {
		log.errorf("Invalid wallet type selection: %s", selection)
		return .BIP44_Standard, .InvalidToken
	}

	// Map to enum
	wallet_types := [4]models.Wallet_Type{
		.BIP44_Standard,  // 1
		.BIP44_Change,    // 2
		.Solana_CLI,      // 3
		.Legacy,          // 4
	}

	wallet_type = wallet_types[selection_int - 1]
	log.infof("Selected wallet type: %v", wallet_type)
	return wallet_type, .None
}

// prompt_account_index prompts for optional account index input
//
// Returns: Account index and error status
prompt_account_index :: proc() -> (account_index: int, err: models.ErrorType) {
	fmt.println("")
	fmt.print("Account index (for multi-account wallets) [0]: ")

	buffer: [256]byte
	n, read_err := os.read(os.stdin, buffer[:])
	if read_err != nil {
		log.error("Failed to read account index")
		return 0, .NetworkError
	}

	input := strings.trim_space(string(buffer[:n]))

	// Default to 0 if empty
	if len(input) == 0 {
		log.info("Using default account index: 0")
		return 0, .None
	}

	// Parse index
	index, parse_ok := strconv.parse_int(input)
	if !parse_ok || index < 0 {
		log.errorf("Invalid account index: %s", input)
		return 0, .InvalidToken
	}

	log.infof("Selected account index: %d", index)
	return index, .None
}

// display_address_confirmation shows derived address and prompts for confirmation
//
// Returns: True if user confirms address match
display_address_confirmation :: proc(address: string, wallet_type: models.Wallet_Type) -> bool {
	fmt.println("")
	output.print_progress(fmt.tprintf("Deriving keypair with %v...", wallet_type))
	fmt.printfln("Derived address: %s", address)
	fmt.println("")
	fmt.print("Does this address match your wallet? [y/N]: ")

	buffer: [256]byte
	n, err := os.read(os.stdin, buffer[:])
	if err != nil {
		log.error("Failed to read confirmation")
		return false
	}

	response := strings.trim_space(string(buffer[:n]))
	response_lower := strings.to_lower(response, context.temp_allocator)

	confirmed := response_lower == "y" || response_lower == "yes"

	if confirmed {
		log.info("User confirmed address match")
	} else {
		log.info("User rejected address match")
	}

	return confirmed
}

// prompt_password_with_confirmation prompts for password twice and validates match
//
// Returns: Password string and error status
prompt_password_with_confirmation :: proc() -> (password: string, err: models.ErrorType) {
	fmt.println("")
	fmt.println("Create a strong password to encrypt your wallet.")
	fmt.println("Requirements:")
	fmt.println("  - At least 12 characters")
	fmt.println("  - Contains uppercase and lowercase letters")
	fmt.println("  - Contains at least one digit")
	fmt.println("  - Contains at least one special character")
	fmt.println("")

	// First password entry
	fmt.print("Password: ")
	password1_buffer: [256]byte
	n1, read_err1 := os.read(os.stdin, password1_buffer[:])
	if read_err1 != nil {
		log.errorf("Failed to read password: %v", read_err1)
		return "", .NetworkError
	}
	password1 := strings.trim_space(string(password1_buffer[:n1]))

	// Second password entry
	fmt.print("Confirm password: ")
	password2_buffer: [256]byte
	n2, read_err2 := os.read(os.stdin, password2_buffer[:])
	if read_err2 != nil {
		log.errorf("Failed to read password confirmation: %v", read_err2)
		// Zero first password before returning
		for i := 0; i < len(password1); i += 1 {
			password1_buffer[i] = 0
		}
		return "", .NetworkError
	}
	password2 := strings.trim_space(string(password2_buffer[:n2]))

	// Compare passwords
	if password1 != password2 {
		log.error("Passwords do not match")
		// Zero both passwords
		for i := 0; i < n1; i += 1 {
			password1_buffer[i] = 0
		}
		for i := 0; i < n2; i += 1 {
			password2_buffer[i] = 0
		}
		return "", .WeakPassword
	}

	// Zero confirmation password buffer
	for i := 0; i < n2; i += 1 {
		password2_buffer[i] = 0
	}

	// Validate password strength before accepting
	password_strength_err := keystore_svc.validate_password_strength(password1)
	if password_strength_err != .None {
		log.errorf("Password validation failed: %v", password_strength_err)
		// Zero both password buffers
		for i := 0; i < n1; i += 1 {
			password1_buffer[i] = 0
		}
		for i := 0; i < n2; i += 1 {
			password2_buffer[i] = 0
		}
		return "", password_strength_err
	}

	// Use command arena allocator (no manual cleanup needed)
	password = strings.clone(password1, memory.command_allocator())

	// Zero original password buffer
	for i := 0; i < n1; i += 1 {
		password1_buffer[i] = 0
	}

	log.info("Password confirmed and validated")
	return password, .None
}

// ============================================================================
// Wallet Subcommand Handlers
// ============================================================================

// print_wallet_help displays comprehensive help for wallet command
print_wallet_help :: proc() {
	fmt.println("hound wallet - Manage Solana wallets and view balances")
	fmt.println("")
	fmt.println("USAGE:")
	fmt.println("  hound wallet [subcommand] [flags]")
	fmt.println("")
	fmt.println("SUBCOMMANDS:")
	fmt.println("  list            List all configured wallets")
	fmt.println("  status          Show current wallet status and balances")
	fmt.println("  switch          Switch active wallet")
	fmt.println("  import          Import a wallet from seed phrase")
	fmt.println("  update-password Update wallet password (requires seed phrase)")
	fmt.println("  delete          Delete a wallet (requires confirmation)")
	fmt.println("  swap            Swap tokens between pairs")
	fmt.println("  help            Show this help message")
	fmt.println("")
	fmt.println("FLAGS:")
	fmt.println("  --compact              Output in compact format (for list command)")
	fmt.println("  --wallet <addr|label>  Use specific wallet (for status command)")
	fmt.println("  --all                  Show zero-balance tokens (for status command)")
	fmt.println("  --json                 Output in JSON format (for status command)")
	fmt.println("  --sort <field>         Sort by: value, symbol, balance (for status command)")
	fmt.println("  --refresh              Force price refresh (for status command)")
	fmt.println("")
	fmt.println("EXAMPLES:")
	fmt.println("  hound wallet list                # List all wallets in table format")
	fmt.println("  hound wallet list --compact      # List wallets in parseable format")
	fmt.println("  hound wallet status              # Show primary wallet balances")
	fmt.println("  hound wallet status --wallet main  # Show specific wallet")
	fmt.println("  hound wallet switch main         # Switch to 'main' wallet")
	fmt.println("  hound wallet import              # Import new wallet")
	fmt.println("  hound wallet swap sol usdc 1.0   # Swap 1 SOL for USDC")
	fmt.println("")
}

// handle_wallet_list lists all configured wallets
//
// Params:
//   args: Command line arguments (checks for --compact flag)
//
// Returns: ErrorType for error handling
handle_wallet_list :: proc(args: []string = nil) -> models.ErrorType {
	log.info("Listing all wallets")

	// Check for --compact flag
	compact := false
	if args != nil {
		for arg in args {
			if arg == "--compact" {
				compact = true
				break
			}
		}
	}

	// Open database
	db_path := token_cfg.get_database_path()
	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		return .DatabaseError
	}
	defer db.database_close(database)

	// Get all wallets
	wallets, wallets_err := db.get_all_wallets(database)
	if wallets_err != .None {
		log.errorf("Failed to get wallets: %v", wallets_err)
		return wallets_err
	}

	if len(wallets) == 0 {
		if compact {
			// In compact mode, just output nothing for empty list
			return .None
		}
		fmt.println("No wallets configured.")
		fmt.println("")
		fmt.println("Import a wallet with: hound wallet import")
		return .None
	}

	// Display wallets in compact or table format
	if compact {
		// Compact format: space-separated columns
		// Format: LABEL ADDRESS IS_PRIMARY WALLET_TYPE
		for wallet in wallets {
			primary_flag := wallet.is_primary ? "primary" : "secondary"
			type_name := fmt.tprintf("%v", wallet.wallet_type)
			fmt.printfln("%s %s %s %s", wallet.label, wallet.address, primary_flag, type_name)
		}
	} else {
		// Table format (original)
		fmt.println("Configured Wallets:")
		fmt.println("")
		fmt.println("┌────────────────────────┬──────────────────────────────────────────────┬───────────────────────┐")
		fmt.println("│ Label                  │ Address                                      │ Status                │")
		fmt.println("├────────────────────────┼──────────────────────────────────────────────┼───────────────────────┤")

		for wallet in wallets {
			label_padded := fmt.tprintf("%-22s", wallet.label)
			address_short := fmt.tprintf("%-44s", wallet.address)

			// Build status with badges
			status_builder := strings.builder_make(memory.command_allocator())
			if wallet.is_primary {
				strings.write_string(&status_builder, "PRIMARY ")
			}

			// Add wallet type badge
			type_name := fmt.tprintf("%v", wallet.wallet_type)
			strings.write_string(&status_builder, "[")
			strings.write_string(&status_builder, type_name)
			strings.write_string(&status_builder, "]")

			status := strings.to_string(status_builder)
			status_padded := fmt.tprintf("%-21s", status)

			fmt.printfln("│ %s │ %s │ %s │", label_padded, address_short, status_padded)
		}

		fmt.println("└────────────────────────┴──────────────────────────────────────────────┴───────────────────────┘")
		fmt.println("")
		fmt.printfln("Total: %d wallet(s)", len(wallets))
		fmt.println("")
	}

	return .None
}

// handle_wallet_status shows current wallet status and balances
//
// This is a wrapper around handle_wallet() that parses flags from os.args
//
// Returns: ErrorType for error handling
handle_wallet_status :: proc() -> models.ErrorType {
	log.info("Showing wallet status")

	// Parse flags from os.args (starting after "wallet status")
	flags, flags_err := parse_wallet_flags(os.args)
	if flags_err != .None {
		return flags_err
	}

	// Delegate to existing handle_wallet implementation
	return handle_wallet(flags)
}

// handle_wallet_switch switches the active (primary) wallet
//
// Parameters:
//   - identifier: Wallet address, partial address, or label
//
// Returns: ErrorType for error handling
handle_wallet_switch :: proc(identifier: string) -> models.ErrorType {
	log.infof("Switching to wallet: %s", identifier)

	// Open database
	db_path := token_cfg.get_database_path()
	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		return .DatabaseError
	}
	defer db.database_close(database)

	// Resolve target wallet (reuses existing function)
	target_wallet, wallet_err := resolve_target_wallet(database, identifier)
	if wallet_err != .None {
		return wallet_err
	}

	// Set as primary wallet
	switch_err := db.set_primary_wallet(database, target_wallet.address)
	if switch_err != .None {
		log.errorf("Failed to set primary wallet: %v", switch_err)
		return switch_err
	}

	// Display success
	output.print_success(fmt.tprintf("Switched to wallet: %s", target_wallet.label))
	fmt.printfln("Address: %s", target_wallet.address)
	fmt.println("")

	return .None
}

// handle_wallet_delete removes a wallet from the database after confirmation
//
// Safety features:
// 1. Requires user confirmation (typing wallet address)
// 2. Shows wallet details before deletion
// 3. Warns about irreversibility
// 4. Recommends keeping seed phrase backup
// 5. Cannot delete if it's the only wallet (must have at least one wallet)
//
// Params:
//   identifier: Wallet address (full/partial) or label to delete
//
// Returns: ErrorType for error handling
handle_wallet_delete :: proc(identifier: string) -> models.ErrorType {
	log.infof("Delete wallet requested: %s", identifier)

	// Open database
	db_path := token_cfg.get_database_path()
	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		return .DatabaseError
	}
	defer db.database_close(database)

	// Resolve target wallet
	target_wallet, wallet_err := resolve_target_wallet(database, identifier)
	if wallet_err != .None {
		return wallet_err
	}

	// Check if this is the only wallet (prevent deletion of last wallet)
	all_wallets, get_err := db.get_all_wallets(database)
	if get_err != .None {
		log.errorf("Failed to get wallets: %v", get_err)
		return get_err
	}

	if len(all_wallets) == 1 {
		fmt.println("You must have at least one wallet configured.")
		fmt.println("Import another wallet before deleting this one.")
		return .InvalidToken
	}

	// Display wallet details
	fmt.println("")
	fmt.println("⚠️  WARNING: You are about to delete this wallet:")
	fmt.println("")
	fmt.printfln("  Label:   %s", target_wallet.label)
	fmt.printfln("  Address: %s", target_wallet.address)
	fmt.printfln("  Type:    %v", target_wallet.wallet_type)
	if target_wallet.is_primary {
		fmt.println("  Status:  PRIMARY WALLET")
	}
	fmt.println("")
	fmt.println("⚠️  This operation is IRREVERSIBLE!")
	fmt.println("")
	fmt.println("Make sure you have your seed phrase backed up securely.")
	fmt.println("Without the seed phrase, you will lose access to your funds forever.")
	fmt.println("")

	// Require confirmation by typing the address
	fmt.println("To confirm deletion, type the wallet address and press Enter:")
	fmt.printf("> ")

	// Read user input
	buf: [1024]byte
	n, read_err := os.read(os.stdin, buf[:])
	if read_err != 0 {
		log.errorf("Failed to read confirmation: %v", read_err)
		return .InvalidToken
	}

	confirmation := strings.trim_space(string(buf[:n]))

	// Check if confirmation matches address
	if confirmation != target_wallet.address {
		fmt.println("Deletion cancelled.")
		return .InvalidToken
	}

	log.info("Confirmation received, proceeding with deletion")

	// If deleting primary wallet, we need to set another wallet as primary first
	if target_wallet.is_primary {
		log.info("Deleting primary wallet, finding replacement")

		// Find another wallet to set as primary
		new_primary_wallet: models.Wallet
		found_replacement := false

		for w in all_wallets {
			if w.address != target_wallet.address {
				new_primary_wallet = w
				found_replacement = true
				break
			}
		}

		if !found_replacement {
			// This should not happen due to earlier check, but be safe
			return .DatabaseError
		}

		// Set new primary wallet before deleting
		log.infof("Setting new primary wallet: %s", new_primary_wallet.label)
		set_primary_err := db.set_primary_wallet(database, new_primary_wallet.address)
		if set_primary_err != .None {
			log.errorf("Failed to set new primary wallet: %v", set_primary_err)
			return set_primary_err
		}

		fmt.println("")
		fmt.printfln("Set %s as new primary wallet", new_primary_wallet.label)
	}

	// Delete the wallet
	delete_err := db.delete_wallet(database, target_wallet.address)
	if delete_err != .None {
		log.errorf("Failed to delete wallet: %v", delete_err)
		return delete_err
	}

	// Display success
	fmt.println("")
	output.print_success(fmt.tprintf("Deleted wallet: %s", target_wallet.label))
	fmt.printfln("Address: %s", target_wallet.address)
	fmt.println("")
	fmt.println("✓ Wallet removed from Hound")
	fmt.println("✓ Encrypted keys deleted")
	fmt.println("")
	fmt.println("Remember: Your seed phrase controls your funds.")
	fmt.println("You can re-import this wallet anytime using the seed phrase.")
	fmt.println("")

	return .None
}
