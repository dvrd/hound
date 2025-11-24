// HTTP client structured logging
// Provides comprehensive request/response logging with timing and error context
package client

import "core:fmt"
import "core:log"
import "core:strings"
import "core:time"
import http ".."

// Request context for logging
Request_Context :: struct {
	method:     string,
	url:        string,
	attempt:    int,
	start_time: time.Time,
}

// Retry statistics for logging
Retry_Stats :: struct {
	attempt_count:  int,
	total_duration: time.Duration,
	last_error:     Error,
	retry_reasons:  [dynamic]string, // History of why retries occurred
}

// Log request start
log_request_start :: proc(ctx: Request_Context, config: Logging_Config) {
	if !config.enabled {
		return
	}

	if config.log_level > log.Level.Debug {
		return
	}

	if ctx.attempt > 1 {
		log.debugf("→ [HTTP] %s %s (attempt %d)", ctx.method, ctx.url, ctx.attempt)
	} else {
		log.debugf("→ [HTTP] %s %s", ctx.method, ctx.url)
	}
}

// Log request headers
log_request_headers :: proc(headers: http.Headers, config: Logging_Config) {
	if !config.enabled || !config.log_request_headers {
		return
	}

	log.debug("  Request Headers:")
	for key, values in headers._kv {
		for value in values {
			log.debugf("    %s: %s", key, value)
		}
	}
}

// Log request body
log_request_body :: proc(body: []byte, config: Logging_Config) {
	if !config.enabled || !config.log_request_body {
		return
	}

	if len(body) == 0 {
		return
	}

	body_str := string(body)
	if len(body_str) > config.max_body_log_bytes {
		log.debugf(
			"  Request Body: %s... (truncated, %d bytes total)",
			body_str[:config.max_body_log_bytes],
			len(body_str),
		)
	} else {
		log.debugf("  Request Body: %s", body_str)
	}
}

// Log response success
log_response_success :: proc(
	ctx: Request_Context,
	status: http.Status,
	duration: time.Duration,
	config: Logging_Config,
) {
	if !config.enabled {
		return
	}

	if config.log_level > log.Level.Info {
		return
	}

	duration_ms := f64(duration) / f64(time.Millisecond)

	if ctx.attempt > 1 {
		log.infof(
			"← [HTTP] %s %s → %v (%.2fms) [attempt %d]",
			ctx.method,
			ctx.url,
			status,
			duration_ms,
			ctx.attempt,
		)
	} else {
		log.infof("← [HTTP] %s %s → %v (%.2fms)", ctx.method, ctx.url, status, duration_ms)
	}
}

// Log response error
log_response_error :: proc(
	ctx: Request_Context,
	err: Error,
	duration: time.Duration,
	retry_stats: Retry_Stats,
	config: Logging_Config,
) {
	if !config.enabled {
		return
	}

	duration_ms := f64(duration) / f64(time.Millisecond)

	if retry_stats.attempt_count > 1 {
		log.errorf(
			"✗ [HTTP] %s %s → ERROR: %v (%.2fms, %d attempts)",
			ctx.method,
			ctx.url,
			err,
			duration_ms,
			retry_stats.attempt_count,
		)

		// Log retry history
		if len(retry_stats.retry_reasons) > 0 {
			log.debug("  Retry history:")
			for reason, i in retry_stats.retry_reasons {
				log.debugf("    Attempt %d: %s", i + 1, reason)
			}
		}
	} else {
		log.errorf("✗ [HTTP] %s %s → ERROR: %v (%.2fms)", ctx.method, ctx.url, err, duration_ms)
	}
}

// Log response headers
log_response_headers :: proc(headers: http.Headers, config: Logging_Config) {
	if !config.enabled || !config.log_response_headers {
		return
	}

	log.debug("  Response Headers:")
	for key, values in headers._kv {
		for value in values {
			log.debugf("    %s: %s", key, value)
		}
	}
}

// Log response body
log_response_body :: proc(body: string, config: Logging_Config) {
	if !config.enabled || !config.log_response_body {
		return
	}

	if len(body) == 0 {
		return
	}

	if len(body) > config.max_body_log_bytes {
		log.debugf(
			"  Response Body: %s... (truncated, %d bytes total)",
			body[:config.max_body_log_bytes],
			len(body),
		)
	} else {
		log.debugf("  Response Body: %s", body)
	}
}
