#+feature global-context
package tests

import "core:testing"
import "core:fmt"
import "core:os"
import "core:strings"

// =============================================================================
// APP BUNDLE VALIDATION TESTS
// =============================================================================
// These tests validate the macOS .app bundle structure after it's been built.
// They check for proper directory structure, required files, permissions,
// and Info.plist configuration.
//
// Prerequisites: Run `task menubar:bundle` before running these tests.
// =============================================================================

@(test)
test_app_bundle_structure_exists :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify basic app bundle directory structure exists
	//
	// Expected structure:
	// Hound.app/
	//   Contents/
	//     Info.plist
	//     MacOS/
	//       Hound
	//     Resources/
	//       AppIcon.icns

	testing.expect(t, os.is_dir("Hound.app"),
		"Hound.app directory should exist (run 'task menubar:bundle' first)")

	testing.expect(t, os.is_dir("Hound.app/Contents"),
		"Hound.app/Contents directory should exist")

	testing.expect(t, os.is_dir("Hound.app/Contents/MacOS"),
		"Hound.app/Contents/MacOS directory should exist")

	testing.expect(t, os.is_dir("Hound.app/Contents/Resources"),
		"Hound.app/Contents/Resources directory should exist")
}

@(test)
test_info_plist_exists_and_valid :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify Info.plist exists and contains required keys
	//
	// The Info.plist must exist and be a valid property list file.
	// Use plutil -lint to validate syntax.

	plist_path := "Hound.app/Contents/Info.plist"

	testing.expect(t, os.is_file(plist_path),
		fmt.tprintf("Info.plist should exist at %s", plist_path))

	// Read plist content to verify it contains key identifiers
	data, ok := os.read_entire_file(plist_path)
	if !ok {
		testing.expect(t, false, "Failed to read Info.plist")
		return
	}
	defer delete(data)

	content := string(data)

	testing.expect(t, strings.contains(content, "CFBundleIdentifier"),
		"Info.plist should contain CFBundleIdentifier key")

	testing.expect(t, strings.contains(content, "com.hound.app"),
		"Info.plist should contain bundle identifier 'com.hound.app'")

	testing.expect(t, strings.contains(content, "CFBundleExecutable"),
		"Info.plist should contain CFBundleExecutable key")

	testing.expect(t, strings.contains(content, "<string>Hound</string>"),
		"Info.plist should specify 'Hound' as executable name")

	testing.expect(t, strings.contains(content, "LSUIElement"),
		"Info.plist should contain LSUIElement key for menu bar app")
}

@(test)
test_binary_exists_and_executable :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify binary exists in MacOS directory with +x permissions
	//
	// The binary must:
	// 1. Exist at Contents/MacOS/Hound
	// 2. Have executable permissions (mode includes 0111)

	binary_path := "Hound.app/Contents/MacOS/Hound"

	testing.expect(t, os.is_file(binary_path),
		fmt.tprintf("Binary should exist at %s", binary_path))

	// Check if file is executable by checking file info
	file_info, err := os.stat(binary_path)
	if err != 0 {
		testing.expect(t, false, fmt.tprintf("Failed to stat binary: %v", err))
		return
	}

	// On Unix, check if any execute bit is set (owner, group, or other)
	when ODIN_OS == .Darwin {
		mode := file_info.mode
		// Check if any execute bit is set (user: 0100, group: 0010, other: 0001)
		has_execute := (mode & 0o111) != 0
		testing.expect(t, has_execute,
			fmt.tprintf("Binary should have executable permissions, mode: %o", mode))
	}
}

@(test)
test_icon_exists :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify app icon exists in Resources directory
	//
	// The AppIcon.icns file should exist at Contents/Resources/AppIcon.icns

	icon_path := "Hound.app/Contents/Resources/AppIcon.icns"

	testing.expect(t, os.is_file(icon_path),
		fmt.tprintf("App icon should exist at %s", icon_path))

	// Verify file is not empty
	file_info, err := os.stat(icon_path)
	if err != 0 {
		testing.expect(t, false, "Failed to stat icon file")
		return
	}

	testing.expect(t, file_info.size > 0,
		"App icon file should not be empty")
}

@(test)
test_version_matches :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify version in Info.plist matches VERSION file
	//
	// The CFBundleVersion and CFBundleShortVersionString in Info.plist
	// should match the version specified in the VERSION file.

	// Read VERSION file
	version_data, ok := os.read_entire_file("VERSION")
	if !ok {
		testing.expect(t, false, "Failed to read VERSION file")
		return
	}
	defer delete(version_data)

	version := strings.trim_space(string(version_data))

	// Read Info.plist
	plist_path := "Hound.app/Contents/Info.plist"
	plist_data, plist_ok := os.read_entire_file(plist_path)
	if !plist_ok {
		testing.expect(t, false, "Failed to read Info.plist")
		return
	}
	defer delete(plist_data)

	plist_content := string(plist_data)

	// Check if version appears in plist (basic check)
	testing.expect(t, strings.contains(plist_content, version),
		fmt.tprintf("Info.plist should contain version '%s'", version))
}
