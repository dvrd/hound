#!/usr/bin/env ruby
# Install git hooks for the hound project
# Run this script after cloning the repository

require 'pathname'
require 'fileutils'

# ANSI color codes
RED = "\033[0;31m"
GREEN = "\033[0;32m"
YELLOW = "\033[1;33m"
NC = "\033[0m" # No Color

def get_project_root
  script_dir = Pathname.new(__FILE__).dirname
  script_dir.parent
end

def main
  project_root = get_project_root
  git_dir = project_root / '.git'
  hooks_dir = git_dir / 'hooks'

  # Check if .git directory exists
  unless git_dir.directory?
    warn "#{RED}Error: Not a git repository#{NC}"
    warn "Make sure you're running this from the hound project directory"
    exit 1
  end

  puts "#{YELLOW}Installing git hooks...#{NC}"
  puts ""

  # Install pre-commit hook
  hook_src = project_root / 'scripts' / 'pre-commit.rb'
  hook_dst = hooks_dir / 'pre-commit'

  unless hook_src.exist?
    warn "#{RED}Error: Hook source not found at #{hook_src}#{NC}"
    exit 1
  end

  # Backup existing hook if present
  if hook_dst.exist?
    backup = "#{hook_dst}.backup.#{Time.now.to_i}"
    puts "#{YELLOW}Backing up existing pre-commit hook to:#{NC}"
    puts "  #{backup}"
    FileUtils.mv(hook_dst, backup)
    puts ""
  end

  # Install hook
  FileUtils.cp(hook_src, hook_dst)
  FileUtils.chmod(0755, hook_dst)

  puts "#{GREEN}✓ Installed pre-commit hook#{NC}"
  puts ""
  puts "The hook will automatically:"
  puts "  • Detect commits to master branch without version bump"
  puts "  • Show reminder with task commands"
  puts "  • Give 3 seconds to abort (Ctrl+C)"
  puts ""
  puts "To bump version before committing:"
  puts "  task version:patch  # Bug fixes (0.0.X)"
  puts "  task version:minor  # New features (0.X.0)"
  puts "  task version:major  # Breaking changes (X.0.0)"
  puts ""
  puts "#{GREEN}Installation complete!#{NC}"
end

# Run main with error handling
begin
  main
rescue StandardError => e
  warn "#{RED}Error: #{e.message}#{NC}"
  exit 1
end
