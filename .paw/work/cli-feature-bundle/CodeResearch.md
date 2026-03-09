---
date: 2026-03-09T06:39:00Z
git_commit: 97a78236715141dc86f122bdc46eb2ed81425db2
branch: feature/cli-feature-bundle
repository: erdemtuna/craft
topic: "CLI Feature Bundle implementation map"
tags: [research, codebase, cli, pinfile, manifest, ui, semver, fetch]
status: complete
last_updated: 2026-03-09
---

# Research: CLI Feature Bundle Implementation Map

## Research Question

Where and how do the existing CLI commands, data layer, UI components, and testing infrastructure work? What are the precise integration points for adding `craft list`, `craft tree`, `craft outdated`, `--verbose`, and `--dry-run`?

## Summary

The codebase follows clean Go conventions with Cobra for CLI, table-driven tests with mock interfaces, and well-separated packages. All building blocks for the five new features already exist — pinfile parsing, manifest parsing, tree rendering, tag listing, semver comparison, and atomic write helpers. The implementation primarily involves wiring existing components into new commands and adding interception points to existing ones.

## Documentation System

- **Framework**: Plain markdown (no generator)
- **Docs Directory**: N/A — no dedicated docs folder
- **Navigation Config**: N/A
- **Style Conventions**: README.md has CLI reference table, command examples with code blocks; CONTRIBUTING.md covers development setup and testing patterns
- **Build Command**: N/A
- **Standard Files**: `README.md` (241 lines), `CONTRIBUTING.md` (193 lines), `CODE_OF_CONDUCT.md`, `LICENSE`; no CHANGELOG.md

## Verification Commands

- **Test Command**: `task test` → `go test -race ./...`
- **Lint Command**: `task lint` → `golangci-lint run ./...` (12 linters)
- **Build Command**: `task build` → `go build -ldflags "..." -o craft ./cmd/craft`
- **Type Check**: N/A (Go compiler handles this during build)
- **Full CI**: `task ci` → fmt:check → vet → lint → vuln → test → build

## Detailed Findings

### Root Command & Command Registration

The root command is defined in `internal/cli/root.go`:
- `rootCmd` variable declaration: `root.go:10-16`
- Properties: `Use: "craft"`, `SilenceUsage: true`, `SilenceErrors: true`
- No `PersistentPreRunE` or `PersistentFlags` defined
- `init()` registers 8 subcommands: `root.go:18-27`
- `Execute()` calls `rootCmd.Execute()`, prints errors to stderr: `root.go:30-36`

Entry point: `cmd/craft/main.go:9-13` — calls `cli.Execute()`, exits with code 1 on error.

### Command Pattern (version.go as template)

Simple command structure in `internal/cli/version.go:8-16`:
- Package-level `var versionCmd = &cobra.Command{...}`
- Uses `Run` (no error return) for simple commands
- Uses `cmd.Printf()` for output
- No `init()` function needed when no flags

Commands with side effects use `RunE` (returns error) — e.g., `install.go:26-32`, `update.go:23-29`.

### Flag Registration Pattern

Flags are registered in package-level `init()` functions with package-level variables:
- `install.go:34-36`: `installCmd.Flags().StringVar(&installTarget, "target", "", "...")`
- `update.go:31-33`: `updateCmd.Flags().StringVar(&updateTarget, "target", "", "...")`
- No persistent/global flags exist on `rootCmd`

### Install Command Flow

Full flow in `internal/cli/install.go`:

