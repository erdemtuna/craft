# Non-Tagged Repository Dependencies Implementation Plan

## Overview

Extend craft's dependency system to support repositories without semver tags. Users will be able to reference dependencies via commit SHA (`@<sha>`) or branch tracking (`@branch:<name>`) in addition to existing semver tags (`@vX.Y.Z`). The core change introduces a `RefType` discriminator that flows through parsing, resolution, pinfile serialization, CLI commands, and validation — enabling ref-type-aware behavior at every layer while preserving existing tagged dependency workflows unchanged.

## Current State Analysis

The dependency pipeline is built around a single assumption: all refs are semver tags.

- **Parsing** (`internal/resolve/depurl.go:12`): Regex enforces `@vMAJOR.MINOR.PATCH` — no other ref format accepted.
- **Manifest validation** (`internal/manifest/validate.go:19`): Separate regex copy also enforces semver-only URLs.
- **Resolver** (`internal/resolve/resolver.go:80-167`): MVS phase groups by `PackageIdentity()` and selects highest version via `semver.Compare()`. No concept of non-comparable refs.
- **Fetcher** (`internal/fetch/gogit.go:42-79`): `ResolveRef` already tries tag → local branch → remote branch. No commit SHA lookup, but branch resolution works.
- **Pinfile** (`internal/pinfile/types.go:16-32`): `ResolvedEntry` has no ref-type metadata.
- **CLI**: `add.go` calls `parsed.GitTag()` to display version; `update.go` uses `ListTags` + `semver.FindLatest` for all deps.

Key positive: the fetcher already resolves branch names, and `PackageIdentity()` is ref-type-agnostic. The extension layers cleanly on top.

## Desired End State

- `ParseDepURL` accepts three ref formats: `@vX.Y.Z` (tag), `@<hex7+>` (commit), `@branch:<name>` (branch)
- Resolver routes by RefType: tag deps use MVS, non-tagged deps bypass version comparison, mixed ref-type conflicts produce errors
- Pinfile entries carry `ref_type` metadata (`tag`, `commit`, `branch`) with backward-compatible defaulting
- `craft add` auto-detects ref type, validates ref existence, displays non-tagged warnings
- `craft update` re-resolves branch deps, skips commit deps, uses existing MVS for tag deps
- `craft validate` warns on non-tagged dependencies
- All existing tagged-dependency tests continue to pass unchanged
- Full test coverage for new ref types at every layer

**Verification approach**: `task test` (go test -race ./...) + `task lint` + `task build` after each phase.

## What We're NOT Doing

