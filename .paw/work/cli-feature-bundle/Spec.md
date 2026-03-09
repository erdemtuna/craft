# Feature Specification: CLI Feature Bundle

**Branch**: feature/cli-feature-bundle  |  **Created**: 2026-03-09  |  **Status**: Draft
**Input Brief**: Add five missing CLI features — craft list, craft tree, craft outdated, --verbose flag, and --dry-run on install/update — to bring craft to package-manager parity.

## Overview

Craft currently lacks basic introspection and safety commands that developers expect from any dependency manager. Users cannot see what dependencies are resolved without manually reading `craft.pin.yaml`, cannot preview available updates before committing to them, and cannot visualize the dependency tree outside of an install operation. These gaps force users into manual file inspection and blind update workflows.

This feature bundle adds five capabilities that close the ergonomic gap. Three new commands (`craft list`, `craft tree`, `craft outdated`) provide read-only visibility into the dependency state. A global `--verbose` flag gives developers diagnostic output for debugging resolution and authentication issues. A `--dry-run` flag on `install` and `update` lets users preview what would change before any files are modified.

Together, these features make craft a more transparent, predictable, and CI-friendly tool. Users gain confidence in their dependency state, can make informed update decisions, and can safely validate operations in production environments before committing to changes.

## Objectives

- Enable developers to inspect their resolved dependency state without reading raw YAML files
- Allow developers to discover available updates and assess their risk (major/minor/patch) before applying them
- Provide a standalone dependency tree visualization for documentation and debugging
- Give developers diagnostic output to troubleshoot resolution, fetching, and authentication issues
- Let developers safely preview install and update operations without modifying any files

## User Scenarios & Testing

### User Story P1 – List Resolved Dependencies

Narrative: A developer joins a project using craft and wants to quickly understand what dependencies are resolved and how many skills each provides, without opening configuration files.

Independent Test: Run `craft list` in a project with resolved dependencies and see a summary table.

Acceptance Scenarios:
1. Given a project with `craft.pin.yaml` containing 3 resolved dependencies, When the user runs `craft list`, Then a table is printed showing each dependency's alias, version, and skill count, sorted alphabetically by alias.
2. Given a project with `craft.pin.yaml`, When the user runs `craft list --detailed`, Then each dependency's alias, version, source URL, and individual skill names are shown.
3. Given a project with no `craft.pin.yaml`, When the user runs `craft list`, Then an error message is printed directing the user to run `craft install` first, and the exit code is non-zero.
4. Given a project with `craft.pin.yaml` containing zero resolved dependencies, When the user runs `craft list`, Then the message "No dependencies resolved." is printed and the exit code is 0.

### User Story P1 – Preview Available Updates

Narrative: A developer wants to check if any dependencies have newer versions available before deciding whether to run `craft update`. They need to see the update type (major/minor/patch) to assess risk.

Independent Test: Run `craft outdated` and see which dependencies have updates available, with version comparison and risk classification.

Acceptance Scenarios:
1. Given a project with 2 resolved dependencies where one has a newer version available, When the user runs `craft outdated`, Then the outdated dependency shows current and latest versions with update type label, the up-to-date dependency shows "(up to date)", and the exit code is 1.
2. Given a project where all dependencies are at their latest versions, When the user runs `craft outdated`, Then all dependencies show "(up to date)" and the exit code is 0.
3. Given a project with no `craft.pin.yaml`, When the user runs `craft outdated`, Then an error message is printed directing the user to run `craft install` first.
4. Given a dependency whose remote repository is unreachable, When the user runs `craft outdated`, Then an error is printed for that specific dependency, remaining dependencies are still checked, and the exit code is 1.
5. Given a project with zero resolved dependencies, When the user runs `craft outdated`, Then the message "No dependencies to check." is printed and the exit code is 0.

### User Story P2 – Visualize Dependency Tree

Narrative: A developer wants to see the full dependency hierarchy — local skills and remote dependencies with their skills — as a tree diagram, without triggering an install.

Independent Test: Run `craft tree` and see an ASCII tree showing the package, its local skills, and all resolved dependencies with their skills.

