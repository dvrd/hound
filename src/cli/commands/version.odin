// Version command implementation
// Displays version information
package commands

import "core:fmt"
import "core:log"
import models "../../src/lib/models"
import version "../../src/version"

// ============================================================================
// Version Command
// ============================================================================

// handle_version implements the version display command
//
// Handles: --version, -v, version
// Displays: Version string from version.odin
// Returns: ErrorType for consistent error handling
handle_version :: proc() -> models.ErrorType {
	log.debug("Version request")
	fmt.println(version.get_version_info())
	return .None
}
