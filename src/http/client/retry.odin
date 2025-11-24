// HTTP client retry logic with exponential backoff
// Handles automatic retry for transient failures with configurable backoff strategy
package client

import "core:fmt"
import "core:log"
import "core:net"
import "core:slice"
import "core:time"
import http ".."

// Determine if error is retryable based on configuration
is_retryable_error :: proc(err: Error, config: Retry_Config) -> bool {
	if err == nil {
		return false
	}

	#partial switch e in err {
	case net.Network_Error:
		return slice.contains(config.retryable_errors[:], Retryable_Error_Type.Network_Timeout)

	case net.Dial_Error, net.TCP_Send_Error:
		return slice.contains(
			config.retryable_errors[:],
			Retryable_Error_Type.Connection_Failed,
		)

	case:
		return false
	}
}

// Determine if HTTP status code is retryable
is_retryable_status :: proc(status: http.Status, config: Retry_Config) -> bool {
	#partial switch status {
	case .Too_Many_Requests: // 429
		return slice.contains(config.retryable_errors[:], Retryable_Error_Type.Rate_Limited)

	case .Internal_Server_Error, .Bad_Gateway, .Service_Unavailable, .Gateway_Timeout: // 500-504
		return slice.contains(config.retryable_errors[:], Retryable_Error_Type.Server_Error)

	case:
		return false
	}
}

// Calculate backoff duration with exponential backoff
calculate_backoff :: proc(attempt: int, config: Retry_Config) -> time.Duration {
	backoff := f64(config.initial_backoff)

	// Apply multiplier for each retry attempt
	for i := 1; i < attempt; i += 1 {
		backoff *= config.backoff_multiplier
	}

	// Cap at max_backoff
	if backoff > f64(config.max_backoff) {
		backoff = f64(config.max_backoff)
	}

	return time.Duration(backoff)
}

// Execute request with retry logic
// Note: This is a simplified version that doesn't handle timeouts yet.
// The full implementation will be integrated when we update client.odin
request_with_retry :: proc(
	req: ^Request,
	target: string,
	retry_config: Retry_Config,
	allocator := context.allocator,
) -> (
	res: Response,
	err: Error,
	stats: Retry_Stats,
) {
	stats.attempt_count = 0
	stats.retry_reasons = make([dynamic]string, allocator)
	start_time := time.now()
	defer stats.total_duration = time.since(start_time)

	if !retry_config.enabled || retry_config.max_attempts <= 1 {
		// No retry - single attempt
		res, err = request(req, target, allocator)
		stats.attempt_count = 1
		stats.last_error = err
		return
	}

	// Retry loop
	for attempt := 1; attempt <= retry_config.max_attempts; attempt += 1 {
		stats.attempt_count = attempt

		log.debugf("Request attempt %d/%d: %s", attempt, retry_config.max_attempts, target)

		// Execute request
		res, err = request(req, target, allocator)

		// Success case
		if err == nil {
			// Check for retryable HTTP status codes
			if !is_retryable_status(res.status, retry_config) {
				log.debugf("Request succeeded on attempt %d", attempt)
				return res, nil, stats
			}

			// Retryable status code
			reason := fmt.tprintf("HTTP %v (retryable status)", res.status)
			append(&stats.retry_reasons, reason)
			log.warnf("Attempt %d failed: %s", attempt, reason)

			// Clean up response before retry
			response_destroy(&res)
		} else {
			// Error case
			stats.last_error = err

			// Check if error is retryable
			if !is_retryable_error(err, retry_config) {
				log.debugf("Non-retryable error on attempt %d: %v", attempt, err)
				return {}, err, stats
			}

			reason := fmt.tprintf("Error: %v", err)
			append(&stats.retry_reasons, reason)
			log.warnf("Attempt %d failed: %s", attempt, reason)
		}

		// Last attempt - don't wait
		if attempt == retry_config.max_attempts {
			log.errorf("All %d attempts exhausted for: %s", retry_config.max_attempts, target)
			return res, err, stats
		}

		// Calculate and apply backoff
		backoff := calculate_backoff(attempt, retry_config)
		log.debugf("Waiting %v before retry %d", backoff, attempt + 1)
		time.sleep(backoff)
	}

	return
}
