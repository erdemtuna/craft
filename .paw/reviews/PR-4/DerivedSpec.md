---
date: 2026-03-07 12:30:10 UTC
git_commit: ff2f2e50c57124b8673a30824de9267c1bf0cf4f
branch: ci/taskfile-and-quality
repository: erdemtuna/craft
topic: "Derived Specification for Replace Makefile with Taskfile, enhance CI/CD and code quality tooling"
tags: [review, specification, analysis]
status: complete
---

# Derived Specification: Replace Makefile with Taskfile, enhance CI/CD and code quality tooling

## Intent Summary

This PR replaces a minimal 13-line Makefile with a comprehensive Taskfile.yml (go-task), restructures the CI pipeline from a single sequential job into three parallel jobs (quality, test, build) with expanded quality gates (format checking, vulnerability scanning, cross-compilation with artifact uploads), and adds developer-experience infrastructure (`.editorconfig`, git hooks, `CONTRIBUTING.md`).

## Explicit Goals (Stated in PR/Issues)

Goals explicitly mentioned in the PR description and commit messages:

1. **Replace Makefile with Taskfile** — Swap the 5-target Makefile for a Taskfile.yml with expanded targets: `build`, `test`, `test:coverage`, `fmt`, `fmt:check`, `vet`, `lint`, `vuln`, `ci`, `install`, `hooks:install`, `hooks:uninstall`, `clean`.
2. **Split CI into 3 parallel jobs** — Restructure `ci.yml` from a single `test` job into `quality`, `test`, and `build` jobs, with `build` depending on the other two.
3. **Add `.golangci-lint.yml` with curated linter set** — Introduce an explicit linter configuration expanding beyond defaults with `gocritic`, `gofmt`, `misspell`, `prealloc`, `unconvert`.
4. **Add `.editorconfig`** — Standardize editor settings (charset, line endings, indentation per file type).
5. **Add git hooks** (`pre-commit`, `pre-push`) — Local quality gates running format/vet checks before commit and lint/test checks before push.
6. **Add `CONTRIBUTING.md`** — Comprehensive developer guide covering setup, code quality, task reference, hooks, testing, and CI/CD.
7. **Update `.gitignore`** — Add `coverage.out`, `coverage.html`, `.task/` entries.
8. **Update `README.md` Development section** — Add a brief Development section pointing to `CONTRIBUTING.md` with a `task ci` quick-start.

*Source: PR description, commits `473fa53`, `f14535a`, `ff2f2e5`*

## Inferred Goals (Observed from Code)

Goals derived from code analysis that weren't explicitly stated in the PR description:

1. **Version injection in local and CI builds** — `Taskfile.yml:6-7` introduces `VERSION` via `git describe --tags --always --dirty` and applies it through `LDFLAGS`. The CI build job (`ci.yml:62-66`) also injects version via ldflags. Previously, version injection only occurred in goreleaser release builds; local `make build` and CI both produced `Version = "dev"`.
2. **Unify Go version sourcing to `go-version-file: go.mod`** — All three CI jobs now use `go-version-file: go.mod` (`ci.yml:14`, `ci.yml:37`, `ci.yml:53`) instead of the previous hardcoded matrix `go-version: ['1.24']`. This aligns CI with the existing `release.yml:18` pattern.
3. **Cross-compilation in CI** — The new `build` job (`ci.yml:43-73`) cross-compiles for 6 platform combinations (linux/darwin/windows × amd64/arm64) with artifact uploads. Previously CI only built for the runner's native platform and did not upload artifacts.
4. **Coverage report generation** — The `test:coverage` Taskfile target (`Taskfile.yml:30-35`) generates both `coverage.out` and `coverage.html`. The CI test job (`ci.yml:39-44`) generates and uploads `coverage.out` as an artifact.
5. **Security scanning with govulncheck** — New quality gate in CI (`ci.yml:25-28`) and local via `task vuln` (`Taskfile.yml:56-58`). This check did not exist before.
6. **Format checking enforcement** — New `fmt:check` task (`Taskfile.yml:44-46`) and CI quality job step (`ci.yml:17`) enforce `gofmt` compliance. Previously, formatting was neither checked nor enforced anywhere.
7. **Hooks management via Taskfile** — `hooks:install` (`Taskfile.yml:74-77`) sets `core.hooksPath` to `.githooks/`; `hooks:uninstall` (`Taskfile.yml:79-82`) reverses it. This is a new developer workflow pattern.

