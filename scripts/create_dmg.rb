#!/usr/bin/env ruby
# Script to create DMG installer from .app bundle
# Usage: ruby scripts/create_dmg.rb

require 'pathname'
require 'fileutils'
require 'tmpdir'

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
  puts "#{GREEN}✓#{NC} Creating DMG for version: #{version}"
  version
end

def check_app_bundle(project_root)
  app_dir = project_root / 'Hound.app'
  unless app_dir.directory?
    warn "#{RED}✗#{NC} Hound.app not found. Run 'task menubar:bundle' first."
    exit 1
  end

  puts "#{GREEN}✓#{NC} App bundle found"
  app_dir
end

def remove_old_dmg(dmg_path)
  return unless dmg_path.exist?

  puts "#{YELLOW}⚙#{NC} Removing old DMG..."
  FileUtils.rm_f(dmg_path)
end

def create_staging_dir(app_dir)
  puts "#{YELLOW}⚙#{NC} Creating temporary staging directory..."

  tmp_dir = Pathname.new(Dir.mktmpdir)

  # Copy app to temp directory
  puts "#{YELLOW}⚙#{NC} Copying app bundle..."
  FileUtils.cp_r(app_dir, tmp_dir / 'Hound.app')
  puts "#{GREEN}✓#{NC} App bundle copied"

  # Create Applications symlink
  puts "#{YELLOW}⚙#{NC} Creating Applications symlink..."
  FileUtils.ln_s('/Applications', tmp_dir / 'Applications')
  puts "#{GREEN}✓#{NC} Symlink created"

  tmp_dir
end

def create_dmg(tmp_dir, volume_name, dmg_path)
  puts "#{YELLOW}⚙#{NC} Creating DMG (this may take a moment)..."

  success, output = run_command(
    "hdiutil create " \
    "-volname '#{volume_name}' " \
    "-srcfolder '#{tmp_dir}' " \
    "-ov " \
    "-format UDZO " \
    "'#{dmg_path}' > /dev/null 2>&1"
  )

  unless dmg_path.exist?
    warn "#{RED}✗#{NC} DMG creation failed"
    warn output unless output.empty?
    exit 1
  end

  puts "#{GREEN}✓#{NC} DMG created"
end

def cleanup(tmp_dir)
  FileUtils.rm_rf(tmp_dir)
  puts "#{GREEN}✓#{NC} Cleaned up temporary files"
end

def get_dmg_size(dmg_path)
  success, output = run_command("du -sh '#{dmg_path}'")
  return 'unknown' unless success
  output.split("\t").first
end

def main
  project_root = get_project_root
  Dir.chdir(project_root)

  # Read version
  version = read_version(project_root)

  # Set variables
  dmg_name = "Hound-#{version}.dmg"
  volume_name = "Hound #{version}"
  dmg_path = project_root / dmg_name

  # Check app bundle exists
  app_dir = check_app_bundle(project_root)

  # Remove old DMG if exists
  remove_old_dmg(dmg_path)

  # Create staging directory and prepare files
  tmp_dir = create_staging_dir(app_dir)

  begin
    # Create DMG
    create_dmg(tmp_dir, volume_name, dmg_path)

    # Cleanup
    cleanup(tmp_dir)

    # Success message
    dmg_size = get_dmg_size(dmg_path)
    puts ""
    puts "#{GREEN}✅ DMG created successfully!#{NC}"
    puts "   Location: #{dmg_name}"
    puts "   Size: #{dmg_size}"
    puts "   Volume: #{volume_name}"
    puts ""
    puts "To test: #{YELLOW}open #{dmg_name}#{NC}"
  rescue StandardError => e
    cleanup(tmp_dir) if tmp_dir
    raise e
  end
end

# Run main with error handling
begin
  main
rescue StandardError => e
  warn "#{RED}Error: #{e.message}#{NC}"
  exit 1
end
