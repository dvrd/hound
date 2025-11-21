#!/usr/bin/env ruby
# Pre-commit hook: Reminder for version management on master branch
# This hook is automatically run before each commit

# ANSI color codes
RED = "\033[0;31m"
GREEN = "\033[0;32m"
YELLOW = "\033[1;33m"
BLUE = "\033[0;34m"
NC = "\033[0m" # No Color

def run_command(cmd)
  output = `#{cmd} 2>&1`
  success = $?.success?
  [success, output.strip]
end

def current_branch
  success, output = run_command('git rev-parse --abbrev-ref HEAD')
  return nil unless success
  output
end

def on_master?
  branch = current_branch
  branch == 'master' || branch == 'main'
end

def version_files_staged?
  success, output = run_command('git diff --cached --name-only')
  return false unless success

  files = output.lines.map(&:strip)
  files.include?('VERSION') || files.include?('src/version/version.odin')
end

# Main logic: check if version was updated on master commits
if on_master? && !version_files_staged?
  puts ""
  puts "#{YELLOW}════════════════════════════════════════════════#{NC}"
  puts "#{YELLOW}⚠  Committing to master without version bump  ⚠#{NC}"
  puts "#{YELLOW}════════════════════════════════════════════════#{NC}"
  puts ""
  puts "#{BLUE}Consider bumping version before committing:#{NC}"
  puts "  • #{GREEN}task version:patch#{NC} - Bug fixes (0.0.X)"
  puts "  • #{GREEN}task version:minor#{NC} - New features (0.X.0)"
  puts "  • #{GREEN}task version:major#{NC} - Breaking changes (X.0.0)"
  puts ""
  puts "#{BLUE}Or abort this commit with:#{NC} Ctrl+C"
  puts ""

  # Give user 3 seconds to cancel
  print "Continuing in 3 seconds..."
  sleep 1
  print " 2..."
  sleep 1
  print " 1..."
  sleep 1
  puts ""
  puts ""
end

# Allow commit to proceed
exit 0