Acceptance Scenarios:
1. Given a project with local skills and resolved dependencies, When the user runs `craft tree`, Then an ASCII box-drawing tree is printed showing the package name, local skills, and each dependency with its skills.
2. Given a project with no dependencies but local skills, When the user runs `craft tree`, Then the tree shows only the package name and local skills.
3. Given a project with no `craft.pin.yaml`, When the user runs `craft tree`, Then an error message is printed directing the user to run `craft install` first.

### User Story P2 – Verbose Diagnostic Output

Narrative: A developer is debugging why `craft install` fails to resolve a dependency. They need to see detailed output — which repositories are being fetched, what versions are being compared, and whether authentication is succeeding.

Independent Test: Run `craft install --verbose` and see detailed step-by-step output for each fetch, resolution, and installation operation.

Acceptance Scenarios:
1. Given a project with dependencies, When the user runs `craft install --verbose`, Then additional diagnostic output is printed showing fetch operations, version comparisons, cache interactions, and integrity checks.
2. Given any craft command run without `--verbose`, When the command executes, Then output matches current behavior with no additional diagnostic messages.
3. Given the `--verbose` flag (or `-v` shorthand), When used with any craft command, Then the flag is accepted without error.

### User Story P2 – Dry-Run Preview for Install and Update

Narrative: A developer working in a production environment wants to verify what `craft install` or `craft update` would do before allowing any files to be written. They need to see the full resolution result without any side effects.

Independent Test: Run `craft install --dry-run` and see what would be resolved and installed, then verify no files were created or modified.

Acceptance Scenarios:
1. Given a project with dependencies, When the user runs `craft install --dry-run`, Then the resolution runs fully, a summary of what would be resolved and installed is printed, and neither `craft.pin.yaml` nor the target directory are modified.
2. Given a project with dependencies, When the user runs `craft update --dry-run`, Then the resolution runs fully, a summary of what would change is printed, and neither `craft.pin.yaml` nor `craft.yaml` nor the target directory are modified.
3. Given a project where resolution would fail (e.g., unresolvable dependency), When the user runs `craft install --dry-run`, Then the same error is shown as a normal install would produce.

### Edge Cases

- `craft list` and `craft tree` with a corrupted or unparseable `craft.pin.yaml`: standard YAML parse error with clear message.
- `craft outdated` with a dependency that has no semver-compatible tags: skip with a warning message, continue checking other dependencies.
- `craft outdated` with a dependency whose pinned version tag no longer exists on remote: show current version with a warning that the tag is missing from remote.
- `--verbose` combined with `--dry-run`: both flags active; verbose shows resolution steps, dry-run prevents writes.
- `craft list --detailed` with a dependency that has zero skills: show the dependency with "0 skills" or empty skill list.

## Requirements

### Functional Requirements

- FR-001: `craft list` prints a table of resolved dependencies showing alias, version, and skill count, sorted alphabetically by alias. (Stories: P1-List)
- FR-002: `craft list --detailed` prints extended information including source URL and individual skill names for each dependency. (Stories: P1-List)
- FR-003: `craft outdated` compares each direct dependency's pinned version against the latest available semver tag from the remote repository. (Stories: P1-Outdated)
- FR-004: `craft outdated` classifies each available update as major, minor, or patch based on which semver component changed. (Stories: P1-Outdated)
- FR-005: `craft outdated` exits with code 1 when any dependency has an available update, and code 0 when all are up to date. (Stories: P1-Outdated)
- FR-006: `craft tree` prints an ASCII dependency tree showing the package name, local skills, and all resolved dependencies with their skills. (Stories: P2-Tree)
- FR-007: A global `--verbose` (`-v`) flag is available on all commands and, when set, causes additional diagnostic output to be printed. (Stories: P2-Verbose)
- FR-008: `craft install --dry-run` runs full dependency resolution but does not write `craft.pin.yaml` or install files to the target directory. (Stories: P2-DryRun)
- FR-009: `craft update --dry-run` runs full dependency resolution but does not modify `craft.yaml`, `craft.pin.yaml`, or install files to the target directory. (Stories: P2-DryRun)
- FR-010: `craft list`, `craft tree`, and `craft outdated` produce a clear error message when no `craft.pin.yaml` exists, directing the user to run `craft install`. (Stories: P1-List, P1-Outdated, P2-Tree)
- FR-011: `craft outdated` handles per-dependency fetch failures gracefully — printing an error for the failing dependency while continuing to check remaining dependencies. (Stories: P1-Outdated)
- FR-012: `--dry-run` on install and update prints a summary of what would be resolved/changed. (Stories: P2-DryRun)

