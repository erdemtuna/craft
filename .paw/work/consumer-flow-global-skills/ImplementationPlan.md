# Consumer Flow & Global Skills Management — Implementation Plan

## Overview

Implement a consumer-friendly skill installation flow for craft. This adds `craft get` as a one-command entry point for consumers, introduces global skill state at `~/.craft/`, adds a `--global/-g` flag to existing commands, and changes `craft install` to vendor dependencies into a project-local `forge/` directory instead of writing to agent directories.

## Current State Analysis

- All commands assume project-scoped manifest at `./craft.yaml` — no global concept exists
- `craft install` writes directly to agent directories (`~/.claude/skills/`, `~/.copilot/skills/`)
- `manifest.Validate()` requires non-empty `Skills` slice (line 37 of `validate.go`) — blocks global manifests
- `~/.craft/cache/` already exists for git cache — global state directory partially in place
- Agent detection, resolution, integrity verification, and atomic installation are all reusable
- `--verbose/-v` is the only persistent flag — model for adding `--global/-g`
- `writeManifestAtomic()` and `writePinfileAtomic()` provide atomic write infrastructure

## Desired End State

- Consumers can run `craft get github.com/org/repo@v1.0.0` with no prior setup and have skills installed to their agent
- Global state tracked at `~/.craft/craft.yaml` + `~/.craft/craft.pin.yaml`, managed via `-g` flag on existing commands
- `craft install` vendors to `forge/` (gitignored), never writes to agent directories
- `craft add --install` triggers forge vendoring
- All existing commands (`list`, `update`, `remove`, `install`, `tree`, `validate`, `outdated`) work with `-g` flag on global state
- `craft remove -g` uninstalls skill files from agent directories
- All changes pass existing tests + new tests, lint, and vet

## What We're NOT Doing

- No changes to `craft init` (remains author-only)
- No central registry or search/browse functionality
- No implicit "latest tag" resolution — version always required
- No persisting agent choice across invocations
- No monorepo/subpath support
- No `craft get` operating on project manifests (always global)

## Phase Status
- [ ] **Phase 1: Global Infrastructure** — Global flag, path helpers, validation relaxation
- [ ] **Phase 2: Forge Vendoring** — Change `craft install` and `craft add --install` to vendor to `forge/`
- [ ] **Phase 3: craft get Command** — New consumer entry point
- [ ] **Phase 4: Global Flag on Existing Commands** — `-g` support on list, update, remove, install, tree, validate, outdated
- [ ] **Phase 5: Documentation** — README updates, Docs.md

## Phase Candidates
<!-- None — all phases defined -->

---

## Phase 1: Global Infrastructure

### Changes Required:

- **`internal/cli/global.go`** (new file): Define `globalFlag` bool variable and `GlobalCraftDir()` helper returning `~/.craft/`. Add `GlobalManifestPath()` and `GlobalPinfilePath()` helpers. Register `--global/-g` as `PersistentFlags` on `rootCmd` (following `verbose.go` pattern).

- **`internal/cli/root.go`**: No structural changes needed — flag registration happens in `global.go`'s `init()` via `rootCmd.PersistentFlags().BoolVarP()`.

- **`internal/cli/helpers.go`**: Add `loadManifestForScope()` and `loadManifestAndPinfileForScope()` helpers. When `globalFlag` is true, resolve paths from `GlobalCraftDir()` instead of cwd. When global manifest doesn't exist, return a descriptive error suggesting `craft get`.

- **`internal/manifest/validate.go`**: Add `ValidateGlobal(m *Manifest) []error` that skips the Skills non-empty check (line 37-39). Alternatively, add an `Options` struct to `Validate()` with a `AllowEmptySkills` bool. The existing `Validate()` signature and behavior must remain unchanged for project manifests.

- **`internal/cli/global_test.go`** (new file): Test `GlobalCraftDir()` path construction, `GlobalManifestPath()`, `GlobalPinfilePath()`. Test `loadManifestForScope()` returns global or project paths based on flag. Test validation allows empty Skills for global manifests.

### Success Criteria:

#### Automated Verification:
- [ ] Tests pass: `task test`
- [ ] Lint/typecheck: `task lint && task vet`

#### Manual Verification:
- [ ] `--global` and `-g` flags appear in help output for all commands
- [ ] `GlobalCraftDir()` correctly resolves to `$HOME/.craft/`

---

## Phase 2: Forge Vendoring

### Changes Required:

- **`internal/cli/install.go`**: Modify `runInstall()` to branch on `globalFlag`:
  - When `!globalFlag` (project scope): Replace `resolveInstallTargets()` call with forge directory path (`filepath.Join(cwd, "forge")`). Reuse existing `collectSkillFiles()` → `verifyIntegrity()` → `installlib.Install(forgePath, skills)` pipeline. Add `.gitignore` auto-update logic after successful install.
  - When `globalFlag`: Keep current behavior (resolve targets to agent dirs) — this becomes `craft install -g`.

- **`internal/cli/install.go`**: Add `ensureGitignore(root, entry)` function. Opens `.gitignore`, checks if `forge/` entry exists, appends if not. Creates `.gitignore` if missing. Uses `os.OpenFile` with append mode.

- **`internal/cli/add.go`**: Modify `--install` flow (lines 161-185) to use forge path instead of `resolveInstallTargets()` when `!globalFlag`. Same change: install to `filepath.Join(cwd, "forge")`.