*Source: Code analysis of `Taskfile.yml`, `.github/workflows/ci.yml`, `.githooks/`*

## Baseline Behavior (Pre-Change)

How the system worked before changes (from CodeResearch.md):

**Module**: `Makefile`
- **Before**: 13-line Makefile with 5 `.PHONY` targets (`build`, `test`, `vet`, `lint`, `ci`). Ran bare Go commands with no variables, flags, or version injection. `ci` target chained `vet lint test build` sequentially.
- **Integration**: Not referenced by any other file — CI and Makefile ran identical commands independently.
- **Patterns**: Simple `.PHONY` declarations, one command per target.

**Module**: `.github/workflows/ci.yml`
- **Before**: Single job `test` on `ubuntu-latest` with Go version hardcoded via matrix `['1.24']`. Four sequential steps: `go vet` → `golangci-lint-action@v7` → `go test -race` → `go build`. No artifact uploads, no coverage, no format check, no vulnerability scan.
- **Integration**: Triggered on push/PR to `main`. Independent from Makefile and `release.yml`.
- **Patterns**: Single-entry matrix strategy, action versions pinned to major (`@v4`, `@v5`, `@v7`).

**Module**: `.gitignore`
- **Before**: 4 entries: `*.exe`, `craft`, `dist/`, `*.out`.

**Module**: `README.md`
- **Before**: Comprehensive project docs but no Development/Contributing section. Build-from-source used `go build` directly.

**Module**: Linting configuration
- **Before**: No `.golangci.yml` or variants. CI used `golangci-lint-action@v7` with default linter set (`errcheck`, `gosimple`, `govet`, `ineffassign`, `staticcheck`, `unused`).

**Module**: Editor/Hook configuration
- **Before**: No `.editorconfig`, no `.githooks/` directory, no `core.hooksPath` configuration.

**Module**: `CONTRIBUTING.md`
- **Before**: Did not exist.

*Source: CodeResearch.md at base commit cefd4b68480a8041baa5da0f91bc0dca2ae25d93*

## Observable Changes (Before → After)

### Changed Interfaces

| Component | Before | After | Breaking? |
|-----------|--------|-------|-----------|
| Build tool | `make build` | `task build` | Yes (user-facing) |
| CI pipeline | 1 job, 4 sequential steps | 3 parallel jobs (quality, test, build) | No (internal) |
| Go version in CI | Hardcoded `'1.24'` matrix | `go-version-file: go.mod` | No |
| Lint config | Default linter set (6 linters) | Explicit config with 11 linters | No |
| Binary output | `./craft` (native only) | `./craft` local; `craft_<os>_<arch>` in CI | No |
| Version in local builds | Always `"dev"` | `git describe` output (e.g., `v1.0.0-3-gabcdef`) | No |

### Changed Behavior

**Feature**: Build system
- **Before**: `make build` ran `go build ./cmd/craft` producing `./craft` with `Version = "dev"`. No version injection.
- **After**: `task build` runs `go build -ldflags "-s -w -X ...Version=<git-describe>"` producing `./craft` with actual version from git tags. Binary is also stripped (`-s -w`).
- **Impact**: Local builds now carry meaningful version info. Binary size reduced due to symbol stripping.

[`Taskfile.yml:19-20`, `Taskfile.yml:6-7`]

**Feature**: CI pipeline structure
- **Before**: Single `test` job ran vet → lint → test → build sequentially (~sum of all step durations).
- **After**: `quality` and `test` run in parallel; `build` runs after both pass (`ci.yml:45`). Quality adds format checking and govulncheck. Test adds coverage upload. Build cross-compiles for 6 platforms with artifact uploads.
- **Impact**: Faster CI feedback (parallel execution), broader quality gates, downloadable binaries from every CI run.

[`.github/workflows/ci.yml:9-73`]

**Feature**: Quality gates
- **Before**: `go vet` + `golangci-lint` (default 6 linters). No format check, no vulnerability scan.
- **After**: `gofmt` check + `go vet` + `golangci-lint` (11 linters) + `govulncheck`. Format checking and vuln scanning are new enforcement points.
- **Impact**: PRs will be blocked by formatting issues or known vulnerabilities that previously passed.

[`.github/workflows/ci.yml:17-28`, `.golangci-lint.yml:1-22`]

