# CLI Feature Bundle Implementation Plan

## Overview

Add five CLI features to bring craft to package-manager parity: `craft list` (show resolved dependencies), `craft tree` (standalone dependency tree), `craft outdated` (preview available updates), `--verbose` global flag (diagnostic output), and `--dry-run` on install/update (safe preview). All features are additive — no existing behavior changes.

## Current State Analysis

- **CLI layer** (`internal/cli/`): 8 commands registered via Cobra. No persistent/global flags on rootCmd. Commands use `RunE` pattern returning errors. Output via `cmd.Printf()` (stdout) and `fmt.Fprintf(os.Stderr)` (errors/progress).
- **Data layer**: `pinfile.ParseFile()` and `manifest.ParseFile()` provide all dependency state. Pinfile keys include version (`github.com/org/repo@v1.0.0`), extractable via `resolve.ParseDepURL()`.
- **UI layer**: `ui.RenderTree()` exists and is standalone-ready. `ui.Progress` handles TTY detection. No verbosity mechanism.
- **Fetch/semver**: `fetcher.ListTags()` + `semver.FindLatest()` + `semver.Compare()` + `semver.ParseParts()` provide everything needed for outdated checks.
- **Install/update flows**: Clean interception points exist — resolution completes before any writes. `writePinfileAtomic()` at install.go:86, `installlib.Install()` at install.go:112. Update similarly at update.go:143/148/165.
- **Testing**: Table-driven tests, `MockFetcher` for network isolation, Cobra tests via `rootCmd.SetArgs()`/`Execute()`, testdata fixtures.

## Desired End State

Five new capabilities available in craft CLI:
- `craft list` and `craft list --detailed` show resolved dependency state from pinfile
- `craft tree` shows standalone ASCII dependency tree
- `craft outdated` checks remote for updates, classifies risk, exits 1 when outdated
- `--verbose` / `-v` available on all commands for diagnostic output
- `craft install --dry-run` and `craft update --dry-run` preview operations without writes

Verification: `task ci` passes (fmt, vet, lint, vuln, test, build). All new commands have unit tests. README updated with new commands.

## What We're NOT Doing

- `--quiet` flag (non-TTY already suppresses progress)
- `--dry-run` on `add` or `remove` commands
- JSON or machine-readable output format
- `craft search` or registry browsing
- Colored output or ANSI formatting
- `--format` flag for custom output templates
- On-disk installation verification for `craft list`
- Parallel fetching for `craft outdated` (sequential is acceptable for v1)
- Checking transitive dependencies in `craft outdated`

## Phase Status
- [ ] **Phase 1: Verbose flag infrastructure** - Add global `--verbose` flag and verbosity helper
- [ ] **Phase 2: craft list command** - Show resolved dependencies from pinfile
- [ ] **Phase 3: craft tree command** - Standalone dependency tree visualization
- [ ] **Phase 4: craft outdated command** - Check for available updates with semver classification
- [ ] **Phase 5: Dry-run on install and update** - Preview operations without side effects
- [ ] **Phase 6: Documentation** - README updates and Docs.md

## Phase Candidates
<!-- No deferred candidates — all features are committed for this bundle -->

---

## Phase 1: Verbose flag infrastructure

### Objective
Add a global `--verbose` / `-v` persistent flag on rootCmd and a package-level helper that commands can query. This is the foundation phase — later phases will emit verbose output through this mechanism.

### Changes Required

- **`internal/cli/root.go`**: Add `var verbose bool` package-level variable. Register `--verbose` / `-v` as a `PersistentBoolVar` on `rootCmd` in `init()`. This makes the flag available to all subcommands automatically.

- **`internal/cli/verbose.go`** (new file): Create a small helper for verbose output. Provide a `verboseLog(cmd *cobra.Command, format string, args ...any)` function that writes to `cmd.ErrOrStderr()` only when `verbose` is true. Prefix verbose lines with a distinguishing marker (e.g., no prefix — just the message, since it goes to stderr and only appears with `--verbose`). Follow the pattern of `printDependencyTree()` which writes to `cmd.ErrOrStderr()`.

