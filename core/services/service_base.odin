// Service base configuration
// Common structures and utilities shared by all service modules
package services

// ============================================================================
// Service Configuration
// ============================================================================

// ServiceConfig holds common configuration for all services
//
// This structure is passed to all service initialization functions to provide
// consistent access to shared resources like database paths and RPC endpoints.
ServiceConfig :: struct {
	db_path:              string,   // Path to SQLite database
	rpc_endpoint:         string,   // Primary Solana RPC endpoint
	backup_rpc_endpoints: []string, // Fallback RPC endpoints
}

// init_service_config creates a new service configuration
//
// ASSERTION 1: Database path must not be empty
// ASSERTION 2: RPC endpoint must not be empty
//
// Returns: Initialized service configuration
init_service_config :: proc(
	db_path: string,
	rpc_endpoint: string,
	backup_endpoints: []string = nil,
) -> ServiceConfig {
	assert(len(db_path) > 0, "Database path cannot be empty")
	assert(len(rpc_endpoint) > 0, "RPC endpoint cannot be empty")

	return ServiceConfig{
		db_path              = db_path,
		rpc_endpoint         = rpc_endpoint,
		backup_rpc_endpoints = backup_endpoints,
	}
}

// ============================================================================
// Service Context Pattern
// ============================================================================

// ServiceContext is a common pattern used by all services
//
// Services receive a context struct containing their dependencies:
// - Database handle
// - RPC client
// - Token configuration
// - Other service references
//
// This enables:
// - Dependency injection
// - Stateless service functions
// - Easy testing with mock contexts
//
// Example:
//   WalletServiceContext :: struct {
//       db:              ^database.Database,
//       rpc_client:      ^RPCClient,
//       balance_fetcher: ^BalanceFetcher,
//       config:          ^models.TokenConfig,
//   }
//
//   fetch_portfolio :: proc(
//       ctx: ^WalletServiceContext,
//       address: string,
//   ) -> (portfolio: Portfolio, err: models.ErrorType) {
//       assert(ctx != nil, "Context cannot be nil")
//       // Business logic using ctx.db, ctx.rpc_client, etc.
//   }
