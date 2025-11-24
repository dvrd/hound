#!/usr/bin/env ruby
# Script to generate macOS icon from an image file
# Usage: ruby scripts/update_icon_from_image.rb <image_path>

require 'pathname'
require 'fileutils'

def run_command(cmd, silent: false)
  redirect = silent ? ' > /dev/null 2>&1' : ''
  output = `#{cmd}#{redirect}`
  success = $?.success?
  [success, output.strip]
end

def get_project_root
  script_dir = Pathname.new(__FILE__).dirname
  script_dir.parent
end

def validate_input(input_image)
  if input_image.nil?
    warn "Usage: ruby scripts/update_icon_from_image.rb <image_path>"
    exit 1
  end

  image_path = Pathname.new(input_image)
  unless image_path.exist?
    warn "Error: Image file not found: #{input_image}"
    exit 1
  end

  image_path
end

def setup_directories(project_root)
  iconset_dir = project_root / 'AppIcon.iconset'
  resources_dir = project_root / 'resources'

  FileUtils.mkdir_p(iconset_dir)
  FileUtils.mkdir_p(resources_dir)

  {
    iconset: iconset_dir,
    resources: resources_dir
  }
end

def generate_icon_sizes(input_image, iconset_dir)
  puts "🎨 Generating Hound icon from: #{input_image}"

  # Define all required sizes for macOS iconset
  # Format: [size, filename]
  sizes = [
    [16, 'icon_16x16.png'],
    [32, 'icon_16x16@2x.png'],
    [32, 'icon_32x32.png'],
    [64, 'icon_32x32@2x.png'],
    [128, 'icon_128x128.png'],
    [256, 'icon_128x128@2x.png'],
    [256, 'icon_256x256.png'],
    [512, 'icon_256x256@2x.png'],
    [512, 'icon_512x512.png'],
    [1024, 'icon_512x512@2x.png']
  ]

  sizes.each do |size, filename|
    output_path = iconset_dir / filename
    success, = run_command(
      "sips -z #{size} #{size} '#{input_image}' --out '#{output_path}' -s format png",
      silent: true
    )

    if success
      puts "  ✓ Created #{filename} (#{size}x#{size}px)"
    else
      warn "  ✗ Failed to create #{filename}"
    end
  end
end

def create_icns(iconset_dir, resources_dir)
  icns_path = resources_dir / 'AppIcon.icns'
  success, output = run_command("iconutil -c icns '#{iconset_dir}' -o '#{icns_path}'")

  unless success
    warn "Error creating icns file"
    warn output
    exit 1
  end

  puts ""
  puts "✅ Icon created: #{icns_path}"
  icns_path
end

def copy_to_app_bundle(project_root, icns_path)
  app_resources = project_root / 'Hound.app' / 'Contents' / 'Resources'
  return unless app_resources.directory?

  target = app_resources / 'AppIcon.icns'
  FileUtils.cp(icns_path, target)
  puts "   Copied to: Hound.app/Contents/Resources/AppIcon.icns"
end

def create_preview(input_image, resources_dir)
  preview_path = resources_dir / 'icon_preview.png'
  run_command(
    "sips -z 512 512 '#{input_image}' --out '#{preview_path}' -s format png",
    silent: true
  )
  puts "   Preview: #{preview_path}"
end

def cleanup(iconset_dir)
  FileUtils.rm_rf(iconset_dir)
end

def main
  # Validate input
  input_image = validate_input(ARGV[0])

  # Get project root
  project_root = get_project_root

  # Setup directories
  dirs = setup_directories(project_root)

  # Generate all icon sizes
  generate_icon_sizes(input_image, dirs[:iconset])

  # Convert iconset to icns
  icns_path = create_icns(dirs[:iconset], dirs[:resources])

  # Copy to app bundle if it exists
  copy_to_app_bundle(project_root, icns_path)

  # Create preview PNG
  create_preview(input_image, dirs[:resources])

  # Cleanup
  cleanup(dirs[:iconset])

  puts ""
  puts "🐕 Icon updated successfully!"
end

# Run main with error handling
begin
  main
rescue StandardError => e
  warn "Error: #{e.message}"
  exit 1
end
