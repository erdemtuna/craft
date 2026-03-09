# Flat Install Layout Implementation Plan

## Overview

Introduce flat directory naming for globally installed skills so AI agents can discover them. The composite key `github.com/org/repo/skill` becomes the flat directory name `github-com--org--repo--skill`. Project-scoped installs (`forge/`) remain nested. Display commands (`list -g`, `tree -g`) show original composite key format for readability.

## Current State Analysis

- `Install()` at [internal/install/installer.go:17](internal/install/installer.go#L17) takes composite keys and creates nested directories via `filepath.Join(target, skillName)`. All atomic staging, path traversal, and overwrite logic lives here.
- Global install calls at [internal/cli/install.go:124](internal/cli/install.go#L124), [internal/cli/get.go:241](internal/cli/get.go#L241), and [internal/cli/update.go:227](internal/cli/update.go#L227) all call `installlib.Install()`.
- Remove at [internal/cli/remove.go:141](internal/cli/remove.go#L141) constructs nested path `filepath.Join(tp, nsPrefix, skillName)` and calls `cleanEmptyParents` at line 161.
- Display: `list.go` detailed mode shows leaf skill names at line 87; `tree.go` via `ui/tree.go:65` shows leaf names. Neither distinguishes global vs project scope for display format.
- 11 existing tests in `installer_test.go`, all self-contained with `t.TempDir()`.

## Desired End State

- Global installs produce flat directories as immediate children of the agent skills root
- Project installs produce nested directories under `forge/` (unchanged)
- Global removes delete flat directories without parent cleanup
- `list -g --detailed` and `tree -g` show composite key format for skills
- All existing tests pass; new tests cover `FlatKey()` and `InstallFlat()`
- Documentation updated with flat layout examples

## What We're NOT Doing

- Changing `Install()` function behavior
- Migrating pre-existing global installs (clean-slate)
- Changing pinfile format or resolution logic
- Modifying project-scoped install/remove/display behavior
- Adding reverse `FlatKey()` → composite key conversion (pinfile is source of truth)

## Phase Status
- [ ] **Phase 1: Core installer** - Add `FlatKey()` + `InstallFlat()` with tests
- [ ] **Phase 2: CLI wiring** - Route global install/get/update through `InstallFlat()`
- [ ] **Phase 3: Remove cleanup** - Use `FlatKey()` for global removes
- [ ] **Phase 4: Display commands** - Show composite keys in `list -g`/`tree -g`
- [ ] **Phase 5: Documentation** - Update README.md, E2E doc, and Docs.md

## Phase Candidates
<!-- None currently -->

---

## Phase 1: Core Installer

Add `FlatKey()` and `InstallFlat()` to the install package, plus comprehensive tests.

### Changes Required

- **`internal/install/installer.go`**: Add two exported functions after the existing `Install()` (after line 88):
  - `FlatKey(compositeKey string) string` — Pure function. Replace `.` → `-`, then `/` → `--`. Return the result. Casing preserved.
  - `InstallFlat(target string, skills map[string]map[string][]byte) error` — Build a new skill map with `FlatKey(k)` as key for each entry `k`, then delegate to `Install()`. This reuses all staging, validation, and atomicity logic.

- **`internal/install/installer_test.go`**: Add new tests after existing tests (after line 210):
  - `TestFlatKey` — Table-driven: standard key, dots in all segments, dashes in skill name, mixed casing, single-segment edge
  - `TestFlatKeyEdgeCases` — Dots in non-host segments, keys with many components, empty-ish edge behavior
  - `TestInstallFlatCreatesStructure` — Basic flat install; verify directories are direct children of target
  - `TestInstallFlatMultiPackage` — Two skills from different repos; verify both flat, no nesting
  - `TestInstallFlatSameName` — Two skills with same leaf name from different repos produce different flat directories (collision safety)
  - `TestInstallFlatOverwrites` — Second flat install replaces content atomically

### Success Criteria

#### Automated Verification:
- [ ] Tests pass: `go test ./internal/install/ -v -run "TestFlatKey|TestInstallFlat"`
- [ ] Full suite: `go test ./...`
- [ ] Vet: `go vet ./...`

#### Manual Verification:
- [ ] `FlatKey("github.com/org/repo/skill")` returns `"github-com--org--repo--skill"`
- [ ] `InstallFlat()` creates flat directories (not nested) in target

---

## Phase 2: CLI Wiring

Route global installs through `InstallFlat()` in all three commands.

### Changes Required

- **`internal/cli/install.go`**: At [line 124](internal/cli/install.go#L124), change `installlib.Install(targetPath, skillFiles)` to `installlib.InstallFlat(targetPath, skillFiles)`. Project path at [line 223](internal/cli/install.go#L223) remains `Install()`.

- **`internal/cli/get.go`**: At [line 241](internal/cli/get.go#L241), change `installlib.Install(targetPath, skillFiles)` to `installlib.InstallFlat(targetPath, skillFiles)`. `get` is always global scope.

- **`internal/cli/update.go`**: At [line 227](internal/cli/update.go#L227), change `installlib.Install(targetPath, skillFiles)` to `installlib.InstallFlat(targetPath, skillFiles)`. Project path at [line 251](internal/cli/update.go#L251) remains `Install()`.

### Success Criteria

#### Automated Verification:
- [ ] Build: `go build ./...`
- [ ] Vet: `go vet ./...`
- [ ] Tests: `go test ./...`

#### Manual Verification:
- [ ] `craft get github.com/anthropics/courses@branch:main --target /tmp/test-skills` creates flat directories under `/tmp/test-skills/`
- [ ] `craft install` in a project still creates nested `forge/` structure

---

## Phase 3: Remove Cleanup

Use `FlatKey()` for global removes, skip parent cleanup.

### Changes Required

- **`internal/cli/remove.go`**: 
  - Add import for `installlib "github.com/erdemtuna/craft/internal/install"`
  - At [line 141](internal/cli/remove.go#L141): branch on `globalFlag`:
    - Global: `skillDir := filepath.Join(tp, installlib.FlatKey(nsPrefix+"/"+skillName))`
    - Project: keep existing `filepath.Join(tp, nsPrefix, skillName)`
  - At [line 161](internal/cli/remove.go#L161): only call `cleanEmptyParents` for project scope (wrap in `if !globalFlag`)

### Success Criteria

#### Automated Verification:
- [ ] Build: `go build ./...`
- [ ] Vet: `go vet ./...`
- [ ] Tests: `go test ./...`

#### Manual Verification:
- [ ] After `craft remove -g alias --target /tmp/test-skills`, flat skill directory is gone
- [ ] No empty parent directories left behind
- [ ] Project `craft remove alias` still cleans nested dirs + empty parents

---

## Phase 4: Display Commands

Show composite key format in `list -g --detailed` and `tree -g`.

### Changes Required

- **`internal/cli/list.go`**: At [line 87](internal/cli/list.go#L87), in the detailed display branch: when `globalFlag` is true, prefix each skill name with `d.url + "/"` to form composite keys. When project scope, display leaf names as-is.

- **`internal/ui/tree.go`**: At [line 65](internal/ui/tree.go#L65), the `RenderTree` function currently shows raw skill names. Add a `ShowCompositeKeys bool` field to `DepNode` struct (or add a parameter). When true, prefix each skill with the package identity from `dep.URL` (extracted before the `@` version). Wire this from `tree.go` CLI command based on `globalFlag`.

  Alternative simpler approach: the CLI command in `internal/cli/tree.go` can transform `dep.Skills` to composite key format before passing to `RenderTree`, avoiding changes to the ui package interface.

### Success Criteria

#### Automated Verification:
- [ ] Build: `go build ./...`
- [ ] Vet: `go vet ./...`
- [ ] Tests: `go test ./...`

#### Manual Verification:
- [ ] `craft list -g --detailed` shows skills as `github.com/org/repo/skill-name`
- [ ] `craft tree -g` shows composite key skill names under each dep
- [ ] `craft list --detailed` (project) shows leaf skill names as before
- [ ] `craft tree` (project) shows leaf skill names as before

---

## Phase 5: Documentation

Update documentation to reflect flat install layout for global installs.

### Changes Required

- **`.paw/work/flat-install-layout/Docs.md`**: Technical reference covering `FlatKey()` format, `InstallFlat()` delegation pattern, CLI routing, and remove cleanup. Load `paw-docs-guidance` for template.

- **`README.md`**: Update the global install directory structure examples to show flat layout. Update agent auto-detection table if it shows nested paths. Keep project/forge examples unchanged.

- **`E2E_REAL_WORLD_TEST.md`**: Update Part 9 (Consumer/global install) expected directory structures from nested to flat format. Update any verification steps that check for nested global directories.

### Success Criteria

#### Automated Verification:
- [ ] No build/test regressions: `go test ./...`

#### Manual Verification:
- [ ] README global install example shows flat directory names
- [ ] E2E doc Part 9 reflects flat layout expectations
- [ ] Docs.md captures implementation details accurately

---

## References
- Spec: `.paw/work/flat-install-layout/Spec.md`
- Research: `.paw/work/flat-install-layout/CodeResearch.md`
- WorkShaping: `.paw/work/flat-install-layout/WorkShaping.md`