| Step | Line(s) | Function/Call |
|------|---------|---------------|
| Command definition | 26-32 | `var installCmd = &cobra.Command{...RunE: runInstall}` |
| Flag registration | 34-36 | `init()` — `--target` flag |
| Handler entry | 38 | `func runInstall(cmd *cobra.Command, args []string) error` |
| Parse manifest | ~41-52 | `manifest.ParseFile()` |
| Load existing pinfile | ~55-65 | `pinfile.ParseFile()` (optional) |
| Create fetcher | 68-71 | `newFetcher()` |
| Resolve dependencies | 75-78 | `resolver.Resolve(m, opts)` |
| **Write pinfile** | **86** | `writePinfileAtomic(pfPath, result.Pinfile)` |
| Resolve install targets | ~90-110 | `resolveInstallTargets(target)` |
| Collect skill files | ~100-108 | `collectSkillFiles(fetcher, result)` |
| Verify integrity | ~109-111 | `verifyIntegrity(result, skillFiles)` |
| **Install to disk** | **112** | `installlib.Install(targetPath, skillFiles)` |
| Print tree | 127 | `printDependencyTree(cmd, m, result)` |

Dry-run interception point: after line 78 (resolve succeeds), before line 86 (first write).

Helper functions:
- `newFetcher()`: `install.go:327-337` — creates cache + GoGitFetcher
- `writePinfileAtomic()`: `install.go:132-136` — wraps `writeAtomic()`
- `writeAtomic()`: `atomic.go:11-31` — temp file + rename pattern
- `printDependencyTree()`: `install.go:211-237` — builds `ui.DepNode` list, calls `ui.RenderTree()`
- `collectSkillFiles()`: `install.go:239-282` — fetches all skill files from remote
- `verifyIntegrity()`: `install.go:287-317` — SHA-256 digest check
- `countSkills()`: `install.go:319-325` — counts total skills
- `resolveInstallTargets()`: `install.go:141-174` — agent auto-detection or explicit path

### Update Command Flow

Full flow in `internal/cli/update.go`:

| Step | Line(s) | Function/Call |
|------|---------|---------------|
| Command definition | 23-29 | `var updateCmd = &cobra.Command{...RunE: runUpdate}` |
| Flag registration | 31-33 | `init()` — `--target` flag |
| Handler entry | 35 | `func runUpdate(cmd *cobra.Command, args []string) error` |
| List tags | 88 | `fetcher.ListTags(cloneURL)` |
| Find latest | ~95 | `semver.FindLatest(tags)` |
| Compare versions | 99 | `semver.Compare(current, latest)` |
| Resolve deps | ~120-130 | `resolver.Resolve(m, opts)` |
| **Write pinfile** | **143** | `writePinfileAtomic(pfPath, result.Pinfile)` |
| **Write manifest** | **148** | `writeManifestAtomic(manifestPath, m)` |
| Install to disk | 165 | `installlib.Install(targetPath, skillFiles)` |

`writeManifestAtomic()`: `update.go:184-188` — same atomic write pattern.

Dry-run interception point: after resolution succeeds, before line 143 (first write).

### Pinfile Data Layer

Types in `internal/pinfile/types.go`:
- `Pinfile` struct: `types.go:6-13` — fields: `PinVersion int`, `Resolved map[string]ResolvedEntry`
- `ResolvedEntry` struct: `types.go:16-32` — fields: `Commit`, `Integrity`, `Source`, `Skills []string`, `SkillPaths []string`
- Map key for `Resolved` is the dependency URL (e.g., `github.com/org/repo`)

Functions:
- `ParseFile(path) (*Pinfile, error)`: `parse.go:27`
- `Parse(r io.Reader) (*Pinfile, error)`: `parse.go:12`
- `Write(p *Pinfile, w io.Writer) error`: `write.go:14`
- `Validate(p *Pinfile) []error`: `validate.go:9`

### Manifest Data Layer

Types in `internal/manifest/types.go`:
- `Manifest` struct: `types.go:6-30` — fields: `SchemaVersion`, `Name`, `Version`, `Description`, `License`, `Skills []string`, `Dependencies map[string]string`, `Metadata`
- `Dependencies` type: `map[string]string` (alias → URL) at line 26

