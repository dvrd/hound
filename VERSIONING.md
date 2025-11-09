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

## Automatic Synchronization (Pre-commit Hook)

Hound includes a **pre-commit git hook** that automatically syncs `src/version.odin` when `VERSION` changes.

### Installation

Install the hook once after cloning:

```bash
# Using Task (recommended)
task hooks:install

# Or directly
./scripts/install-hooks.sh
```

### How It Works

When you commit a change to `VERSION`:

```bash
echo "0.5.0" > VERSION
git add VERSION
git commit -m "chore: bump version to 0.5.0"
```

The hook automatically:
1. ✅ Detects `VERSION` in the commit
2. ✅ Runs `./scripts/update_version.sh`
3. ✅ Adds `src/version.odin` to the commit
4. ✅ Continues with the commit

**No manual sync needed!** Both files stay synchronized automatically.

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

With the pre-commit hook installed, releasing is simplified:

1. **Update VERSION file**:
   ```bash
   echo "1.0.0" > VERSION
   # Or: task version:update -- 1.0.0
   ```

2. **Commit changes** (hook auto-adds version.odin):
   ```bash
   git add VERSION
   git commit -m "chore: bump version to 1.0.0"
   # Hook automatically syncs and includes src/version.odin
   ```

3. **Create git tag**:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   # Or: task version:tag -- 1.0.0 (does both step 1 & 3)
   ```

4. **Push changes and tag**:
   ```bash
   git push origin master
   git push origin v1.0.0
   ```

### Without Hook Installed

If the pre-commit hook is not installed, manually sync:

```bash
echo "1.0.0" > VERSION
./scripts/update_version.sh
git add VERSION src/version.odin
git commit -m "chore: bump version to 1.0.0"
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
