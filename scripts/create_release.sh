#!/bin/bash
# Automated release creation script for Hound
# Usage: ./scripts/create_release.sh <version> [--push]
#
# Example: ./scripts/create_release.sh 0.7.0 --push

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Parse arguments
VERSION="$1"
PUSH_FLAG="$2"

if [ -z "$VERSION" ]; then
    echo -e "${RED}Error: Version number required${NC}"
    echo "Usage: $0 <version> [--push]"
    echo "Example: $0 0.7.0 --push"
    exit 1
fi

# Validate semver format
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo -e "${RED}Error: Version must be in semver format (e.g., 0.7.0)${NC}"
    exit 1
fi

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Hound Release Automation v$VERSION${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

cd "$PROJECT_ROOT"

# Step 1: Update version files
echo -e "${YELLOW}[1/7]${NC} Updating version files..."
./scripts/update_version.sh "$VERSION"
echo -e "${GREEN}✓${NC} Version files updated"
echo ""

# Step 2: Build menubar app
echo -e "${YELLOW}[2/7]${NC} Building menubar app..."
task menubar:build > /dev/null 2>&1
echo -e "${GREEN}✓${NC} Binary built"
echo ""

# Step 3: Create app bundle
echo -e "${YELLOW}[3/7]${NC} Creating app bundle..."
task menubar:bundle > /dev/null 2>&1
echo -e "${GREEN}✓${NC} Bundle created: Hound.app"
echo ""

# Step 4: Create DMG
echo -e "${YELLOW}[4/7]${NC} Creating DMG installer..."
task menubar:dmg > /dev/null 2>&1
DMG_FILE="Hound-${VERSION}.dmg"
echo -e "${GREEN}✓${NC} DMG created: $DMG_FILE"
echo ""

# Step 5: Generate checksum
echo -e "${YELLOW}[5/7]${NC} Generating SHA256 checksum..."
shasum -a 256 "$DMG_FILE" > "${DMG_FILE}.sha256"
CHECKSUM=$(cat "${DMG_FILE}.sha256" | cut -d' ' -f1)
echo -e "${GREEN}✓${NC} Checksum: $CHECKSUM"
echo ""

# Step 6: Create git commits and tag
echo -e "${YELLOW}[6/7]${NC} Creating git commits and tag..."

# Commit version bump
git add VERSION src/version.odin
git commit -m "chore: bump version to $VERSION

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>" > /dev/null 2>&1

# Create annotated tag with release notes
git tag -a "v${VERSION}" -m "Release v${VERSION}

See RELEASE_NOTES_v${VERSION}.md for detailed changelog.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

echo -e "${GREEN}✓${NC} Git tag created: v${VERSION}"
echo ""

# Step 7: Summary
echo -e "${YELLOW}[7/7]${NC} Release summary"
echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Release v${VERSION} Ready!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "📦 ${BLUE}Files created:${NC}"
echo -e "   - $DMG_FILE ($(ls -lh "$DMG_FILE" | awk '{print $5}'))"
echo -e "   - ${DMG_FILE}.sha256"
echo ""
echo -e "🏷️  ${BLUE}Git tag:${NC} v${VERSION}"
echo ""
echo -e "🔐 ${BLUE}SHA256:${NC}"
echo -e "   $CHECKSUM"
echo ""

# Optional: Push to remote
if [ "$PUSH_FLAG" = "--push" ]; then
    echo -e "${YELLOW}Pushing to remote...${NC}"
    git push origin master
    git push origin "v${VERSION}"
    echo -e "${GREEN}✓${NC} Pushed commits and tag to origin"
    echo ""
    echo -e "${GREEN}🎉 Release v${VERSION} published!${NC}"
else
    echo -e "${YELLOW}⚠  Commits and tag created locally${NC}"
    echo ""
    echo -e "To push to remote repository, run:"
    echo -e "  ${BLUE}git push origin master${NC}"
    echo -e "  ${BLUE}git push origin v${VERSION}${NC}"
    echo ""
    echo -e "Or run this script with ${BLUE}--push${NC} flag:"
    echo -e "  ${BLUE}./scripts/create_release.sh $VERSION --push${NC}"
fi

echo ""
echo -e "${GREEN}Next steps:${NC}"
echo -e "  1. Create release notes in ${BLUE}RELEASE_NOTES_v${VERSION}.md${NC}"
echo -e "  2. Create GitHub release with tag ${BLUE}v${VERSION}${NC}"
echo -e "  3. Upload ${BLUE}$DMG_FILE${NC} to GitHub release"
echo -e "  4. Add SHA256 checksum to release notes"
echo ""