Functions:
- `ParseFile(path) (*Manifest, error)`: `parse.go:28`
- `Parse(r io.Reader) (*Manifest, error)`: `parse.go:13`
- `Write(m *Manifest, w io.Writer) error`: `write.go:14`

### UI Tree Rendering

In `internal/ui/tree.go`:
- `DepNode` struct: `tree.go:11-15` — fields: `Alias string`, `URL string`, `Skills []string`
- `RenderTree(w io.Writer, packageName string, localSkills []string, deps []DepNode)`: `tree.go:19`
- `FormatTree(packageName, localSkills, deps) string`: `tree.go:71`
- Deps sorted by `Alias` ascending: `tree.go:42-47` using `sort.Slice()`
- Uses box-drawing characters: `├──`, `└──`, `│`

### UI Progress

In `internal/ui/progress.go`:
- `Progress` struct: `progress.go:16-20` — fields: `w io.Writer`, `isTTY bool`, `mu sync.Mutex`
- `NewProgress() *Progress`: `progress.go:24` — uses `term.IsTerminal(int(os.Stderr.Fd()))` for TTY detection
- `IsTTY() bool`: `progress.go:87`
- Non-TTY: suppresses progress output entirely (CI-friendly)

### Semver Package

In `internal/semver/semver.go`:
- `Compare(a, b string) int`: `semver.go:11` — returns -1, 0, or 1
- `ParseParts(v string) [3]int`: `semver.go:27` — extracts [major, minor, patch] using `fmt.Sscanf`
- `FindLatest(tags []string) string`: `semver.go:36` — filters `v`-prefixed tags, rejects pre-release (`-`/`+`), returns highest

Version handling:
- Tags must start with `v` prefix: `semver.go:41`
- V-prefix stripped: `tag[1:]` at `semver.go:44`
- Pre-release/build metadata rejected: `strings.ContainsAny(version, "-+")` at `semver.go:46`

### Fetch Layer

Interface in `internal/fetch/fetcher.go:7-22`:
```
GitFetcher interface {
    ResolveRef(url, ref string) (commitSHA string, err error)
    ListTags(url string) ([]string, error)
    ListTree(url, commitSHA string) ([]string, error)
    ReadFiles(url, commitSHA string, paths []string) (map[string][]byte, error)
}
```

Implementation:
- `NewGoGitFetcher(cache *Cache) *GoGitFetcher`: `gogit.go:30`
- `NewCache(root string) (*Cache, error)`: `cache.go:21`
- `DefaultCacheRoot() (string, error)`: `cache.go:29` — returns `~/.craft/cache`

Mock for testing:
- `MockFetcher` struct: `fetch/mock.go` — implements `GitFetcher` with configurable maps (`Refs`, `TagsByURL`, `Trees`, `Files`, `Errors`)

### Error Handling Pattern

Commands return errors via `RunE`; errors flow upward:
1. `RunE` returns `fmt.Errorf("context: %w", err)` — e.g., `install.go:41,50,52,81,86,114`
2. Cobra receives error but doesn't print (due to `SilenceErrors: true`)
3. `Execute()` in `root.go:30-36` prints error to stderr
4. `main()` in `cmd/craft/main.go:10-11` calls `os.Exit(1)` on non-nil error

### Testing Infrastructure

**CLI test pattern** (`internal/cli/cli_test.go:11-46`):
- Cobra command testing: `rootCmd.SetOut(buf)` → `rootCmd.SetArgs(args)` → `rootCmd.Execute()`
- Output validation via buffer contents
- Used for `TestVersionCommand` (line 11) and `TestRootHelp` (line 30)

**Test helpers** (`internal/cli/add_test.go:11-35`):
- `testChdir(t, dir)`: changes dir with cleanup (line 11)
- `testWriteFile(t, path, data)`: writes test files (line 23)
- `testMkdirAll(t, path)`: creates directories (line 30)

