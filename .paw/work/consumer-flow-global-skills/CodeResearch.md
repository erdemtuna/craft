---
date: 2026-03-09T18:00:00Z
git_commit: 4272bcbb15de8a34001ff2f201399a67893eb3ae
branch: feature/consumer-flow-global-skills
repository: erdemtuna/craft
topic: "Consumer Flow & Global Skills Management — Implementation Map"
tags: [research, codebase, cli, manifest, install, agent, global]
status: complete
last_updated: 2026-03-09
---

# Research: Consumer Flow & Global Skills Management

## Research Question

Where and how do the existing CLI commands, manifest/pinfile types, installer, agent detection, and resolver work? What must change to support `craft get`, `--global` flag, and `forge/` vendoring?

## Summary

The craft codebase is well-structured with clear separation: CLI commands in `internal/cli/`, core types in `internal/manifest/` and `internal/pinfile/`, resolution in `internal/resolve/`, installation in `internal/install/`, and agent detection in `internal/agent/`. Key reusable components for the new features include `resolveInstallTargets()`, `collectSkillFiles()`, `verifyIntegrity()`, the `Resolver`, and `Install()`. The main constraint is that `manifest.Validate()` requires non-empty `Skills`, which must be relaxed for global manifests. All commands load manifests independently from the current working directory — a new code path is needed to load from `~/.craft/` when `-g` is set.

## Documentation System

- **Framework**: Plain markdown (no doc site framework)
- **Docs Directory**: N/A — docs are in README.md and CONTRIBUTING.md at repo root
- **Navigation Config**: N/A
- **Style Conventions**: Concise sections with code examples, tabular comparisons, emoji-free
- **Build Command**: N/A
- **Standard Files**: `README.md` (comprehensive user docs), `CONTRIBUTING.md` (dev guide), `CODE_OF_CONDUCT.md`, `LICENSE`

## Verification Commands

- **Test Command**: `task test` → `go test -race ./...`
- **Lint Command**: `task lint` → `golangci-lint run`
- **Build Command**: `task build` → `go build` with ldflags version injection
- **Type Check**: `task vet` → `go vet ./...`
- **Full CI**: `task ci` → fmt:check → vet → lint → vuln → test → build

## Detailed Findings

### CLI Command Registration

Root command defined at `internal/cli/root.go:10-16`. All 11 subcommands registered in `init()` at lines 21-31. Single global flag `--verbose/-v` registered as `PersistentFlags` at line 19 via `verbose.go:12`.

Each command follows the pattern: cobra `Command` struct with `RunE` function, flags registered in command-local `init()`. No shared command factory — each file is self-contained.

### Manifest Loading Patterns

Commands load manifests independently — no shared `loadManifest()` function:

- `add.go:76` — `manifest.ParseFile()` direct call
- `install.go:49` — `manifest.ParseFile()` direct call
- `update.go:50` — `manifest.ParseFile()` direct call
- `remove.go:41` — `manifest.ParseFile()` direct call
- `helpers.go:16-42` — `requireManifestAndPinfile()` shared helper loads both manifest and pinfile (used by `validate`, `list`, `tree`)

All paths resolve from current working directory. For `-g` support, a new path resolution mechanism is needed that reads from `~/.craft/` instead.

### Manifest Type & Validation

`internal/manifest/types.go:8-26` — Manifest struct:
- **Required (no omitempty)**: `SchemaVersion` (int), `Name` (string), `Skills` ([]string)
- **Optional (omitempty)**: `Description`, `License`, `Dependencies`, `Metadata`

`internal/manifest/validate.go:19-49` — Validation rules:
- `SchemaVersion` must be exactly `1` (line 23-25)
- `Name` required, must match `^[a-z][a-z0-9]*(-[a-z0-9]+)*$` (lines 28-34)
- **`Skills` must be non-empty** (line 37-39: "must contain at least one skill path") — **this blocks global manifests with no skills**
- `Dependencies` URLs validated if present (lines 42-46)

`internal/manifest/write.go:14-46` — Deterministic YAML serialization with hardcoded field ordering. Empty Skills serialized as empty sequence.

`internal/manifest/parse.go:13-25` — `Parse(r io.Reader)` unmarshals YAML. `ParseFile(path)` at lines 28-36 wraps with file I/O.

### Pinfile Type

`internal/pinfile/types.go:8-35` — Pinfile struct:
- `PinVersion` (int), `Resolved` (map[string]ResolvedEntry)
- ResolvedEntry: `Commit`, `RefType` (omitempty, defaults "tag"), `Integrity`, `Source` (omitempty), `Skills`, `SkillPaths` (omitempty)

`internal/pinfile/parse.go:12-33` — Parse with auto-default of empty `RefType` to `"tag"`.
`internal/pinfile/write.go:14-87` — Deterministic serialization sorted by URL key.

### Dependency URL Parsing

`internal/resolve/depurl.go:40-63` — DepURL struct: `Raw`, `Host`, `Org`, `Repo`, `Version`, `Ref`, `RefType`.

