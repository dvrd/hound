#!/bin/bash
# Script to create DMG installer from .app bundle
# Usage: ./scripts/create_dmg.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Read VERSION file
if [ ! -f "VERSION" ]; then
    echo -e "${RED}✗${NC} VERSION file not found"
    exit 1
fi

VERSION=$(cat VERSION | tr -d '[:space:]')
echo -e "${GREEN}✓${NC} Creating DMG for version: $VERSION"

# Set variables
DMG_NAME="Hound-${VERSION}.dmg"
VOLUME_NAME="Hound ${VERSION}"
APP_DIR="Hound.app"

# Check app bundle exists
if [ ! -d "$APP_DIR" ]; then
    echo -e "${RED}✗${NC} Hound.app not found. Run 'task menubar:bundle' first."
    exit 1
fi
echo -e "${GREEN}✓${NC} App bundle found"

# Remove old DMG if exists
if [ -f "$DMG_NAME" ]; then
    echo -e "${YELLOW}⚙${NC} Removing old DMG..."
    rm -f "$DMG_NAME"
fi

# Create temporary directory
echo -e "${YELLOW}⚙${NC} Creating temporary staging directory..."
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Copy app to temp directory
echo -e "${YELLOW}⚙${NC} Copying app bundle..."
cp -R "$APP_DIR" "$TMP_DIR/"
echo -e "${GREEN}✓${NC} App bundle copied"

# Create Applications symlink
echo -e "${YELLOW}⚙${NC} Creating Applications symlink..."
ln -s /Applications "$TMP_DIR/Applications"
echo -e "${GREEN}✓${NC} Symlink created"

# Create DMG
echo -e "${YELLOW}⚙${NC} Creating DMG (this may take a moment)..."
hdiutil create \
    -volname "$VOLUME_NAME" \
    -srcfolder "$TMP_DIR" \
    -ov \
    -format UDZO \
    "$DMG_NAME" > /dev/null 2>&1

if [ ! -f "$DMG_NAME" ]; then
    echo -e "${RED}✗${NC} DMG creation failed"
    exit 1
fi
echo -e "${GREEN}✓${NC} DMG created"

# Cleanup (trap will handle this)
echo -e "${GREEN}✓${NC} Cleaned up temporary files"

# Success message
DMG_SIZE=$(du -sh "$DMG_NAME" | cut -f1)
echo ""
echo -e "${GREEN}✅ DMG created successfully!${NC}"
echo -e "   Location: $DMG_NAME"
echo -e "   Size: $DMG_SIZE"
echo -e "   Volume: $VOLUME_NAME"
echo ""
echo -e "To test: ${YELLOW}open $DMG_NAME${NC}"