- **Tests — `internal/cli/verbose_test.go`** (new file): Test that `verboseLog` writes to stderr when verbose is true and suppresses when false. Test that `--verbose` and `-v` are accepted by rootCmd without error.

- **Tests — `internal/cli/cli_test.go`**: Add a test verifying `--verbose` flag is accepted on a command (e.g., `craft version --verbose` does not error).

### Success Criteria

#### Automated Verification:
- [ ] Tests pass: `task test`
- [ ] Lint clean: `task lint`
- [ ] Build succeeds: `task build`

#### Manual Verification:
- [ ] `./craft version --verbose` runs without error
- [ ] `./craft version -v` runs without error (shorthand)
- [ ] `./craft --verbose version` runs without error (persistent flag before subcommand)
- [ ] Default behavior unchanged — no verbose output without the flag

---

## Phase 2: craft list command

### Objective
Add `craft list` command that reads manifest + pinfile and prints a summary table of resolved dependencies. Support `--detailed` flag for extended output.

### Changes Required

- **`internal/cli/list.go`** (new file): New cobra command `listCmd` with `Use: "list"`, `Short: "List resolved dependencies"`, `RunE: runList`. Register `--detailed` bool flag. Implementation:
  1. Parse manifest via `manifest.ParseFile("craft.yaml")` — get `Dependencies` map (alias → URL) and package metadata
  2. Parse pinfile via `pinfile.ParseFile("craft.pin.yaml")` — get `Resolved` map
  3. Handle missing pinfile → error with "run 'craft install' first" suggestion
  4. Handle zero dependencies → print "No dependencies resolved." and return nil
  5. Join data: for each manifest dependency, find matching pinfile entry by URL key. Extract version via `resolve.ParseDepURL()` on the pinfile key
  6. Sort entries alphabetically by alias
  7. Default format: `alias  vX.Y.Z  (N skills)` — aligned columns using `fmt.Fprintf` with `tabwriter` or manual padding
  8. Detailed format: alias, version, source URL, then indented skill names on next lines
  9. Emit verbose output for each dependency lookup when `--verbose` is set

- **`internal/cli/root.go`**: Add `rootCmd.AddCommand(listCmd)` in `init()`.

- **Tests — `internal/cli/list_test.go`** (new file): Test scenarios from Spec.md:
  1. List with resolved dependencies shows table (default format)
  2. List with `--detailed` shows extended output
  3. List with no pinfile returns error with helpful message
  4. List with zero dependencies shows "No dependencies resolved."
  5. Use `testdata/` fixtures or inline manifest+pinfile setup via `testWriteFile()`

### Success Criteria

#### Automated Verification:
- [ ] Tests pass: `task test`
- [ ] Lint clean: `task lint`

#### Manual Verification:
- [ ] `./craft list` in a project with dependencies shows a clean table
- [ ] `./craft list --detailed` shows URLs and skill names
- [ ] `./craft list` with no `craft.pin.yaml` shows actionable error
- [ ] `./craft list --verbose` shows additional diagnostic output

---

## Phase 3: craft tree command

### Objective
Wire existing `ui.RenderTree()` to a standalone `craft tree` command. This is the smallest phase — primarily wiring code.

### Changes Required

- **`internal/cli/tree.go`** (new file): New cobra command `treeCmd` with `Use: "tree"`, `Short: "Print dependency tree"`, `RunE: runTree`. Implementation:
  1. Parse manifest — get package name, version, local skills
  2. Parse pinfile — get resolved dependencies
  3. Handle missing pinfile → same error pattern as `craft list`
  4. Build `[]ui.DepNode` from pinfile data — follow exact pattern from `printDependencyTree()` at `install.go:211-237`
  5. Extract local skill names from manifest `Skills` paths — same logic as `install.go:218-223`
  6. Call `ui.RenderTree(cmd.OutOrStdout(), packageName, localSkills, deps)` — note: write to stdout (not stderr like install does) since this is the primary output, not a side effect

