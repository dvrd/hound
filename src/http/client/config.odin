// HTTP client configuration
// Provides timeout, retry, and logging configuration for enhanced HTTP client operations
package client

import "core:log"
import "core:slice"
import "core:time"

// HTTP client configuration
Config :: struct {
	timeout: Timeout_Config,
	retry:   Retry_Config,
	logging: Logging_Config,
}

// Timeout configuration
Timeout_Config :: struct {
	enabled:         bool, // Enable timeout mechanism
	connect_timeout: time.Duration, // Connection establishment timeout
	request_timeout: time.Duration, // Total request timeout (connect + read + write)
}

// Retry configuration
Retry_Config :: struct {
	enabled:            bool, // Enable retry mechanism
	max_attempts:       int, // Maximum retry attempts (1 = no retry, 3 = 2 retries)
	initial_backoff:    time.Duration, // Initial backoff duration
	max_backoff:        time.Duration, // Maximum backoff duration
	backoff_multiplier: f64, // Backoff multiplier (exponential: 2.0, linear: 1.0)
	retryable_errors:   []Retryable_Error_Type, // Which errors trigger retry
}

// Retryable error types
Retryable_Error_Type :: enum {
	Network_Timeout,
	Connection_Failed,
	Rate_Limited, // HTTP 429
	Server_Error, // HTTP 500-599
}

// Logging configuration
Logging_Config :: struct {
	enabled:             bool, // Enable enhanced logging
	log_level:           log.Level, // Minimum log level
	log_request_headers:  bool, // Log request headers
	log_response_headers: bool, // Log response headers
	log_request_body:     bool, // Log request body (be careful with sensitive data)
	log_response_body:    bool, // Log response body (limited to first N bytes)
	max_body_log_bytes:   int, // Max bytes to log from body
}

// Default configuration - Balanced for general use
DEFAULT_CONFIG :: Config {
	timeout = Timeout_Config {
		enabled         = true,
		connect_timeout = 5 * time.Second,
		request_timeout = 30 * time.Second,
	},
	retry = Retry_Config {
		enabled            = true,
		max_attempts       = 3, // 1 initial + 2 retries
		initial_backoff    = 500 * time.Millisecond,
		max_backoff        = 5 * time.Second,
		backoff_multiplier = 2.0, // Exponential backoff
		retryable_errors   = {
			.Network_Timeout,
			.Connection_Failed,
			.Rate_Limited,
			.Server_Error,
		},
	},
	logging = Logging_Config {
		enabled              = true,
		log_level            = log.Level.Debug,
		log_request_headers  = false,
		log_response_headers = false,
		log_request_body     = false,
		log_response_body    = false,
		max_body_log_bytes   = 500,
	},
}

// Production configuration - Optimized for production environments
PRODUCTION_CONFIG :: Config {
	timeout = Timeout_Config {
		enabled         = true,
		connect_timeout = 5 * time.Second,
		request_timeout = 30 * time.Second,
	},
	retry = Retry_Config {
		enabled            = true,
		max_attempts       = 3,
		initial_backoff    = 1 * time.Second,
		max_backoff        = 10 * time.Second,
		backoff_multiplier = 2.0,
		retryable_errors   = {
			.Network_Timeout,
			.Connection_Failed,
			.Server_Error,
		},
	},
	logging = Logging_Config {
		enabled              = true,
		log_level            = log.Level.Info, // Less verbose
		log_request_headers  = false,
		log_response_headers = false,
		log_request_body     = false,
		log_response_body    = false,
		max_body_log_bytes   = 200,
	},
}

// Development configuration - Verbose logging, no retry for easier debugging
DEVELOPMENT_CONFIG :: Config {
	timeout = Timeout_Config {
		enabled         = true,
		connect_timeout = 10 * time.Second,
		request_timeout = 60 * time.Second, // Longer for debugging
	},
	retry = Retry_Config {
		enabled            = false, // Disable retry for debugging
		max_attempts       = 1,
		initial_backoff    = 0,
		max_backoff        = 0,
		backoff_multiplier = 1.0,
		retryable_errors   = {},
	},
	logging = Logging_Config {
		enabled              = true,
		log_level            = log.Level.Debug, // Verbose
		log_request_headers  = true,
		log_response_headers = true,
		log_request_body     = true,
		log_response_body    = true,
		max_body_log_bytes   = 2000,
	},
}