`ParseDepURL()` at line 72 — requires `@` separator and non-empty ref. Three ref types:
- Tag: `@vMAJOR.MINOR.PATCH`
- Commit: `@<hex7-64>`
- Branch: `@branch:<name>`

`PackageIdentity()` at line 120 returns `host/org/repo` — used as namespace prefix for skill paths.

### Resolver

`internal/resolve/resolver.go:24-31` — `Resolver` wraps `GitFetcher`.

`Resolve()` at line 53 — takes `Manifest` + `ResolveOptions` (ExistingPinfile, ForceResolve map). Phases: collect deps recursively → cycle detection → MVS selection → resolve commits → build pinfile.

Existing pinfile reuse at lines 332-341: tags and commits reused unless in `ForceResolve`; branches always re-resolved.

`collectDeps()` at line 260 — recursive with `maxResolutionDepth=20`, `maxTotalDeps=200`.

### Installer

`internal/install/installer.go:17-88` — `Install(target string, skills map[string]map[string][]byte)`:
- Atomic staging: writes to `.staging` dir, then renames
- Path traversal protection at lines 33 and 55
- Permissions: dirs `0o700`, files `0o644`

This function is target-agnostic — works for both agent directories and `forge/`. No changes needed to the installer itself.

### Agent Detection

`internal/agent/detect.go:48-67` — `Detect(homeDir)` returns single agent or error.
`internal/agent/detect.go:71-78` — `DetectAll(homeDir)` returns all agents (no error).

Detection markers at lines 85-103:
- ClaudeCode: `~/.claude/` → `~/.claude/skills/`
- Copilot: `~/.copilot/` → `~/.copilot/skills/`

Interactive prompt in `internal/cli/install.go:185-217` — `promptAgentChoice()` shows numbered menu, supports "Both" option.

### Install Command Pipeline

`internal/cli/install.go:40-139` — `runInstall()` flow:
1. Load manifest (line 49)
2. Load existing pinfile (line 63-67)
3. Create fetcher (line 70-73)
4. Resolve (line 76-85)
5. Dry-run check (line 88-92)
6. Write pinfile atomically (line 95)
7. Resolve install targets (line 100)
8. Collect skill files (line 106)
9. Verify integrity (line 113)
10. Install to targets (line 119-125)

`resolveInstallTargets()` at line 147 — handles `--target` override, single/multi agent, TTY prompt.
`collectSkillFiles()` at line 245 — fetches skill files, returns `map[compositeKey]map[filename][]byte`.
`verifyIntegrity()` at line 296 — compares computed digests against pinfile entries.
`newFetcher()` at line 343 — creates `GoGitFetcher` with default cache at `~/.craft/cache/`.

### Add Command

`internal/cli/add.go:19-32` — `add [alias] <url>`, 1-2 args.
Flags: `--install` (line 35), `--target` (line 36).

Alias derivation at line 65-67: `alias = parsed.Repo` if not provided.

`--install` flow at lines 161-185: writes pinfile → resolves targets → collects files → installs. This currently installs to agent dirs — must change to vendor to `forge/`.

### Remove Command & Cleanup

`internal/cli/remove.go:31-150` — `runRemove()`:
1. Load manifest, validate alias exists (lines 32-54)
2. Extract orphaned skills from pinfile (lines 56-66)
3. Delete from manifest + write atomically (lines 68-76)
4. Delete from pinfile + write atomically (lines 78-86)
5. Cleanup: delete skill dirs from install targets with path traversal protection (lines 92-146)

`cleanEmptyParents()` at lines 152-169 — walks up directory tree removing empty dirs, stops at root boundary.

Remove currently cleans from agent install targets — for `-g`, it would clean from agent dirs using global manifest/pinfile.

### Update Command

`internal/cli/update.go:24-34` — `update [alias]` with `--target` and `--dry-run`.
Main loop at lines 87-149: commits skipped, branches re-resolve HEAD, tags check for latest semver.
Post-update: force-resolves, writes manifest+pinfile atomically, installs (lines 162-219).

### List Command

`internal/cli/list.go:15-21` — `list` with `--detailed`.
Loads via `requireManifestAndPinfile()`. Builds alias lookup, sorts alphabetically, outputs via tabwriter.

### Tree Command

`internal/cli/tree.go:11-17` — `tree`, no flags.
Loads via `requireManifestAndPinfile()`. Extracts local skills + deps, delegates to `ui.RenderTree()`.

### Validate Command

`internal/cli/validate.go:13-48` — `validate`, no flags.
Creates `validate.Runner` and executes. Checks: schema, skill paths, frontmatter, dep URLs, pinfile consistency, name collisions.

### Outdated Command

`internal/cli/outdated.go:15-21` — `outdated`, no flags.
Loads via `requireManifestAndPinfile()`. Skips commit/branch deps, checks tags for newer semver. Exit code 1 if updates found.

### Atomic Write Infrastructure

`internal/cli/atomic.go:11-31` — `writeAtomic(path, writeFn)` writes to `.tmp`, renames atomically.
`writePinfileAtomic()` at `install.go:141-145`.
`writeManifestAtomic()` at `update.go:227-231`.

### Cache Infrastructure

