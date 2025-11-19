#!/bin/bash
# Script to generate macOS icon from an image file
# Usage: ./scripts/update_icon_from_image.sh <image_path>

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <image_path>"
    exit 1
fi

INPUT_IMAGE="$1"

if [ ! -f "$INPUT_IMAGE" ]; then
    echo "Error: Image file not found: $INPUT_IMAGE"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ICONSET_DIR="$PROJECT_ROOT/AppIcon.iconset"
RESOURCES_DIR="$PROJECT_ROOT/resources"

# Create directories
mkdir -p "$ICONSET_DIR"
mkdir -p "$RESOURCES_DIR"

echo "🎨 Generating Hound icon from: $INPUT_IMAGE"

# Generate all required sizes for macOS iconset
# Format: size filename
SIZES=(
    "16 icon_16x16.png"
    "32 icon_16x16@2x.png"
    "32 icon_32x32.png"
    "64 icon_32x32@2x.png"
    "128 icon_128x128.png"
    "256 icon_128x128@2x.png"
    "256 icon_256x256.png"
    "512 icon_256x256@2x.png"
    "512 icon_512x512.png"
    "1024 icon_512x512@2x.png"
)

for size_info in "${SIZES[@]}"; do
    SIZE=$(echo $size_info | awk '{print $1}')
    FILENAME=$(echo $size_info | awk '{print $2}')
    OUTPUT_PATH="$ICONSET_DIR/$FILENAME"

    # Use sips with explicit PNG format
    sips -z $SIZE $SIZE "$INPUT_IMAGE" --out "$OUTPUT_PATH" -s format png > /dev/null 2>&1
    echo "  ✓ Created $FILENAME (${SIZE}x${SIZE}px)"
done

# Convert iconset to icns
ICNS_PATH="$RESOURCES_DIR/AppIcon.icns"
iconutil -c icns "$ICONSET_DIR" -o "$ICNS_PATH"
echo ""
echo "✅ Icon created: $ICNS_PATH"

# Copy to app bundle if it exists
if [ -d "$PROJECT_ROOT/Hound.app/Contents/Resources" ]; then
    cp "$ICNS_PATH" "$PROJECT_ROOT/Hound.app/Contents/Resources/AppIcon.icns"
    echo "   Copied to: Hound.app/Contents/Resources/AppIcon.icns"
fi

# Create preview PNG
PREVIEW_PATH="$RESOURCES_DIR/icon_preview.png"
sips -z 512 512 "$INPUT_IMAGE" --out "$PREVIEW_PATH" -s format png > /dev/null 2>&1
echo "   Preview: $PREVIEW_PATH"

# Clean up iconset directory
rm -rf "$ICONSET_DIR"
echo ""
echo "🐕 Icon updated successfully!"
