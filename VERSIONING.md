# Version Control System

Hound uses [Semantic Versioning 2.0.0](https://semver.org/) for release management.

## Version Format

```
MAJOR.MINOR.PATCH (e.g., 0.4.2)
```

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

## Current Version

The current version is stored in the `VERSION` file at the project root.

```bash
# View current version
cat VERSION

# Or use the CLI
./bin/hound --version
```

## Updating Version

### Option 1: Using Task (Recommended)

```bash
# Update to new version
task version:update -- 0.5.0

# Update and create git tag
task version:tag -- 0.5.0

# Sync version.odin with VERSION file
task version:sync
```

### Option 2: Manual Update

1. Edit `VERSION` file with new version number
2. Run sync script to update `src/version.odin`:
   ```bash
   ./scripts/update_version.sh
   ```

## Version in Code

Version information is available in `src/version.odin`:

```odin
VERSION :: "0.4.2"
VERSION_MAJOR :: 0
VERSION_MINOR :: 4
VERSION_PATCH :: 2

get_version() -> string           // Returns "0.4.2"
get_version_info() -> string      // Returns "hound v0.4.2"
```

## CLI Usage

```bash
# Show version
./bin/hound --version
./bin/hound -v
./bin/hound version

# Output: hound v0.4.2
```

## Release Workflow

1. **Update VERSION file**:
   ```bash
   task version:update -- 1.0.0
   ```

2. **Commit changes**:
   ```bash
   git add VERSION src/version.odin
   git commit -m "chore: bump version to 1.0.0"
   ```

3. **Create git tag**:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   ```

4. **Push changes and tag**:
   ```bash
   git push origin master
   git push origin v1.0.0
   ```

## Automated Version Bumping

The `scripts/update_version.sh` script handles:
- ✅ Validates semver format
- ✅ Updates `VERSION` file
- ✅ Generates `src/version.odin` with parsed components
- ✅ Optionally creates git tags

## Development Versions

Pre-1.0 versions (0.x.y) indicate active development:
- Breaking changes may occur in MINOR updates
- Current phase: **Phase 4.2** (SOL oracle with caching)

## Version History

Check git tags for release history:
```bash
git tag -l "v*"
```

Or view commit history:
```bash
git log --oneline --grep="version"
```