**Mock fetcher** (`internal/fetch/mock.go`):
- `MockFetcher` with maps: `Refs`, `TagsByURL`, `Trees`, `Files`, `Errors`
- Used extensively in `install_test.go` and `update_test.go`

**Test files by package**:

| Package | Test Files | Lines |
|---------|-----------|-------|
| `internal/cli/` | 7 files: add, cache, cli, install, remove, update, validate | ~1,234 |
| `internal/pinfile/` | 3 files: parse, validate, write | ~398 |
| `internal/manifest/` | 3 files: parse, validate, write | ~495 |
| `internal/ui/` | 2 files: progress, tree | ~100 |
| `internal/semver/` | 1 file: semver | ~100 |
| `internal/fetch/` | 3 files: auth, cache, fetcher | ~200 |

**Testdata fixtures** at `testdata/`:
- `manifests/`: minimal.yaml, valid.yaml, with-extras.yaml
- `pinfiles/`: valid.yaml
- `packages/`: 7 package fixtures (collision, escape, missing-skill, pinfile-mismatch, pinfile-no-deps, valid, no-manifest)
- `skills/`: 5 SKILL.md fixtures

### Data Flow for New Commands

**craft list** data flow:
1. `manifest.ParseFile("craft.yaml")` → get `Dependencies` map (alias → URL) and package `Name`/`Version`
2. `pinfile.ParseFile("craft.pin.yaml")` → get `Resolved` map (URL → ResolvedEntry with Skills)
3. Join on URL: for each manifest dependency alias/URL, look up pinfile entry to get version (from commit tag) and skills
4. Sort by alias, print table

**Observation**: The pinfile `Resolved` map is keyed by URL, and the `ResolvedEntry` does not store the resolved version tag directly — it stores `Commit` (SHA). To display versions, we need to either:
- Store version info during resolution (not currently done in pinfile)
- Re-derive version from tags (expensive)
- Parse version from the dependency URL if it includes a version constraint

This is a constraint that affects `craft list` and `craft outdated` design. The manifest `Dependencies` map stores `alias → URL` where URL may include a version constraint (e.g., `github.com/org/repo@v1.0.0`).

**craft tree** data flow:
1. Same as list: parse manifest + pinfile
2. Build `[]ui.DepNode` from joined data
3. Call `ui.RenderTree(os.Stdout, packageName, localSkills, deps)`

**craft outdated** data flow:
1. Parse manifest + pinfile (same join)
2. For each direct dependency URL: `fetcher.ListTags(url)` → `semver.FindLatest(tags)`
3. Compare pinned version vs latest: `semver.Compare()`
4. Classify: compare `ParseParts()` results to determine major/minor/patch
5. Print table, exit code 1 if any outdated

## Code References

