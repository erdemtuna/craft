---
date: 2026-03-09T09:45:00Z
git_commit: f43eb6a
branch: feature/non-tagged-deps
repository: erdemtuna/craft
topic: "Non-Tagged Repository Dependencies - Implementation Map"
tags: [research, codebase, depurl, resolver, fetcher, pinfile, cli, validate]
status: complete
last_updated: 2026-03-09
---

# Research: Non-Tagged Repository Dependencies

## Research Question

What existing code structures, patterns, and integration points must be understood to extend craft's dependency system with commit SHA and branch ref support?

## Summary

The codebase has a clean, layered architecture with well-separated concerns. The dependency URL parsing (`depurl.go`) is the primary entry point — its regex enforces semver-only refs. The resolver uses a 6-phase pipeline with MVS, and the fetcher's `ResolveRef` method **already supports branch resolution** (tries tag → local branch → remote branch). Key changes center on: (1) extending `depurl.go` parsing and struct with RefType, (2) adding ref-type-aware routing in the resolver, (3) extending pinfile types with `ref_type` field, (4) modifying `add.go`/`update.go` for new ref types, and (5) adding validation warnings.

## Documentation System

- **Framework**: Plain markdown (no static site generator)
- **Docs Directory**: N/A (README.md at root)
- **Navigation Config**: N/A
- **Style Conventions**: README with problem statement, comparison tables, code blocks, step-by-step instructions. CONTRIBUTING.md with setup/testing/PR guidelines.
- **Build Command**: N/A
- **Standard Files**: README.md (root), CONTRIBUTING.md (root, 194 lines), CODE_OF_CONDUCT.md (root), LICENSE (root)

## Verification Commands

- **Test Command**: `task test` → `go test -race ./...`
- **Lint Command**: `task lint` → `golangci-lint run ./...`
- **Build Command**: `task build` → `go build -ldflags "..." -o craft ./cmd/craft`
- **Type Check**: `go vet ./...` (via `task vet`)
- **Full CI**: `task ci` → fmt:check, vet, lint, vuln, test, build
- **Pre-commit Hook**: `.githooks/pre-commit` — gofmt check + go vet
- **Pre-push Hook**: `.githooks/pre-push` — golangci-lint + full test suite

## Detailed Findings

### Dependency URL Parsing (depurl)

The `DepURL` struct and parsing are in `internal/resolve/depurl.go`.

**Regex** (`depurl.go:12`): Enforces strict `host/org/repo@vMAJOR.MINOR.PATCH` format only. This is the primary gate that must be extended.

```
^([a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?)/([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)@v((0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*))$
```

**DepURL Struct** (`depurl.go:15-30`): Fields: Raw, Host, Org, Repo, Version. The `Version` field is semver-only (no 'v' prefix). Must add `Ref` and `RefType` fields.

**ParseDepURL** (`depurl.go:34-47`): Single entry point. Returns `*DepURL, error`. Error message currently says "expected host/org/repo@vMAJOR.MINOR.PATCH".

**Key Methods**:
- `PackageIdentity()` (`depurl.go:52-54`): Returns `host/org/repo` — ref-type-agnostic, no changes needed.
- `GitTag()` (`depurl.go:57-59`): Returns `"v" + d.Version` — used by resolver to call `ResolveRef`. Must be generalized to return the appropriate ref string based on RefType.
- `WithVersion()` (`depurl.go:73-76`): Returns URL with new version — used by update command. Must be adapted for non-tag ref types.
- `String()` (`depurl.go:79-81`): Returns `d.Raw`.

**Callers** of `ParseDepURL` (7 call sites):
- `internal/resolve/resolver.go:84,95,98,107,221,279` — MVS grouping, version comparison, collectDeps, resolveOne
- `internal/cli/add.go:45` — URL validation on add
- `internal/cli/update.go:84` — URL parsing for update
- `internal/manifest/validate.go:53-55` — depURLPattern used directly (separate regex copy)