- **`internal/cli/install_test.go`**: Add tests for forge vendoring: verify files land in `forge/`, verify `.gitignore` is updated, verify no agent directory writes in project scope. Test `install -g` still writes to agent directories.

### Success Criteria:

#### Automated Verification:
- [ ] Tests pass: `task test`
- [ ] Lint/typecheck: `task lint && task vet`

#### Manual Verification:
- [ ] `craft install` in a project creates `forge/` with vendored skill files
- [ ] `forge/` appears in `.gitignore` after first install
- [ ] No files written to `~/.claude/skills/` or `~/.copilot/skills/` during project install

---

## Phase 3: craft get Command

### Changes Required:

- **`internal/cli/get.go`** (new file): New cobra command `get [alias] <url> [url...]`. Accepts 1+ positional args (parsed as optional alias + URLs). Flags: `--dry-run`, `--target`. Core flow:
  1. Parse each URL via `resolve.ParseDepURL()`
  2. Load or create global manifest from `GlobalManifestPath()`
  3. For each dep: check if already installed, prompt if version differs
  4. Add all deps to global manifest
  5. Resolve full tree via `resolver.Resolve()`
  6. If `--dry-run`: print summary, exit
  7. Write global manifest and pinfile atomically
  8. Resolve agent install targets (reuse `resolveInstallTargets()`)
  9. Collect files, verify integrity, install to agent dirs

- **`internal/cli/get.go`**: Auto-create global manifest function. When `~/.craft/craft.yaml` doesn't exist, create with `schema_version: 1`, `name: global`, empty skills, and the new dependency. Ensure `~/.craft/` directory exists via `os.MkdirAll`.

- **`internal/cli/get.go`**: Prompt-on-existing logic. When dependency alias already exists with different URL/version, prompt user via stdin (if TTY) whether to update. If not TTY, error.

- **`internal/cli/root.go`**: Register `getCmd` in `init()`.

- **`internal/cli/get_test.go`** (new file): Tests for: single URL install, multiple URLs, alias derivation, alias override, already-installed prompt, dry-run mode, missing ref error, no agent error, global manifest auto-creation.

### Success Criteria:

#### Automated Verification:
- [ ] Tests pass: `task test`
- [ ] Lint/typecheck: `task lint && task vet`

#### Manual Verification:
- [ ] `craft get github.com/org/repo@v1.0.0` creates `~/.craft/craft.yaml`, `~/.craft/craft.pin.yaml`, and installs skills to agent directory
- [ ] Running `craft get` again with same URL reports "already installed"
- [ ] Running `craft get` with different version prompts to update
- [ ] `craft get --dry-run` shows preview without writing files
- [ ] `craft get url1 url2` installs both

---

## Phase 4: Global Flag on Existing Commands

### Changes Required:

- **`internal/cli/list.go`**: When `globalFlag`, use `loadManifestAndPinfileForScope()` to load from `~/.craft/`. Same output format. Add "No global skills installed" message if global manifest doesn't exist.

- **`internal/cli/tree.go`**: When `globalFlag`, load from `~/.craft/`. Use `name: global` as root. No local skills section (global manifests have no skills).

- **`internal/cli/validate.go`**: When `globalFlag`, validate global manifest with `ValidateGlobal()`. Check pinfile consistency against global state.

- **`internal/cli/outdated.go`**: When `globalFlag`, load from `~/.craft/`. Same logic for checking newer versions.

- **`internal/cli/update.go`**: When `globalFlag`, load from `~/.craft/`. After resolution, install to agent directories (not forge). Use `resolveInstallTargets()` for agent detection. Write updated global manifest and pinfile.

- **`internal/cli/remove.go`**: When `globalFlag`, load from `~/.craft/`. After removing from global manifest/pinfile, clean up skill files from agent directories. Reuse existing `cleanEmptyParents()` logic. Use `resolveInstallTargets()` or `--target` for cleanup paths.

- **`internal/cli/install.go`**: `install -g` path already handled in Phase 2 (when `globalFlag`, use agent targets instead of forge).

- **Tests across `*_test.go`**: Add `-g` flag tests for each command. Test global manifest loading, error on missing global state, correct output.

### Success Criteria:

#### Automated Verification:
- [ ] Tests pass: `task test`
- [ ] Lint/typecheck: `task lint && task vet`

#### Manual Verification:
- [ ] `craft list -g` shows globally installed dependencies
- [ ] `craft tree -g` shows global dependency tree
- [ ] `craft update -g` updates global dependencies and reinstalls to agent
- [ ] `craft remove -g <alias>` removes from global state and deletes skill files from agent
- [ ] `craft validate -g` validates global manifest and pinfile
- [ ] `craft outdated -g` reports outdated global dependencies
- [ ] `craft install -g` reinstalls global deps to agent directories

---

## Phase 5: Documentation

### Changes Required:

- **`.paw/work/consumer-flow-global-skills/Docs.md`**: Technical reference capturing all implementation details, load `paw-docs-guidance` for template.
- **`README.md`**: Add consumer workflow section (`craft get`), document `forge/` vendoring, document `-g` flag, update command reference table.

### Success Criteria:
- [ ] Content accurate, style consistent with existing README
- [ ] All new commands and flags documented

---

## References
- Spec: `.paw/work/consumer-flow-global-skills/Spec.md`
- Research: `.paw/work/consumer-flow-global-skills/CodeResearch.md`
- WorkShaping: `.paw/work/WorkShaping-consumer-flow.md`