- **`internal/cli/root.go`**: Add `rootCmd.AddCommand(treeCmd)` in `init()`.

- **Tests — `internal/cli/tree_test.go`** (new file): Test scenarios:
  1. Tree with dependencies shows full tree with box-drawing characters
  2. Tree with no dependencies shows package and local skills only
  3. Tree with no pinfile returns error
  4. Verify output goes to stdout (capture via `cmd.SetOut()`)

### Success Criteria

#### Automated Verification:
- [ ] Tests pass: `task test`
- [ ] Lint clean: `task lint`

#### Manual Verification:
- [ ] `./craft tree` in a project with dependencies shows ASCII tree
- [ ] Output matches the format from `craft install` tree rendering

---

## Phase 4: craft outdated command

### Objective
Add `craft outdated` command that checks each direct dependency against its remote repository for newer versions, classifies update type (major/minor/patch), and exits with code 1 when updates are available.

### Changes Required

- **`internal/cli/outdated.go`** (new file): New cobra command `outdatedCmd` with `Use: "outdated"`, `Short: "Show available dependency updates"`, `RunE: runOutdated`. Implementation:
  1. Parse manifest + pinfile (same pattern as list/tree)
  2. Handle missing pinfile, zero dependencies (same error patterns)
  3. Create fetcher via `newFetcher()` (reuse helper from `install.go:327-337` — may need to move to a shared location or keep as-is since it's package-level)
  4. For each direct dependency (from manifest `Dependencies` map):
     a. Extract current version from pinfile key via `resolve.ParseDepURL()`
     b. Compute clone URL via `DepURL.HTTPSURL()` or `fetch.NormalizeCloneURL()`
     c. Call `fetcher.ListTags(cloneURL)` — wrap in error handling per-dependency
     d. Call `semver.FindLatest(tags)` — skip if no semver tags (warn)
     e. Compare via `semver.Compare(current, latest)` — skip if up to date
     f. Classify update type by comparing `semver.ParseParts()` results: if major differs → "major", else if minor differs → "minor", else → "patch"
  5. Print results sorted by alias: outdated deps show `alias  vCurrent → vLatest  (type)`, up-to-date deps show `alias  vCurrent  (up to date)`
  6. Track whether any dep is outdated — if so, return a sentinel error or use `os.Exit(1)` pattern
  7. Verbose output: log each fetch operation, version comparison result

- **Exit code handling**: The current error flow (`RunE` returns error → `Execute()` prints to stderr → `main()` exits 1) would print an error message for exit code 1. For `craft outdated`, exit 1 should be silent (just the exit code). Options:
  - Define a sentinel error type (e.g., `type exitError struct{ code int }`) that `Execute()` recognizes and doesn't print
  - Or have `runOutdated` call `os.Exit(1)` directly (simpler but less testable)
  - **Recommended**: Sentinel error approach — define `type silentExitError struct{ code int }` in a new file or in `root.go`, check for it in `Execute()` before printing. This keeps the error flow clean and testable.

- **`internal/cli/root.go`**: Add `rootCmd.AddCommand(outdatedCmd)` in `init()`. Update `Execute()` to check for `silentExitError` before printing.

- **Tests — `internal/cli/outdated_test.go`** (new file): Test scenarios using `MockFetcher`:
  1. One outdated dependency → shows update with classification, exit 1
  2. All up to date → shows "(up to date)" for all, exit 0
  3. No pinfile → error
  4. Zero dependencies → "No dependencies to check." message
  5. Fetch failure for one dep → error for that dep, continue checking others, exit 1
  6. Dependency with no semver tags → skip with warning
  7. Major/minor/patch classification correctness

### Success Criteria

#### Automated Verification:
- [ ] Tests pass: `task test`
- [ ] Lint clean: `task lint`

#### Manual Verification:
- [ ] `./craft outdated` shows update status for each dependency
- [ ] Exit code is 1 when updates available, 0 when all current
- [ ] Network failures for individual deps don't block other checks
- [ ] `./craft outdated --verbose` shows fetch operations

---

## Phase 5: Dry-run on install and update

### Objective
Add `--dry-run` flag to `craft install` and `craft update` that runs full resolution but prevents all file writes and installation.

### Changes Required

- **`internal/cli/install.go`**: 
  1. Add `var installDryRun bool` package-level variable
  2. Register `--dry-run` flag in `init()`: `installCmd.Flags().BoolVar(&installDryRun, "dry-run", false, "...")`
  3. In `runInstall()`, after `resolver.Resolve()` succeeds (after line 78), check `installDryRun`. If true:
     a. Print dry-run summary: "Would resolve N dependencies:" followed by each dependency with alias, version, skill count
     b. Print "Would install to: <targetPath>" (still resolve target for informational purposes, but skip actual install)
     c. Print "No changes made."
     d. Return nil (skip `writePinfileAtomic`, `collectSkillFiles`, `verifyIntegrity`, `installlib.Install`, `printDependencyTree`)
  4. Verbose output in dry-run: show resolution steps

- **`internal/cli/update.go`**:
  1. Add `var updateDryRun bool` package-level variable
  2. Register `--dry-run` flag in `init()`: `updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "...")`
  3. In `runUpdate()`, after resolution succeeds, check `updateDryRun`. If true:
     a. Print dry-run summary: what would be updated (version changes), what would stay the same
     b. Print "No changes made."
     c. Return nil (skip `writePinfileAtomic`, `writeManifestAtomic`, `installlib.Install`)

- **Tests — `internal/cli/install_test.go`**: Add dry-run test scenarios:
  1. `install --dry-run` produces summary output
  2. `install --dry-run` does NOT write `craft.pin.yaml` (verify file doesn't exist/isn't modified)
  3. `install --dry-run` does NOT install to target

- **Tests — `internal/cli/update_test.go`**: Add dry-run test scenarios:
  1. `update --dry-run` produces summary output
  2. `update --dry-run` does NOT modify `craft.pin.yaml` or `craft.yaml`

### Success Criteria

#### Automated Verification:
- [ ] Tests pass: `task test`
- [ ] Lint clean: `task lint`

#### Manual Verification:
- [ ] `./craft install --dry-run` shows what would be resolved without writing files
- [ ] `./craft update --dry-run` shows what would change without modifying anything
- [ ] `./craft install --dry-run --verbose` shows resolution steps plus dry-run summary
- [ ] Verify no files created: `ls craft.pin.yaml` after dry-run in clean project should fail

---

## Phase 6: Documentation

### Objective
Update README.md with new commands and flags. Create Docs.md technical reference.

### Changes Required

- **`.paw/work/cli-feature-bundle/Docs.md`**: Technical reference covering all five features — architecture decisions, component interactions, testing approach, usage examples. Load `paw-docs-guidance` skill for template.

- **`README.md`**: Update the Commands table (around line 128) to include `craft list`, `craft tree`, `craft outdated`. Add sections for each new command with usage examples, following the existing pattern of `### craft add` (line 141) and `### craft remove` (line 159). Document `--verbose` and `--dry-run` flags in existing command sections or a new "Global Flags" section.

### Success Criteria

#### Automated Verification:
- [ ] Lint clean: `task lint` (no Go changes, but verify)
- [ ] Build succeeds: `task build`

#### Manual Verification:
- [ ] README accurately describes all new commands and flags
- [ ] Docs.md is complete and accurate
- [ ] Examples in README match actual command output format

---

## References
- Spec: `.paw/work/cli-feature-bundle/Spec.md`
- Research: `.paw/work/cli-feature-bundle/CodeResearch.md`
- Work Shaping: `.paw/work/cli-feature-bundle/WorkShaping.md` (session artifact)