**CRITICAL**: `internal/manifest/validate.go:19` has a **separate copy** of the dep URL regex used for manifest validation. Both must be updated together.

### Resolution Pipeline (resolver)

**File**: `internal/resolve/resolver.go` (537 lines)

**6-Phase Pipeline** (`resolver.go:53-210`):
1. **Phase 1** (`resolver.go:62-69`): `collectDeps()` — recursive traversal, builds dependency graph
2. **Phase 2** (`resolver.go:72-74`): Cycle detection via DFS
3. **Phase 3** (`resolver.go:76-167`): MVS — groups by `PackageIdentity()`, uses `semver.Compare()` to select highest version. **This is the core area needing ref-type-aware routing**: non-tagged deps skip MVS comparison.
4. **Phase 4** (`resolver.go:174-182`): `resolveOne()` — resolves commit SHA, discovers skills, computes integrity
5. **Phase 5** (`resolver.go:189-192`): Skill name collision detection
6. **Phase 6** (`resolver.go:194-209`): Build pinfile from resolved deps

**collectDeps** (`resolver.go:213-275`):
- Calls `ParseDepURL` at line 221
- Calls `r.fetcher.ResolveRef(cloneURL, parsed.GitTag())` at line 247 — this must use the appropriate ref string
- Reads transitive `craft.yaml` at lines 252-271

**resolveOne** (`resolver.go:278-314`):
- Checks pinfile reuse at lines 285-293
- Calls `r.fetcher.ResolveRef(cloneURL, parsed.GitTag())` at line 297
- Discovers skills and computes integrity at lines 304-311

**MVS Phase Detail** (`resolver.go:80-167`):
- Groups deps by `PackageIdentity()` (`resolver.go:82-89`)
- Selects highest version via `semver.Compare()` (`resolver.go:92-115`)
- Re-collects transitive deps for version changes (`resolver.go:117-167`)
- **Key constraint**: For non-tagged deps, there's no version to compare. Must either skip MVS or use identity-only grouping with conflict detection.

**ResolveOptions** (`resolver.go:34-41`): `ExistingPinfile` and `ForceResolve` — no ref-type awareness yet.

### Fetcher Interface & Implementation

**Interface** (`internal/fetch/fetcher.go:7-22`): 4 methods — `ResolveRef`, `ListTags`, `ListTree`, `ReadFiles`.

**ResolveRef Implementation** (`internal/fetch/gogit.go:42-79`):
- Already tries: tag → local branch → remote branch
- **No changes needed for basic branch resolution** — passing a branch name to `ResolveRef` already works
- For commit SHA resolution: the fetcher doesn't currently try to resolve a raw commit SHA. Need to add commit SHA lookup (check if hash exists in repo).

**MockFetcher** (`internal/fetch/mock.go`):
- `Refs` map: `"url:ref" → commitSHA` — supports arbitrary refs already
- No changes needed to mock structure

**NormalizeCloneURL** (`gogit.go:310-317`): Converts package identity to HTTPS clone URL. Ref-type-agnostic.

### Pinfile Types and Serialization

**Types** (`internal/pinfile/types.go:6-32`):
- `Pinfile`: `PinVersion int`, `Resolved map[string]ResolvedEntry`
- `ResolvedEntry`: `Commit`, `Integrity`, `Source`, `Skills`, `SkillPaths`
- **Must add**: `RefType string \`yaml:"ref_type,omitempty"\`` to `ResolvedEntry`

**Write** (`internal/pinfile/write.go:14-82`): Custom YAML serialization for deterministic output. Writes fields in fixed order: commit, integrity, source, skills, skill_paths. **Must add `ref_type` field** to the write order (after commit, before integrity).

**Parse** (`internal/pinfile/parse.go`): Standard YAML unmarshal — adding `ref_type` field to struct with `omitempty` provides backward compatibility automatically.