**Feature**: Linter configuration
- **Before**: No config file. Default set: `errcheck`, `gosimple`, `govet`, `ineffassign`, `staticcheck`, `unused`.
- **After**: `.golangci-lint.yml` enables the 6 defaults plus `gocritic`, `gofmt`, `misspell`, `prealloc`, `unconvert`. Adds 5m timeout. Excludes `testdata/` directory.
- **Impact**: 5 additional linters may flag issues in existing code. `testdata/` exclusion prevents false positives from fixture files.

[`.golangci-lint.yml:1-22`]

**Feature**: Developer experience — git hooks
- **Before**: No git hooks.
- **After**: `pre-commit` runs `gofmt -l` and `go vet` (~1-2s). `pre-push` runs `golangci-lint` and `go test` (~10-30s). Installed via `task hooks:install` setting `core.hooksPath`.
- **Impact**: Developers catch formatting and quality issues before reaching CI. Hooks are opt-in (require explicit install).

[`.githooks/pre-commit:1-17`, `.githooks/pre-push:1-10`, `Taskfile.yml:74-82`]

**Feature**: Developer experience — documentation
- **Before**: No `CONTRIBUTING.md`. No Development section in README.
- **After**: 193-line `CONTRIBUTING.md` covering setup, code quality, task reference, git hooks, testing patterns, CI/CD pipeline, and PR guidelines. README gains a Development section pointing to `CONTRIBUTING.md` with `task ci` quick-start.
- **Impact**: New contributors have a comprehensive onboarding guide.

[`CONTRIBUTING.md:1-193`, `README.md:229-238`]

## Scope Boundaries

**In Scope**:
- Build tool replacement (Makefile → Taskfile.yml)
- CI pipeline restructuring and expansion
- Linter configuration
- Editor configuration (`.editorconfig`)
- Git hooks infrastructure
- Developer documentation (`CONTRIBUTING.md`, README update)
- `.gitignore` updates for new artifacts

**Out of Scope**:
- Release pipeline (`release.yml` — unchanged)
- Application source code (no changes to `cmd/`, `internal/`)
- Go module dependencies (`go.mod`, `go.sum` — unchanged)
- `.goreleaser.yml` configuration
- Test code or test fixtures
- `CODE_OF_CONDUCT.md`

## Assumptions

1. **Taskfile adoption is intentional**: The Makefile is fully removed, not kept as a fallback. Users must install `go-task` to use the new build targets.
2. **`.golangci-lint.yml` filename is intentional**: The config file uses `.golangci-lint.yml` (with hyphen before `lint`) rather than the more conventional `.golangci.yml`. golangci-lint recognizes both naming patterns.
3. **`coverage.out` redundancy in `.gitignore` is intentional**: The existing `*.out` glob already covers `coverage.out`, but the explicit entry may serve as documentation of expected artifacts.
4. **`.editorconfig` Makefile rule is future-proofing**: The `.editorconfig` includes a `[Makefile]` section despite the Makefile being deleted in this PR, likely as a general-purpose rule for any future Makefiles.

## Open Questions

*None — all goals are clear from code analysis and PR description.*

## Discrepancies Flagged

**1. `.editorconfig` contains `[Makefile]` section but Makefile is deleted**
- **PR Description States**: Replaces Makefile with Taskfile
- **Code Analysis Shows**: `.editorconfig:16-17` includes `[Makefile]` indent rule, but `Makefile` is deleted in this PR
- **Resolution**: Resolved — this is a standard `.editorconfig` convention for any future Makefiles. Not a functional issue.

**2. `.gitignore` adds `coverage.out` explicitly despite existing `*.out` glob**
- **Code Analysis Shows**: `.gitignore:4` has `*.out` which already matches `coverage.out`. Line 5 (new) adds explicit `coverage.out`.
- **Resolution**: Resolved — redundant but harmless. The explicit entry serves as documentation.

**3. Pre-push hook runs `go test ./...` without `-race` flag**
- **Code Analysis Shows**: `.githooks/pre-push:8` runs `go test ./...` without `-race`, while CI (`ci.yml:39`) and `task test` (`Taskfile.yml:26`) both use `-race`.
- **Resolution**: Noted as potential inconsistency — likely intentional for faster pre-push checks, but worth reviewer consideration.

## References

- **ReviewContext.md**: PR metadata, changed files summary, review configuration
- **CodeResearch.md**: Pre-change baseline at commit `cefd4b68`
- **Commits**: `cefd4b68`..`ff2f2e50` (3 commits)
- **Changed Files**: 10 files, +433 -23
