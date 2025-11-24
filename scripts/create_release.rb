#!/usr/bin/env ruby
# Automated release creation script for Hound
# Usage: ruby scripts/create_release.rb <version> [--push]
#
# Example: ruby scripts/create_release.rb 0.7.0 --push

require 'pathname'

# ANSI color codes
RED = "\033[0;31m"
GREEN = "\033[0;32m"
YELLOW = "\033[1;33m"
BLUE = "\033[0;34m"
NC = "\033[0m" # No Color

def run_command(cmd, silent: false)
  redirect = silent ? ' > /dev/null 2>&1' : ' 2>&1'
  output = `#{cmd}#{redirect}`
  success = $?.success?
  [success, output.strip]
end

def get_project_root
  script_dir = Pathname.new(__FILE__).dirname
  script_dir.parent
end

def validate_version(version)
  unless version
    warn "#{RED}Error: Version number required#{NC}"
    puts "Usage: ruby scripts/create_release.rb <version> [--push]"
    puts "Example: ruby scripts/create_release.rb 0.7.0 --push"
    exit 1
  end

  unless version.match?(/^\d+\.\d+\.\d+$/)
    warn "#{RED}Error: Version must be in semver format (e.g., 0.7.0)#{NC}"
    exit 1
  end
end

def update_version(version)
  puts "#{YELLOW}[1/7]#{NC} Updating version files..."
  success, output = run_command("ruby scripts/update_version.rb #{version}")

  unless success
    warn "#{RED}✗#{NC} Failed to update version"
    warn output
    exit 1
  end

  puts "#{GREEN}✓#{NC} Version files updated"
  puts ""
end

def build_menubar
  puts "#{YELLOW}[2/7]#{NC} Building menubar app..."
  success, = run_command('task menubar:build', silent: true)

  unless success
    warn "#{RED}✗#{NC} Build failed"
    exit 1
  end

  puts "#{GREEN}✓#{NC} Binary built"
  puts ""
end

def create_bundle
  puts "#{YELLOW}[3/7]#{NC} Creating app bundle..."
  success, = run_command('task menubar:bundle', silent: true)

  unless success
    warn "#{RED}✗#{NC} Bundle creation failed"
    exit 1
  end

  puts "#{GREEN}✓#{NC} Bundle created: Hound.app"
  puts ""
end

def create_dmg(version)
  puts "#{YELLOW}[4/7]#{NC} Creating DMG installer..."
  success, = run_command('task menubar:dmg', silent: true)

  dmg_file = "Hound-#{version}.dmg"
  unless success || File.exist?(dmg_file)
    warn "#{RED}✗#{NC} DMG creation failed"
    exit 1
  end

  puts "#{GREEN}✓#{NC} DMG created: #{dmg_file}"
  puts ""
  dmg_file
end

def generate_checksum(dmg_file)
  puts "#{YELLOW}[5/7]#{NC} Generating SHA256 checksum..."

  success, output = run_command("shasum -a 256 #{dmg_file}")
  unless success
    warn "#{RED}✗#{NC} Checksum generation failed"
    exit 1
  end

  checksum = output.split(' ').first
  checksum_file = "#{dmg_file}.sha256"
  File.write(checksum_file, output + "\n")

  puts "#{GREEN}✓#{NC} Checksum: #{checksum}"
  puts ""
  checksum
end

