# CLI Feature Bundle

## Overview

Five CLI features added to bring craft to package-manager parity: dependency listing (`craft list`), dependency tree visualization (`craft tree`), update checking (`craft outdated`), verbose diagnostic output (`--verbose`), and safe operation preview (`--dry-run`). All features are additive — no existing behavior was changed.

These features close the ergonomic gap between craft and established package managers like npm, cargo, and go modules. Users can now inspect dependency state, assess update risk, debug resolution issues, and preview operations without reading raw YAML or risking unintended changes.

## Architecture and Design

### High-Level Architecture

All five features integrate into the existing Cobra CLI layer (`internal/cli/`) and reuse the established data and fetch packages:

```
┌─────────────────────────────────────────────────┐
│ New Commands          │ Modified Commands        │
│ list.go               │ install.go (--dry-run)   │
│ tree.go               │ update.go  (--dry-run)   │
│ outdated.go           │                          │
├───────────────────────┴──────────────────────────┤
│ Shared Infrastructure                            │
│ helpers.go  (requireManifestAndPinfile,           │
│              printDryRunSummary)                  │
│ verbose.go  (verboseLog, sanitize)               │
│ root.go     (--verbose flag, silentExitError,    │
│              ExitCode)                           │
├──────────────────────────────────────────────────┤
│ Existing Packages (unchanged)                    │
│ manifest  pinfile  ui  fetch  semver  resolve    │
└──────────────────────────────────────────────────┘
```

**Data flow pattern**: Read-only commands (`list`, `tree`, `outdated`) call `requireManifestAndPinfile()` to load both files, then join/transform the data for display. `outdated` additionally uses `newFetcher()` for remote tag listing.

### Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Data source for `list`/`tree` | Pinfile (`craft.pin.yaml`) | Like `go list -m all`. Avoids requiring target detection. Pinfile is source of truth after install. |
| Data source for `outdated` | Manifest (`craft.yaml`) direct deps | Only the user's declared dependencies are checked. Transitive deps are managed by their parent packages. |
| Verbosity model | `--verbose` only (2 levels) | Go-style simplicity. Non-TTY already suppresses progress output, so `--quiet` isn't needed. |
| `outdated` exit code | 1 when updates available | CI scripts can use `craft outdated || echo "updates"`. Standard pattern (npm, pip). |
| `--dry-run` scope | `install` and `update` only | These are the commands with filesystem side effects. `add`/`remove` modify `craft.yaml` which is a different risk profile. |
| `silentExitError` pattern | Type-checked error with exit code | Allows `outdated` to signal exit code 1 without printing an error message. `ExitCode()` helper lets `main.go` extract the intended code. |
| Terminal sanitization | `sanitize()` strips control chars | Dependency aliases, URLs, and skill names come from YAML files that could contain ANSI escape sequences. All user-controlled strings are sanitized before terminal output. |
| `tree` output destination | stdout (not stderr) | `craft tree` is a standalone command whose primary purpose IS the tree. Install's tree goes to stderr because it's decorative feedback alongside the real operation. |

### Integration Points

- **`requireManifestAndPinfile()`** (`helpers.go`): Shared loader for list/tree/outdated. Returns hard error on missing files — distinct from install/update which tolerate absent pinfiles.
- **`printDryRunSummary()`** (`helpers.go`): Shared formatter for install/update dry-run output.
- **`sanitize()`** (`verbose.go`): Terminal escape stripping, used by list, outdated, and dry-run summary.
- **`ExitCode()`** (`root.go`): Extracts exit code from `silentExitError` for `main.go`.
- **`newFetcher()`** (`install.go`): Reused by `outdated` for remote tag listing. Pre-existing shared helper.
- **`ui.RenderTree()`** (`internal/ui/tree.go`): Reused by `craft tree` — the rendering code already existed, this PR just wires it to a standalone command.

## User Guide

### Prerequisites

- A craft project with `craft.yaml` (created by `craft init`)
- For `list`, `tree`, `outdated`: a `craft.pin.yaml` must exist (created by `craft install`)
- For `outdated`: network access to dependency remotes

### Basic Usage

**List resolved dependencies:**
```bash
$ craft list
company-standards  v2.1.0  (2 skills)
utility-skills     v1.0.0  (1 skill)

# Extended info with URLs and skill names
$ craft list --detailed
company-standards  v2.1.0  github.com/org/standards
  skills: api-conventions, error-formats
```

**View dependency tree:**
```bash
$ craft tree
my-package@1.0.0
├── Local skills
│   ├── skill-a
│   └── skill-b
└── company-standards (github.com/org/standards@v2.1.0)
    ├── api-conventions
    └── error-formats
```

**Check for updates:**
```bash
$ craft outdated
company-standards  v2.1.0 → v2.2.0  (minor)
utility-skills     v1.0.0            (up to date)

$ echo $?
1  # non-zero when updates available
```

**Preview operations:**
```bash
$ craft install --dry-run
Would resolve 2 dependency(ies):
  + company-standards  v2.1.0  (2 skills: api-conventions, error-formats)
  + utility-skills     v1.0.0  (1 skill: git-helper)

No changes made.
```