- `internal/cli/root.go:10-16` — Root command definition
- `internal/cli/root.go:18-27` — Command registration
- `internal/cli/root.go:30-36` — Execute function
- `internal/cli/version.go:8-16` — Simple command template
- `internal/cli/install.go:26-32` — Install command definition
- `internal/cli/install.go:38` — runInstall handler
- `internal/cli/install.go:75-78` — Resolver.Resolve call
- `internal/cli/install.go:86` — writePinfileAtomic (dry-run boundary)
- `internal/cli/install.go:112` — installlib.Install (dry-run boundary)
- `internal/cli/install.go:127` — printDependencyTree
- `internal/cli/install.go:211-237` — printDependencyTree implementation
- `internal/cli/install.go:327-337` — newFetcher helper
- `internal/cli/update.go:23-29` — Update command definition
- `internal/cli/update.go:35` — runUpdate handler
- `internal/cli/update.go:88` — ListTags call
- `internal/cli/update.go:99` — semver.Compare call
- `internal/cli/update.go:143` — writePinfileAtomic (dry-run boundary)
- `internal/cli/update.go:148` — writeManifestAtomic (dry-run boundary)
- `internal/cli/update.go:165` — installlib.Install (dry-run boundary)
- `internal/cli/atomic.go:11-31` — writeAtomic helper
- `internal/pinfile/types.go:6-13` — Pinfile struct
- `internal/pinfile/types.go:16-32` — ResolvedEntry struct
- `internal/pinfile/parse.go:12,27` — Parse, ParseFile
- `internal/manifest/types.go:6-30` — Manifest struct
- `internal/manifest/parse.go:13,28` — Parse, ParseFile
- `internal/ui/tree.go:11-15` — DepNode struct
- `internal/ui/tree.go:19` — RenderTree function
- `internal/ui/tree.go:71` — FormatTree function
- `internal/ui/progress.go:16-20` — Progress struct
- `internal/ui/progress.go:24` — NewProgress (TTY detection)
- `internal/semver/semver.go:11` — Compare function
- `internal/semver/semver.go:27` — ParseParts function
- `internal/semver/semver.go:36` — FindLatest function
- `internal/fetch/fetcher.go:7-22` — GitFetcher interface
- `internal/fetch/gogit.go:30` — NewGoGitFetcher
- `internal/fetch/cache.go:21,29` — NewCache, DefaultCacheRoot
- `internal/fetch/mock.go` — MockFetcher for testing
- `internal/cli/cli_test.go:11-46` — Cobra command test pattern
- `internal/cli/add_test.go:11-35` — Test helper functions
- `Taskfile.yml:21-24` — test task
- `Taskfile.yml:49-52` — lint task
- `Taskfile.yml:16-19` — build task
- `Taskfile.yml:59-67` — ci task (full pipeline)

## Architecture Documentation

**Command registration**: Package-level `var xCmd = &cobra.Command{...}` + `init()` for flags + `rootCmd.AddCommand(xCmd)` in `root.go init()`.

**Data flow pattern**: Commands call `manifest.ParseFile()` and `pinfile.ParseFile()` to load state, use `fetch` layer for remote operations, and `ui` layer for output.

**Atomic writes**: All file mutations use `writeAtomic()` (temp file + `os.Rename`) to prevent partial writes.

**Testing convention**: Table-driven tests, `MockFetcher` for network isolation, `testdata/` fixtures for YAML parsing, `rootCmd.SetArgs()`/`Execute()` for command integration tests.

**Dependency URL format**: Manifest stores `alias → URL` where URL may include version constraint. Pinfile stores `URL → ResolvedEntry` with commit SHA but no version tag.

### Resolve Layer Types

In `internal/resolve/types.go`:
- `ResolvedDep` struct: `types.go:4-26` — fields: `URL` (full URL with version, e.g., `github.com/example/skills@v1.0.0`), `Alias`, `Commit`, `Integrity`, `Skills []string`, `SkillPaths []string`, `Source`
- `ResolveResult` struct: `resolver.go:44-50` — fields: `Resolved []ResolvedDep`, `Pinfile *pinfile.Pinfile`

In `internal/resolve/depurl.go`:
- `DepURL` struct: `depurl.go:15-30` — parsed URL with `Host`, `Org`, `Repo`, `Version` (without v-prefix)
- `ParseDepURL(raw string) (*DepURL, error)`: `depurl.go:34` — parses `host/org/repo@vMAJOR.MINOR.PATCH`
- `PackageIdentity() string`: `depurl.go:52` — returns `host/org/repo` (no version)
- `GitTag() string`: `depurl.go:57` — returns `v` + version
- `WithVersion(version string) string`: `depurl.go:73` — creates new URL with different version

**Version in pinfile keys**: The pinfile `Resolved` map key **includes the version** (e.g., `github.com/example/git-skills@v1.0.0` — see `testdata/pinfiles/valid.yaml:4`). Version is directly extractable via `ParseDepURL()` on the key. This resolves the version display question for `craft list` and `craft outdated`.

## Open Questions

None — all implementation details are mapped.