def create_git_commit_and_tag(version)
  puts "#{YELLOW}[6/7]#{NC} Creating git commits and tag..."

  # Commit version bump
  commit_message = <<~MSG
    chore: bump version to #{version}

    🤖 Generated with [Claude Code](https://claude.com/claude-code)

    Co-Authored-By: Claude <noreply@anthropic.com>
  MSG

  success, = run_command("git add VERSION src/version/version.odin", silent: true)
  unless success
    warn "#{RED}✗#{NC} Failed to stage version files"
    exit 1
  end

  success, = run_command("git commit -m '#{commit_message.gsub("'", "'\\''")}' > /dev/null 2>&1")
  unless success
    warn "#{RED}✗#{NC} Failed to create commit"
    exit 1
  end

  # Create annotated tag
  tag_message = <<~MSG
    Release v#{version}

    See RELEASE_NOTES_v#{version}.md for detailed changelog.

    🤖 Generated with [Claude Code](https://claude.com/claude-code)

    Co-Authored-By: Claude <noreply@anthropic.com>
  MSG

  success, = run_command("git tag -a 'v#{version}' -m '#{tag_message.gsub("'", "'\\''")}' 2>&1")
  unless success
    warn "#{RED}✗#{NC} Failed to create tag"
    exit 1
  end

  puts "#{GREEN}✓#{NC} Git tag created: v#{version}"
  puts ""
end

def print_summary(version, dmg_file, checksum)
  puts "#{YELLOW}[7/7]#{NC} Release summary"
  puts ""
  puts "#{GREEN}========================================#{NC}"
  puts "#{GREEN}  Release v#{version} Ready!#{NC}"
  puts "#{GREEN}========================================#{NC}"
  puts ""

  file_size = `ls -lh #{dmg_file} 2>/dev/null | awk '{print $5}'`.strip
  puts "📦 #{BLUE}Files created:#{NC}"
  puts "   - #{dmg_file} (#{file_size})"
  puts "   - #{dmg_file}.sha256"
  puts ""
  puts "🏷️  #{BLUE}Git tag:#{NC} v#{version}"
  puts ""
  puts "🔐 #{BLUE}SHA256:#{NC}"
  puts "   #{checksum}"
  puts ""
end

def push_to_remote(version, push_flag)
  if push_flag == '--push'
    puts "#{YELLOW}Pushing to remote...#{NC}"

    success, = run_command('git push origin master')
    unless success
      warn "#{RED}✗#{NC} Failed to push commits"
      exit 1
    end

    success, = run_command("git push origin v#{version}")
    unless success
      warn "#{RED}✗#{NC} Failed to push tag"
      exit 1
    end

    puts "#{GREEN}✓#{NC} Pushed commits and tag to origin"
    puts ""
    puts "#{GREEN}🎉 Release v#{version} published!#{NC}"
  else
    puts "#{YELLOW}⚠  Commits and tag created locally#{NC}"
    puts ""
    puts "To push to remote repository, run:"
    puts "  #{BLUE}git push origin master#{NC}"
    puts "  #{BLUE}git push origin v#{version}#{NC}"
    puts ""
    puts "Or run this script with #{BLUE}--push#{NC} flag:"
    puts "  #{BLUE}ruby scripts/create_release.rb #{version} --push#{NC}"
  end
end

def print_next_steps(version, dmg_file)
  puts ""
  puts "#{GREEN}Next steps:#{NC}"
  puts "  1. Create release notes in #{BLUE}RELEASE_NOTES_v#{version}.md#{NC}"
  puts "  2. Create GitHub release with tag #{BLUE}v#{version}#{NC}"
  puts "  3. Upload #{BLUE}#{dmg_file}#{NC} to GitHub release"
  puts "  4. Add SHA256 checksum to release notes"
  puts ""
end

def main
  # Parse arguments
  version = ARGV[0]
  push_flag = ARGV[1]

  # Validate version
  validate_version(version)

  puts "#{BLUE}========================================#{NC}"
  puts "#{BLUE}  Hound Release Automation v#{version}#{NC}"
  puts "#{BLUE}========================================#{NC}"
  puts ""

  # Change to project root
  project_root = get_project_root
  Dir.chdir(project_root)

  # Execute release steps
  update_version(version)
  build_menubar
  create_bundle
  dmg_file = create_dmg(version)
  checksum = generate_checksum(dmg_file)
  create_git_commit_and_tag(version)

  # Print summary
  print_summary(version, dmg_file, checksum)

  # Push to remote if requested
  push_to_remote(version, push_flag)

  # Print next steps
  print_next_steps(version, dmg_file)
end

# Run main with error handling
begin
  main
rescue StandardError => e
  warn "#{RED}Error: #{e.message}#{NC}"
  exit 1
end
