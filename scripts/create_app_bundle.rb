#!/usr/bin/env ruby
# Script to create macOS .app bundle from Odin binary
# Usage: ruby scripts/create_app_bundle.rb

require 'pathname'
require 'fileutils'

# ANSI color codes
RED = "\033[0;31m"
GREEN = "\033[0;32m"
YELLOW = "\033[1;33m"
NC = "\033[0m" # No Color

def run_command(cmd)
  output = `#{cmd} 2>&1`
  success = $?.success?
  [success, output.strip]
end

def get_project_root
  script_dir = Pathname.new(__FILE__).dirname
  script_dir.parent
end

def read_version(project_root)
  version_file = project_root / 'VERSION'
  unless version_file.exist?
    warn "#{RED}✗#{NC} VERSION file not found"
    exit 1
  end

  version = version_file.read.strip
  puts "#{GREEN}✓#{NC} Building version: #{version}"
  version
end

def build_binary(project_root)
  puts "#{YELLOW}⚙#{NC} Building Odin binary..."

  bin_dir = project_root / 'bin'
  FileUtils.mkdir_p(bin_dir)

  success, output = run_command(
    'odin build src/menubar -out:bin/hound-menubar -o:speed ' \
    '-extra-linker-flags:"-framework AppKit -framework Foundation -lsqlite3"'
  )

  binary_path = bin_dir / 'hound-menubar'
  unless binary_path.exist?
    warn "#{RED}✗#{NC} Build failed - binary not created"
    warn output unless output.empty?
    exit 1
  end

  puts "#{GREEN}✓#{NC} Binary built successfully"
  binary_path
end

def create_bundle_structure(project_root)
  puts "#{YELLOW}⚙#{NC} Creating bundle structure..."

  app_dir = project_root / 'Hound.app'
  contents_dir = app_dir / 'Contents'
  macos_dir = contents_dir / 'MacOS'
  resources_dir = contents_dir / 'Resources'

  FileUtils.mkdir_p(macos_dir)
  FileUtils.mkdir_p(resources_dir)

  puts "#{GREEN}✓#{NC} Bundle directories created"

  {
    app: app_dir,
    contents: contents_dir,
    macos: macos_dir,
    resources: resources_dir
  }
end

def copy_binary(binary_path, macos_dir, app_name)
  puts "#{YELLOW}⚙#{NC} Copying binary to bundle..."

  target = macos_dir / app_name
  FileUtils.cp(binary_path, target)
  FileUtils.chmod(0755, target)

  puts "#{GREEN}✓#{NC} Binary copied"
  puts "#{GREEN}✓#{NC} Executable permissions set"
end

def generate_info_plist(project_root, contents_dir, version)
  puts "#{YELLOW}⚙#{NC} Generating Info.plist..."

  template_path = project_root / 'resources' / 'Info.plist.template'
  unless template_path.exist?
    warn "#{RED}✗#{NC} Info.plist.template not found"
    exit 1
  end

  template = template_path.read
  plist_content = template.gsub('${VERSION}', version)

  plist_path = contents_dir / 'Info.plist'
  plist_path.write(plist_content)

  puts "#{GREEN}✓#{NC} Info.plist generated"
  plist_path
end

def copy_icon(project_root, resources_dir)
  puts "#{YELLOW}⚙#{NC} Copying app icon..."

  icon_path = project_root / 'resources' / 'AppIcon.icns'
  unless icon_path.exist?
    puts "#{YELLOW}⚠#{NC} AppIcon.icns not found, skipping"
    return
  end

  FileUtils.cp(icon_path, resources_dir / 'AppIcon.icns')
  puts "#{GREEN}✓#{NC} Icon copied"
end

def validate_plist(plist_path)
  puts "#{YELLOW}⚙#{NC} Validating Info.plist..."

  success, output = run_command("plutil -lint #{plist_path}")
  unless success
    warn "#{RED}✗#{NC} Info.plist validation failed"
    warn output
    exit 1
  end

  puts "#{GREEN}✓#{NC} Info.plist is valid"
end

def sign_bundle(app_dir)
  puts "#{YELLOW}⚙#{NC} Self-signing bundle (optional)..."

  success, = run_command("codesign -s - #{app_dir} 2>/dev/null")
  if success
    puts "#{GREEN}✓#{NC} Bundle signed"
  else
    puts "#{YELLOW}⚠#{NC} Signing skipped (no developer tools)"
  end
end

def get_bundle_size(app_dir)
  success, output = run_command("du -sh #{app_dir}")
  return 'unknown' unless success
  output.split("\t").first
end

def main
  project_root = get_project_root
  Dir.chdir(project_root)

  # Read version
  version = read_version(project_root)

  # Build binary
  binary_path = build_binary(project_root)

  # Create bundle structure
  dirs = create_bundle_structure(project_root)

  # Copy binary
  app_name = 'Hound'
  copy_binary(binary_path, dirs[:macos], app_name)

  # Generate Info.plist
  plist_path = generate_info_plist(project_root, dirs[:contents], version)

  # Copy icon
  copy_icon(project_root, dirs[:resources])

  # Validate plist
  validate_plist(plist_path)

  # Sign bundle
  sign_bundle(dirs[:app])

  # Success message
  bundle_size = get_bundle_size(dirs[:app])
  puts ""
  puts "#{GREEN}✅ Bundle created successfully!#{NC}"
  puts "   Location: #{dirs[:app]}"
  puts "   Size: #{bundle_size}"
  puts "   Version: #{version}"
  puts ""
  puts "To test: #{YELLOW}open #{dirs[:app]}#{NC}"
end

# Run main with error handling
begin
  main
rescue StandardError => e
  warn "#{RED}Error: #{e.message}#{NC}"
  exit 1
end