`internal/fetch/cache.go:29` — `DefaultCacheRoot()` returns `~/.craft/cache/`.
Cache paths use SHA-256 hash of normalized URL for collision resistance.

### Testing Patterns

Test helpers: `testChdir()`, `testWriteFile()`, `testMkdirAll()` — create temp dirs, change cwd, cleanup on test end.

Mock fetcher: `fetch.NewMockFetcher()` used in unit tests for offline resolution.

Test execution pattern: `rootCmd.SetArgs()` + `rootCmd.SetOut/SetErr()` + `Execute()` for CLI integration tests.

36 total test files. Key test files: `install_test.go` (23 tests), `list_test.go` (10 tests), `outdated_test.go` (8 tests).

Fixtures in `testdata/`: `manifests/`, `pinfiles/`, `packages/` (full package structures), `skills/` (individual SKILL.md files).

## Code References

- `internal/cli/root.go:10-31` — Root command and registration
- `internal/cli/root.go:19` — Verbose persistent flag (model for `-g`)
- `internal/cli/verbose.go:12` — Verbose flag variable
- `internal/cli/helpers.go:16-42` — `requireManifestAndPinfile()` shared loader
- `internal/cli/helpers.go:44-66` — `printDryRunSummary()`
- `internal/cli/atomic.go:11-31` — `writeAtomic()` generic helper
- `internal/cli/add.go:19-188` — Add command full implementation
- `internal/cli/add.go:65-67` — Alias auto-derivation
- `internal/cli/add.go:161-185` — `--install` flow
- `internal/cli/install.go:27-139` — Install command full implementation
- `internal/cli/install.go:141-145` — `writePinfileAtomic()`
- `internal/cli/install.go:147-183` — `resolveInstallTargets()`
- `internal/cli/install.go:185-217` — `promptAgentChoice()`
- `internal/cli/install.go:245-291` — `collectSkillFiles()`
- `internal/cli/install.go:296-333` — `verifyIntegrity()`
- `internal/cli/install.go:343-353` — `newFetcher()`
- `internal/cli/remove.go:31-150` — Remove command with cleanup
- `internal/cli/remove.go:152-169` — `cleanEmptyParents()`
- `internal/cli/update.go:24-224` — Update command
- `internal/cli/update.go:227-231` — `writeManifestAtomic()`
- `internal/cli/list.go:15-106` — List command
- `internal/cli/tree.go:11-69` — Tree command
- `internal/cli/validate.go:13-48` — Validate command
- `internal/cli/outdated.go:15-204` — Outdated command
- `internal/manifest/types.go:8-26` — Manifest struct
- `internal/manifest/validate.go:19-49` — Validation rules (Skills non-empty at line 37)
- `internal/manifest/parse.go:13-36` — Parse/ParseFile
- `internal/manifest/write.go:14-46` — Write with field ordering
- `internal/pinfile/types.go:8-35` — Pinfile/ResolvedEntry structs
- `internal/pinfile/parse.go:12-44` — Parse with RefType default
- `internal/pinfile/write.go:14-87` — Write sorted by URL
- `internal/resolve/resolver.go:24-31` — Resolver type
- `internal/resolve/resolver.go:34-50` — ResolveOptions/ResolveResult
- `internal/resolve/resolver.go:53-257` — Resolve() main function
- `internal/resolve/depurl.go:40-63` — DepURL struct
- `internal/resolve/depurl.go:72` — ParseDepURL()
- `internal/resolve/depurl.go:120` — PackageIdentity()
- `internal/install/installer.go:17-88` — Install() atomic skill writer
- `internal/agent/detect.go:48-67` — Detect() single agent
- `internal/agent/detect.go:71-78` — DetectAll() all agents
- `internal/agent/detect.go:85-103` — Detection markers
- `internal/fetch/cache.go:29` — DefaultCacheRoot() → `~/.craft/cache/`
- `Taskfile.yml:21-24` — Test command
- `Taskfile.yml:49-52` — Lint command
- `Taskfile.yml:59-67` — CI pipeline

## Architecture Documentation

**Command Pattern**: Each CLI command is a self-contained file with cobra Command definition, init() for flags, and RunE function. No command factory or base class.

**Manifest Loading**: Commands independently call `manifest.ParseFile()` from cwd, or use `requireManifestAndPinfile()` helper. Path is always `filepath.Join(cwd, "craft.yaml")`.

**Atomic Operations**: All file writes use temp-file-then-rename pattern via `writeAtomic()`. Install uses staging directories with atomic swap.

**Resolution**: MVS (Minimum Version Selection) algorithm. Existing pins reused unless forced. Branch deps always re-resolved. Max depth 20, max total deps 200.

**Install Pipeline**: Resolve → write pinfile → detect agent → collect files → verify integrity → atomic install. Target-agnostic — `Install()` accepts any directory path.

**Global Flag Model**: `--verbose/-v` is the only existing persistent flag. Defined as package-level var in `verbose.go`, registered as `PersistentFlags` in `root.go`. This is the model for adding `--global/-g`.

## Open Questions

None — research is comprehensive for the Spec requirements.