### Cross-Cutting / Non-Functional

- All new commands must follow existing cobra command registration patterns.
- `--verbose` output goes to stderr so it does not interfere with parseable stdout output.
- Read-only commands (`list`, `tree`, `outdated`) must not modify any files on disk.
- All new commands and flags must have unit tests.

## Success Criteria

- SC-001: A user can determine the alias, version, and skill count of every resolved dependency by running a single command. (FR-001)
- SC-002: A user can see the source URL and individual skill names for each resolved dependency using a flag. (FR-002)
- SC-003: A user can identify which dependencies have newer versions available and the risk level of each update without applying any changes. (FR-003, FR-004)
- SC-004: A CI script can detect whether any dependencies are outdated via exit code. (FR-005)
- SC-005: A user can view the full dependency hierarchy as a tree without triggering installation. (FR-006)
- SC-006: A user can enable verbose diagnostic output on any command to debug resolution or authentication issues. (FR-007)
- SC-007: A user can preview the full result of an install or update operation with confidence that no files are modified. (FR-008, FR-009, FR-012)
- SC-008: All three new commands produce a helpful error when the pinfile is missing. (FR-010)
- SC-009: A single dependency failure during `craft outdated` does not prevent checking remaining dependencies. (FR-011)

## Assumptions

- The pinfile (`craft.pin.yaml`) is the authoritative source for "resolved" dependency state. There is no separate "installed on disk" verification.
- Only direct dependencies (declared in `craft.yaml`) are checked by `craft outdated`. Transitive dependencies are managed by their parent packages.
- Cache warming during `--dry-run` (fetching repository data to resolve) is acceptable since the cache is a side-effect-free optimization layer.
- Dependencies without any semver-compatible tags are skipped by `craft outdated` with a warning rather than treated as errors.
- The `--verbose` flag uses a 2-level model (normal/verbose). There is no `--quiet` flag — non-TTY environments already suppress progress output.

## Scope

In Scope:
- `craft list` command with `--detailed` flag
- `craft tree` command (standalone, read-only)
- `craft outdated` command with semver classification and CI-friendly exit codes
- `--verbose` / `-v` global persistent flag on root command
- `--dry-run` flag on `install` and `update` commands
- Unit tests for all new commands and flags
- Error handling for missing pinfile across all new commands

Out of Scope:
- `--quiet` flag (non-TTY already suppresses progress)
- `--dry-run` on `add` or `remove` commands
- JSON or machine-readable output format
- `craft search` or registry browsing
- Colored output or ANSI formatting
- `--format` flag for custom output templates
- Verification of on-disk installation state

## Dependencies

- Existing `internal/pinfile` package for reading resolved dependency state
- Existing `internal/manifest` package for reading declared dependencies and package info
- Existing `internal/ui` package (tree rendering, progress output)
- Existing `internal/fetch` and `internal/semver` packages for remote version checking
- Cobra CLI framework for command and flag registration

## Risks & Mitigations

- **`--dry-run` control flow complexity**: Inserting an early-return into install/update could skip necessary setup or leave inconsistent state. Mitigation: Intercept at a clean boundary (after resolution, before writes) and test that resolution still works identically with and without the flag.
- **`craft outdated` network latency**: Checking each dependency sequentially could be slow for projects with many dependencies. Mitigation: Accept sequential for v1; note parallel fetching as a future optimization.
- **`--verbose` output noise**: Too much diagnostic output reduces its usefulness. Mitigation: Limit verbose output to key operations (fetch, resolve, verify) rather than every internal step.

## References

- WorkShaping: `.paw/work/cli-feature-bundle/WorkShaping.md` (session artifact)
- Codebase audit: `codeFindings.md` section 6 (feature source)
