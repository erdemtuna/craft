---
date: 2026-03-07 12:25:36 UTC
git_commit: ff2f2e50c57124b8673a30824de9267c1bf0cf4f
branch: ci/taskfile-and-quality
repository: erdemtuna/craft
topic: "Research Questions for PR #4: Replace Makefile with Taskfile"
tags: [review, research, baseline]
status: complete
base_commit: cefd4b68480a8041baa5da0f91bc0dca2ae25d93
---

# Research Questions for PR #4

Investigate the codebase at base commit `cefd4b6` to establish baseline understanding for reviewing PR #4.

---

## 1. Makefile (Deleted)

**File**: `Makefile`

### Questions

1. **What targets does the existing Makefile define, and what commands does each run?**
   - Investigation: Read `Makefile` at base commit to document all `.PHONY` targets and their recipes.
   - Purpose: Verify the new Taskfile.yml is a functional superset (no targets lost).

2. **Are there any other files or scripts that reference `make` or `Makefile`?**
   - Investigation: Search codebase for `make `, `make\t`, `Makefile`, `$(MAKE)` references in README.md, CI workflows, scripts, and documentation.
   - Purpose: Identify any references that must be updated to `task` after the Makefile removal.

---

## 2. CI Workflow (`.github/workflows/ci.yml`)

**File**: `.github/workflows/ci.yml`

### Questions

3. **What is the current structure of the CI workflow (jobs, steps, triggers)?**
   - Investigation: Read `.github/workflows/ci.yml` at base commit. Document job names, step order, Go version strategy, and trigger events.
   - Purpose: Understand what the single-job CI did to evaluate correctness of the new 3-job split.

4. **How is the Go version specified in the current CI (matrix vs go-version-file)?**
   - Investigation: Check if the workflow uses `go-version`, `go-version-file`, or a matrix strategy.
   - Purpose: The new workflow switches to `go-version-file: go.mod`. Confirm this is a valid approach and no version pinning is lost.

5. **Does the current CI produce any artifacts (binaries, coverage reports)?**
   - Investigation: Search for `actions/upload-artifact` or any artifact-related steps in the base workflow.
   - Purpose: The new CI adds coverage and binary artifact uploads. Understand if this is entirely new functionality.

6. **What checks does the current CI run (vet, lint, test, build)?**
   - Investigation: Document the exact commands run in CI steps.
   - Purpose: Verify the new CI covers all existing checks and identify any gaps or additions.

---

## 3. `.gitignore`

**File**: `.gitignore`

### Questions

7. **What entries are in the current `.gitignore`?**
   - Investigation: Read `.gitignore` at base commit.
   - Purpose: Verify the new entries (`coverage.out`, `coverage.html`, `.task/`) are additive and no existing entries were removed.

---

## 4. `README.md`

**File**: `README.md`

### Questions

8. **What does the current Development section in README.md contain?**
   - Investigation: Read `README.md` at base commit, specifically looking for any "Development" or "Contributing" sections.
   - Purpose: Understand what content existed before the PR's changes to evaluate whether the new section is an improvement and whether any important information was lost.

9. **Are there any references to `make` or `Makefile` in the README?**
   - Investigation: Search README.md for `make`, `Makefile` references.
   - Purpose: Ensure migration to `task` is reflected consistently.

---

## 5. Go Module & Build Configuration

**File**: `go.mod`, `cmd/craft/main.go`

### Questions

10. **What Go version is specified in `go.mod`?**
    - Investigation: Read `go.mod` to find the `go` directive version.
    - Purpose: The new CI uses `go-version-file: go.mod` — confirm the version in go.mod is appropriate.

11. **Does the project currently use ldflags or version injection in any build commands?**
    - Investigation: Search for `-ldflags`, `version.Version`, or `internal/version` references in Makefile, CI, scripts, or Go source.
    - Purpose: The new Taskfile and CI inject version via `-ldflags "-s -w -X github.com/erdemtuna/craft/internal/version.Version=..."`. Determine if this is new or an existing pattern.

12. **Does `internal/version` package exist, and what does it expose?**
    - Investigation: Check for existence of `internal/version/` directory and read any files in it.
    - Purpose: The ldflags target `internal/version.Version` — confirm this variable exists and is used.

---

## 6. Linting Configuration

**File**: `.golangci-lint.yml` (new)

### Questions

13. **Is there an existing `.golangci.yml` or `.golangci-lint.yml` configuration file?**
    - Investigation: Search for `golangci` config files at base commit (`.golangci.yml`, `.golangci.yaml`, `.golangci-lint.yml`).
    - Purpose: Determine if the new config file replaces an existing one or is entirely new.

14. **What linters does the current `golangci-lint run` invocation use (default set)?**
    - Investigation: If no config exists, document golangci-lint's default linter set for comparison.
    - Purpose: Understand if the curated linter list in the new config is expanding, restricting, or matching the previous effective configuration.

---

## 7. Editor & Hook Configuration

**Files**: `.editorconfig` (new), `.githooks/pre-commit` (new), `.githooks/pre-push` (new)

### Questions

15. **Does the project currently have an `.editorconfig` file?**
    - Investigation: Check for `.editorconfig` at base commit.
    - Purpose: Confirm this is entirely new configuration.

16. **Does the project have any existing git hooks or hook configuration?**
    - Investigation: Check for `.githooks/`, `.husky/`, or `core.hooksPath` configuration at base commit.
    - Purpose: Confirm hooks are entirely new, not replacing existing ones.

---

## 8. Cross-Cutting Concerns

### Questions

17. **Are there any other CI/CD workflow files besides `ci.yml`?**
    - Investigation: List files in `.github/workflows/` at base commit.
    - Purpose: Understand if the release workflow mentioned in CONTRIBUTING.md (`release.yml`) already exists or is referenced.

18. **Does `CONTRIBUTING.md` exist at the base commit?**
    - Investigation: Check for existence of `CONTRIBUTING.md`.
    - Purpose: Confirm this is a new file, not a modification.

---

## Files to Investigate at Base Commit `cefd4b6`

- `Makefile`
- `.github/workflows/ci.yml`
- `.gitignore`
- `README.md`
- `go.mod`
- `cmd/craft/main.go`
- `internal/version/` (directory listing + files)
- `.golangci.yml` / `.golangci-lint.yml` / `.golangci.yaml` (check existence)
- `.editorconfig` (check existence)
- `.githooks/` (check existence)
- `.github/workflows/` (directory listing)
- `CONTRIBUTING.md` (check existence)
