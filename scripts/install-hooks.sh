#!/bin/bash
# Install git hooks for the hound project
# Run this script after cloning the repository

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
HOOKS_DIR="$PROJECT_ROOT/.git/hooks"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if .git directory exists
if [ ! -d "$PROJECT_ROOT/.git" ]; then
    echo -e "${RED}Error: Not a git repository${NC}"
    echo "Make sure you're running this from the hound project directory"
    exit 1
fi

echo -e "${YELLOW}Installing git hooks...${NC}"
echo ""

# Install pre-commit hook
HOOK_SRC="$SCRIPT_DIR/pre-commit"
HOOK_DST="$HOOKS_DIR/pre-commit"

if [ ! -f "$HOOK_SRC" ]; then
    echo -e "${RED}Error: Hook source not found at $HOOK_SRC${NC}"
    exit 1
fi

# Backup existing hook if present
if [ -f "$HOOK_DST" ]; then
    BACKUP="$HOOK_DST.backup.$(date +%s)"
    echo -e "${YELLOW}Backing up existing pre-commit hook to:${NC}"
    echo "  $BACKUP"
    mv "$HOOK_DST" "$BACKUP"
    echo ""
fi

# Install hook
cp "$HOOK_SRC" "$HOOK_DST"
chmod +x "$HOOK_DST"

echo -e "${GREEN}✓ Installed pre-commit hook${NC}"
echo ""
echo "The hook will automatically:"
echo "  • Detect when VERSION file changes"
echo "  • Sync src/version.odin with VERSION"
echo "  • Add version.odin to the commit"
echo ""
echo -e "${GREEN}Installation complete!${NC}"