**Debug with verbose output:**
```bash
$ craft outdated --verbose
No pinfile entry for new-dep, using manifest version v1.0.0
Fetching tags from https://github.com/org/repo.git...
Found 5 tags for company-standards
Fetching tags from https://github.com/org/utils.git...
Found 3 tags for utility-skills
```

### Advanced Usage

**CI integration with `outdated`:**
```bash
# Fail CI if dependencies are outdated
craft outdated || { echo "Dependencies need updating"; exit 1; }
```

**Pipe-friendly output:**
```bash
# craft tree output goes to stdout (pipe-friendly)
craft tree | grep "api-conventions"

# craft list output goes to stdout
craft list | wc -l  # count dependencies
```

**Combining flags:**
```bash
# Verbose dry-run shows resolution steps without making changes
craft install --dry-run --verbose
```

## API Reference

### Key Components

**`requireManifestAndPinfile() (*manifest.Manifest, *pinfile.Pinfile, error)`**
Loads both `craft.yaml` and `craft.pin.yaml` from the working directory. Returns user-friendly errors when either file is missing. Used by list, tree, and outdated commands.

**`sanitize(s string) string`**
Strips control characters (except tab and newline) from strings. Use on any user-controlled data before terminal output to prevent escape injection.

**`verboseLog(cmd *cobra.Command, format string, args ...any)`**
Writes formatted message to stderr when `--verbose` is enabled. No-op otherwise.

**`ExitCode(err error) int`**
Returns the exit code from a `silentExitError`, or 1 for any other error. Used by `main.go` to propagate intended exit codes.

**`classifyUpdate(current, latest string) string`**
Returns `"major"`, `"minor"`, or `"patch"` based on which semver component changed between two version strings.

### Configuration Options

| Environment Variable | Effect on New Commands |
|---------------------|----------------------|
| `CRAFT_TOKEN` | Used by `outdated` when fetching remote tags from trusted hosts |
| `GITHUB_TOKEN` | Fallback for `outdated` on github.com when `CRAFT_TOKEN` not set |
| `CRAFT_TOKEN_HOSTS` | Restricts which hosts receive `CRAFT_TOKEN` during `outdated` checks |

| Flag | Commands | Effect |
|------|----------|--------|
| `--verbose`, `-v` | All | Enables diagnostic output to stderr |
| `--dry-run` | `install`, `update` | Prevents file writes, shows preview |
| `--detailed` | `list` | Shows URLs and individual skill names |

## Testing

### How to Test

**Quick smoke test:**
```bash
cd your-craft-project
craft list                    # Should show resolved deps
craft tree                    # Should show ASCII tree
craft outdated                # Should check for updates (needs network)
craft install --dry-run       # Should show preview, no files changed
craft version --verbose       # Should accept flag without error
```

**Verify dry-run safety:**
```bash
rm -f craft.pin.yaml
craft install --dry-run       # Should error: no deps to install (if no deps)
                              # OR show preview without creating craft.pin.yaml
ls craft.pin.yaml             # Should not exist
```

**Verify exit codes:**
```bash
craft outdated; echo "exit: $?"  # 1 if updates available, 0 if current
```

### Edge Cases

| Scenario | Behavior |
|----------|----------|
| No `craft.pin.yaml` | list/tree/outdated error with "run `craft install` first" |
| No `craft.yaml` | All commands error with "run `craft init` first" |
| Zero dependencies | list: "No dependencies resolved." / outdated: "No dependencies to check." |
| Dependency with no semver tags | outdated: skipped with warning, not shown as "(up to date)" |
| Pinned version tag absent from remote | outdated: warning printed, still shows current + latest |
| Network failure for one dependency | outdated: error for that dep, continues checking others, exit 1 |
| Single skill | list: "(1 skill)" singular, not "(1 skills)" |
| Zero skills | list --detailed: "(none)" / dry-run: "(0 skills)" |
| ANSI escapes in dependency names | Stripped by `sanitize()` before display |

## Limitations and Future Work

**Current limitations:**
- `craft outdated` fetches tags sequentially — could be slow with many dependencies. Parallel fetching is a future optimization.
- No `--quiet` flag — non-TTY already suppresses progress, but explicit quiet mode could be added later.
- No JSON/machine-readable output format — all output is human-readable text.
- `--dry-run` is not available on `add` or `remove` (these only modify `craft.yaml`, a different risk profile).
- `CRAFT_TOKEN` default-allow policy for unknown hosts is a pre-existing security concern (see `internal/fetch/auth.go`). Setting `CRAFT_TOKEN_HOSTS` is recommended in shared/CI environments.

**Possible future work:**
- `--format json` for machine-readable output
- `--quiet` flag for suppressing all non-error output
- Parallel tag fetching in `craft outdated`
- `craft search` for discovering available skill packages
- Default-deny for `CRAFT_TOKEN` on unknown hosts (breaking change, needs version bump)