**Validate** (`internal/pinfile/validate.go`): Checks `pin_version=1`, required fields. May need to validate `ref_type` values.

### Resolve Types

**ResolvedDep** (`internal/resolve/types.go:4-26`): URL, Alias, Commit, Integrity, Skills, SkillPaths, Source. **Must add RefType field** to carry ref type through the resolution pipeline.

### craft add Command

**File**: `internal/cli/add.go` (162 lines)

**Flow** (`add.go:35-162`):
1. Parse args → alias + depURL (`add.go:36-42`)
2. Validate URL via `ParseDepURL` (`add.go:45-48`) — **must accept non-tagged refs**
3. Derive alias from repo name (`add.go:51-53`)
4. Parse existing manifest (`add.go:55-68`)
5. Check for existing dep (`add.go:71-79`)
6. Full resolution to verify dep exists (`add.go:100-107`)
7. Write manifest atomically (`add.go:118-121`)
8. Print summary with `parsed.GitTag()` (`add.go:132`) — **must adapt for non-tag refs**
9. Optional `--install` flag (`add.go:135-159`)

**Hint message** (`add.go:47`): `"hint: expected format: github.com/org/repo@v1.0.0"` — must be updated.
**Long description** (`add.go:22`): `"The URL must be in the format host/org/repo@vMAJOR.MINOR.PATCH"` — must be updated.

### craft update Command

**File**: `internal/cli/update.go` (197 lines)

**Flow** (`update.go:37-191`):
1. Parse manifest (`update.go:45-57`)
2. For each dep: `ParseDepURL` → `ListTags` → `semver.FindLatest` → update if newer (`update.go:78-107`)
3. Resolve updated deps (`update.go:120-143`)
4. Write pinfile → manifest → install (`update.go:152-188`)

**Key adaptation points**:
- Line 84: `ParseDepURL` — must handle non-tagged URLs
- Lines 90-99: Tag listing + semver.FindLatest — **only applies to tag deps**
- Line 102: `WithVersion` — **only for tag deps**
- For branch deps: re-resolve branch HEAD, update pinfile if commit changed
- For commit deps: skip silently

### craft install Command

**File**: `internal/cli/install.go` (346 lines)

**Flow**: Reads manifest → resolves (reuses pinfile if available) → verifies integrity → installs skill files.
- Uses `Resolve()` with `ExistingPinfile` and no `ForceResolve` — existing pinfile entries are reused.
- Integrity verification at lines 113-116.
- No ref-type-specific logic needed beyond what the resolver handles.

### Validation (validate package)

**Runner** (`internal/validate/runner.go:28-55`): Executes 7 checks. Currently no dependency-specific warnings for non-tagged deps.

**Error/Warning types** (`internal/validate/errors.go`):
- `Error`: Category, Path, Field, Message, Suggestion
- `Warning`: Message only (simple string)
- `Result`: `Errors []*Error`, `Warnings []*Warning`, `OK() bool`
- Categories: Schema, SkillPath, Frontmatter, Dependency, Pinfile, Collision, Safety

**Adding non-tagged warnings**: Create a new check method (e.g., `checkNonTaggedDeps`) that parses each dep URL, checks RefType, and appends `Warning` entries. The `CategoryDependency` category exists but is currently unused — appropriate for non-tagged warnings.

**Manifest Validation** (`internal/manifest/validate.go:53-55`): Validates dep URLs against `depURLPattern` regex. **This regex must be updated** to accept non-tagged ref formats, or the validation logic must be restructured to use `ParseDepURL` instead of a separate regex.

### Testing Patterns

**Test files** (relevant):
- `internal/resolve/depurl_test.go` — depurl parsing tests
- `internal/resolve/resolver_test.go` — resolver integration tests with MockFetcher
- `internal/cli/add_test.go` — add command tests
- `internal/cli/update_test.go` — update command tests
- `internal/cli/install_test.go` — install command tests
- `internal/cli/validate_test.go` — validate command tests
- `internal/pinfile/write_test.go` — pinfile serialization tests
- `internal/pinfile/parse_test.go` — pinfile parsing tests
- `internal/fetch/mock.go` — MockFetcher for unit tests

