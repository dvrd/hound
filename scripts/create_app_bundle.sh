#!/bin/bash
# Script to create macOS .app bundle from Odin binary
# Usage: ./scripts/create_app_bundle.sh

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
echo -e "${GREEN}✓${NC} Building version: $VERSION"

# Set variables
APP_NAME="Hound"
BUNDLE_ID="com.hound.app"
APP_DIR="Hound.app"
CONTENTS_DIR="$APP_DIR/Contents"
MACOS_DIR="$CONTENTS_DIR/MacOS"
RESOURCES_DIR="$CONTENTS_DIR/Resources"

# Step 1: Build Odin binary
echo -e "${YELLOW}⚙${NC} Building Odin binary..."
mkdir -p bin
odin build src/menubar_main -out:bin/hound-menubar -o:speed -extra-linker-flags:"-framework AppKit -framework Foundation -lsqlite3"

if [ ! -f "bin/hound-menubar" ]; then
    echo -e "${RED}✗${NC} Build failed - binary not created"
    exit 1
fi
echo -e "${GREEN}✓${NC} Binary built successfully"

# Step 2: Create bundle directories
echo -e "${YELLOW}⚙${NC} Creating bundle structure..."
mkdir -p "$MACOS_DIR"
mkdir -p "$RESOURCES_DIR"
echo -e "${GREEN}✓${NC} Bundle directories created"

# Step 3: Copy binary
echo -e "${YELLOW}⚙${NC} Copying binary to bundle..."
cp bin/hound-menubar "$MACOS_DIR/$APP_NAME"
echo -e "${GREEN}✓${NC} Binary copied"

# Step 4: Set executable permissions
chmod +x "$MACOS_DIR/$APP_NAME"
echo -e "${GREEN}✓${NC} Executable permissions set"

# Step 5: Generate Info.plist
echo -e "${YELLOW}⚙${NC} Generating Info.plist..."
if [ ! -f "resources/Info.plist.template" ]; then
    echo -e "${RED}✗${NC} Info.plist.template not found"
    exit 1
fi

sed "s/\${VERSION}/$VERSION/g" resources/Info.plist.template > "$CONTENTS_DIR/Info.plist"
echo -e "${GREEN}✓${NC} Info.plist generated"

# Step 6: Copy icon
echo -e "${YELLOW}⚙${NC} Copying app icon..."
if [ ! -f "resources/AppIcon.icns" ]; then
    echo -e "${YELLOW}⚠${NC} AppIcon.icns not found, skipping"
else
    cp resources/AppIcon.icns "$RESOURCES_DIR/"
    echo -e "${GREEN}✓${NC} Icon copied"
fi

# Step 7: Validate plist
echo -e "${YELLOW}⚙${NC} Validating Info.plist..."
if ! plutil -lint "$CONTENTS_DIR/Info.plist" > /dev/null 2>&1; then
    echo -e "${RED}✗${NC} Info.plist validation failed"
    plutil -lint "$CONTENTS_DIR/Info.plist"
    exit 1
fi
echo -e "${GREEN}✓${NC} Info.plist is valid"

# Step 8: Optional self-sign (ignore errors)
echo -e "${YELLOW}⚙${NC} Self-signing bundle (optional)..."
codesign -s - "$APP_DIR" 2>/dev/null && echo -e "${GREEN}✓${NC} Bundle signed" || echo -e "${YELLOW}⚠${NC} Signing skipped (no developer tools)"

# Success message
BUNDLE_SIZE=$(du -sh "$APP_DIR" | cut -f1)
echo ""
echo -e "${GREEN}✅ Bundle created successfully!${NC}"
echo -e "   Location: $APP_DIR"
echo -e "   Size: $BUNDLE_SIZE"
echo -e "   Version: $VERSION"
echo ""
echo -e "To test: ${YELLOW}open $APP_DIR${NC}"