- `craft init` wizard integration (tracked as GitHub issue #25)
- Automatic migration from non-tagged to tagged refs
- Default branch inference for URLs without any ref
- Support for arbitrary git refs (refs/notes/, refs/stash)
- Monorepo/subdirectory scoping changes
- Changes to the fetcher's `ResolveRef` implementation for basic branch support (already works)

## Phase Status
- [ ] **Phase 1: Foundation Types & Parsing** - Add RefType, extend DepURL, update parsing and manifest validation
- [ ] **Phase 2: Resolution Pipeline & Pinfile** - Ref-type-aware resolver routing, conflict detection, pinfile ref_type field
- [ ] **Phase 3: CLI Commands** - Non-tagged ref support in craft add and craft update
- [ ] **Phase 4: Validation Warnings** - Non-tagged dependency warnings in craft validate
- [ ] **Phase 5: Documentation** - Technical reference and README updates

## Phase Candidates
<!-- No unresolved candidates — all features mapped to phases -->

---

## Phase 1: Foundation Types & Parsing

### Changes Required:

- **`internal/resolve/depurl.go`**:
  - Add `RefType` type (`string` enum: `RefTypeTag`, `RefTypeCommit`, `RefTypeBranch`) as exported constants
  - Extend `DepURL` struct with `Ref string` (the raw ref value — tag name, SHA, or branch name) and `RefType RefType`
  - Replace single regex with ref-type-aware parsing in `ParseDepURL`: after splitting on `@`, detect ref type by pattern — `branch:` prefix → Branch, `v` + semver → Tag, hex string ≥7 chars → Commit, else error
  - Update `GitTag()` → rename to `GitRef()` returning the appropriate ref string for each type: tag → `"v" + version`, commit → SHA, branch → branch name
  - Update `WithVersion()` to only work for tag refs (return error or empty for non-tag types)
  - Add `RefString()` method returning the `@`-suffixed ref as it appears in URLs (e.g., `@v1.0.0`, `@abc1234`, `@branch:main`)
  - Update `String()` to reconstruct from parsed components when `Raw` is empty

- **`internal/manifest/validate.go`**:
  - Remove the duplicate `depURLPattern` regex (line 19)
  - Replace inline regex check in `Validate()` (lines 53-55) with a call to `resolve.ParseDepURL()` — single source of truth for URL validation

- **`internal/resolve/depurl_test.go`**:
  - Add table-driven tests for commit SHA refs (7-char, 12-char, 40-char hex strings)
  - Add tests for branch refs (`branch:main`, `branch:feature/foo`, `branch:deadbeef`)
  - Add tests for invalid refs (5-char hex, empty ref, bare non-hex strings without prefix)
  - Add tests for edge cases: branch name with slashes, branch name that is valid hex
  - Verify existing semver tag tests still pass unchanged

- **`internal/manifest/validate_test.go`** (if exists, otherwise manifest validation tests):
  - Add tests verifying non-tagged dep URLs pass manifest validation

### Success Criteria:

#### Automated Verification:
- [ ] Tests pass: `go test -race ./internal/resolve/... ./internal/manifest/...`
- [ ] Lint clean: `golangci-lint run ./internal/resolve/... ./internal/manifest/...`

#### Manual Verification:
- [ ] `ParseDepURL("github.com/acme/tools@v1.0.0")` returns RefType=Tag, Version="1.0.0"
- [ ] `ParseDepURL("github.com/acme/tools@abc1234def")` returns RefType=Commit, Ref=the SHA
- [ ] `ParseDepURL("github.com/acme/tools@branch:main")` returns RefType=Branch, Ref="main"
- [ ] `ParseDepURL("github.com/acme/tools")` returns an error (no ref)
- [ ] `ParseDepURL("github.com/acme/tools@abc")` returns an error (SHA too short)

---

## Phase 2: Resolution Pipeline & Pinfile

### Changes Required:

- **`internal/resolve/types.go`**:
  - Add `RefType RefType` field to `ResolvedDep` struct

- **`internal/pinfile/types.go`**:
  - Add `RefType string \`yaml:"ref_type,omitempty"\`` to `ResolvedEntry` struct

- **`internal/pinfile/write.go`**:
  - Add `ref_type` field output after `commit` and before `integrity` in the `Write` function's YAML node construction (only when non-empty)

- **`internal/fetch/gogit.go`**:
  - Add commit SHA resolution path in `ResolveRef`: before trying tag/branch, check if `ref` looks like a hex SHA and attempt to resolve it directly as a commit hash via `repo.CommitObject(plumbing.NewHash(ref))`. This handles both full and short SHAs (go-git resolves short hashes).

- **`internal/resolve/resolver.go`**:
  - **Phase 3 (MVS)** (`resolver.go:76-167`): Restructure to handle ref types:
    - Group all deps by `PackageIdentity()` as before
    - For each identity group, check ref-type consistency — if mixed ref types exist, return conflict error (FR-012)
    - For tag-only groups: existing MVS with `semver.Compare()` (unchanged)
    - For commit-only groups: all must have the same SHA (or error — same package, different commits is a conflict)
    - For branch-only groups: all must reference the same branch name (or error)
  - **collectDeps** (`resolver.go:213-275`): Update `parsed.GitTag()` calls to use the new `GitRef()` method. Set `RefType` on collected `ResolvedDep`.
  - **resolveOne** (`resolver.go:278-314`): Update `parsed.GitTag()` to `parsed.GitRef()`. Carry `RefType` through to the resolved dep.
  - **Phase 6 (Build pinfile)** (`resolver.go:194-209`): Set `RefType` on `pinfile.ResolvedEntry` from `ResolvedDep.RefType`

- **`internal/resolve/resolver_test.go`**:
  - Add tests for resolving commit SHA deps (direct and transitive)
  - Add tests for resolving branch deps
  - Add tests for mixed ref-type conflict detection (tag+branch, tag+commit, branch+commit for same package)
  - Add tests for same-package-same-branch (should succeed) and same-package-different-branch (should error)
  - Verify existing tag-based resolution tests pass unchanged

- **`internal/pinfile/write_test.go`**:
  - Add test for pinfile output including `ref_type` field
  - Verify backward compatibility: pinfile without `ref_type` parses correctly (defaults to empty/tag)

- **`internal/pinfile/parse_test.go`**:
  - Add test for parsing pinfile with `ref_type` field

- **`internal/fetch/fetcher_test.go`** or mock tests:
  - Add test for commit SHA resolution via MockFetcher

### Success Criteria:

#### Automated Verification:
- [ ] Tests pass: `go test -race ./internal/resolve/... ./internal/pinfile/... ./internal/fetch/...`
- [ ] Lint clean: `golangci-lint run ./internal/resolve/... ./internal/pinfile/... ./internal/fetch/...`

#### Manual Verification:
- [ ] Resolver correctly resolves a branch dep through collectDeps → resolveOne → pinfile
- [ ] Resolver correctly resolves a commit dep through the full pipeline
- [ ] Mixed ref-type conflict produces clear error message
- [ ] Pinfile YAML output includes `ref_type: branch` or `ref_type: commit` for non-tagged deps
- [ ] Existing pinfiles without `ref_type` parse and work correctly (backward compat)

---

## Phase 3: CLI Commands

### Changes Required:

- **`internal/cli/add.go`**:
  - Update `Long` description (line 22) and hint message (line 47) to include non-tagged ref formats
  - After `ParseDepURL`, add non-tagged dependency warning: if `RefType != Tag`, print yellow-text warning about weaker reproducibility guarantees
  - Update summary output (line 132): replace `parsed.GitTag()` with ref-type-appropriate display (e.g., `"commit: abc1234..."` or `"branch: main"`)
  - For commit refs: resolve short SHA to full SHA via fetcher before storing in manifest. Store the full URL with the original ref in craft.yaml (the pinfile stores the resolved full SHA)

- **`internal/cli/update.go`**:
  - After `ParseDepURL` (line 84), branch by ref type:
    - **Tag**: Existing behavior — `ListTags` → `FindLatest` → `WithVersion` (unchanged)
    - **Branch**: Re-resolve branch HEAD via `fetcher.ResolveRef(cloneURL, parsed.GitRef())`. Compare against existing pinfile commit. If different, mark as updated. No manifest change (URL stays `@branch:<name>`), only pinfile updates.
    - **Commit**: Skip silently — commit pins are deliberate freezes
  - Update progress/summary messages to reflect ref-type-aware update behavior
  - Handle edge case: `craft update` with only commit-pinned deps → clean no-op exit

- **`internal/cli/add_test.go`**:
  - Add tests for adding commit SHA deps (valid, invalid, short SHA)
  - Add tests for adding branch deps
  - Add tests verifying warning output for non-tagged deps
  - Add tests for error cases (nonexistent commit, nonexistent branch)

- **`internal/cli/update_test.go`**:
  - Add tests for updating branch deps (branch HEAD changed → pinfile updated)
  - Add tests for commit dep skip behavior
  - Add tests for mixed dep types (tag updated, branch re-resolved, commit skipped)
  - Add test for update with only commit deps (no-op)

### Success Criteria:

#### Automated Verification:
- [ ] Tests pass: `go test -race ./internal/cli/...`
- [ ] Lint clean: `golangci-lint run ./internal/cli/...`

#### Manual Verification:
- [ ] `craft add skills github.com/acme/tools@branch:main` adds dep and prints warning
- [ ] `craft add skills github.com/acme/tools@abc1234` adds dep and prints warning
- [ ] `craft update` re-resolves branch deps and skips commit deps
- [ ] `craft update` with only commit deps exits cleanly
- [ ] Existing tagged dep add/update workflows unchanged

---

## Phase 4: Validation Warnings

### Changes Required:

- **`internal/validate/runner.go`**:
  - Add `checkNonTaggedDeps(result *Result, m *manifest.Manifest)` method
  - Call it from `Run()` after `checkManifest` (when manifest is valid)
  - For each dependency URL: parse with `resolve.ParseDepURL`, check `RefType`
  - If `RefType == Commit`: append `Warning{Message: "dependency \"<alias>\" uses a commit pin (<url>) — reproducible but frozen; no updates available"}`
  - If `RefType == Branch`: append `Warning{Message: "dependency \"<alias>\" tracks a branch (<url>) — weaker reproducibility guarantees than tagged versions"}`

- **`internal/cli/validate.go`** (or wherever validate output is formatted):
  - Verify warnings are displayed in yellow text (check existing warning rendering pattern)

- **`internal/validate/runner_test.go`**:
  - Add test for project with branch dep → warning present
  - Add test for project with commit dep → warning present
  - Add test for project with only tag deps → no non-tagged warnings
  - Add test for mixed deps → appropriate warnings for each non-tagged dep

### Success Criteria:

#### Automated Verification:
- [ ] Tests pass: `go test -race ./internal/validate/...`
- [ ] Lint clean: `golangci-lint run ./internal/validate/...`

#### Manual Verification:
- [ ] `craft validate` on project with branch dep shows yellow warning
- [ ] `craft validate` on project with commit dep shows yellow warning
- [ ] `craft validate` on project with only tagged deps shows no non-tagged warnings
- [ ] Warnings are non-blocking (validation can still pass with warnings)

---

## Phase 5: Documentation

### Changes Required:

- **`.paw/work/non-tagged-deps/Docs.md`**: Technical reference capturing implementation details, usage patterns, and verification approach (load `paw-docs-guidance` for template)
- **`README.md`**: Update dependency URL format documentation to include commit SHA and branch ref syntax. Add examples for non-tagged deps.

### Success Criteria:
- [ ] Docs.md accurately describes the implementation
- [ ] README examples show all three ref formats
- [ ] Content is consistent with Spec.md and actual implementation

---

## References
- Issue: none
- Spec: `.paw/work/non-tagged-deps/Spec.md`
- Research: `.paw/work/non-tagged-deps/CodeResearch.md`