**Patterns**:
- Table-driven tests with `t.Run(name, func(t *testing.T) {...})`
- MockFetcher with pre-populated `Refs`, `Trees`, `Files` maps
- `testdata/` directory for fixture files (manifests, pinfiles, skill packages)
- Error injection via `MockFetcher.Errors` map

### Manifest Dependency URL Validation (CRITICAL)

**Two regex copies exist**:
1. `internal/resolve/depurl.go:12` — used by `ParseDepURL()`
2. `internal/manifest/validate.go:19` — used by `Validate()` for dep URL format checking

Both enforce `@vMAJOR.MINOR.PATCH` only. Both must be updated to accept commit SHA and branch refs, or the manifest validation must delegate to `ParseDepURL`.

## Code References

- `internal/resolve/depurl.go:12` — depURLPattern regex (primary gate)
- `internal/resolve/depurl.go:15-30` — DepURL struct
- `internal/resolve/depurl.go:34-47` — ParseDepURL function
- `internal/resolve/depurl.go:57-59` — GitTag method
- `internal/resolve/depurl.go:73-76` — WithVersion method
- `internal/resolve/types.go:4-26` — ResolvedDep struct
- `internal/resolve/resolver.go:53-210` — Resolve 6-phase pipeline
- `internal/resolve/resolver.go:80-167` — MVS phase with semver.Compare
- `internal/resolve/resolver.go:213-275` — collectDeps recursive traversal
- `internal/resolve/resolver.go:278-314` — resolveOne single dep resolution
- `internal/fetch/fetcher.go:7-22` — GitFetcher interface
- `internal/fetch/gogit.go:42-79` — ResolveRef implementation (already handles branches)
- `internal/fetch/mock.go:7-78` — MockFetcher for testing
- `internal/pinfile/types.go:6-32` — Pinfile and ResolvedEntry structs
- `internal/pinfile/write.go:14-82` — Deterministic YAML write
- `internal/manifest/types.go:6-30` — Manifest struct
- `internal/manifest/validate.go:19` — Duplicate depURLPattern regex (CRITICAL)
- `internal/manifest/validate.go:53-55` — Dep URL validation in manifest
- `internal/cli/add.go:35-162` — runAdd function
- `internal/cli/add.go:45-48` — ParseDepURL call + hint message
- `internal/cli/update.go:37-191` — runUpdate function
- `internal/cli/update.go:84-106` — Tag listing + version comparison
- `internal/validate/runner.go:28-55` — Validation runner
- `internal/validate/errors.go:44-47` — Warning type
- `internal/semver/semver.go:11-23` — Compare function
- `internal/semver/semver.go:36-62` — FindLatest function
- `Taskfile.yml:1-100` — Build, test, lint commands

## Architecture Documentation

**Package Layering**: `cli` → `resolve` + `validate` → `fetch` + `pinfile` + `manifest` + `semver` + `integrity` + `skill` + `install`

**Key Design Patterns**:
- Interface-based fetcher for testability (GitFetcher interface + GoGitFetcher + MockFetcher)
- Atomic file writes for manifest and pinfile
- 6-phase resolution pipeline with clear separation of concerns
- Table-driven tests with comprehensive fixture data
- Deterministic YAML output via custom yaml.Node construction

**Ref Resolution Flow**: `depurl.Parse → PackageIdentity() → NormalizeCloneURL() → fetcher.ResolveRef(url, ref) → commitSHA`

**MVS Flow**: `collectDeps → group by PackageIdentity → semver.Compare → select highest → re-collect if changed → resolveOne for each selected`

## Open Questions

None — all research objectives addressed with file:line evidence.
