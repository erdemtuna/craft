# ImplementationPlan: craft-polish

## Architecture Overview

Workflow 3 adds two commands (`craft add`, `craft remove`), an internal UI package for progress and tree rendering, multi-agent interactive detection, improved error messages, and README documentation. All new code follows existing patterns (cobra commands, atomic writes, MockFetcher for tests).

## Phase 1: UI Package — Progress & Tree Rendering

**Goal:** Create `internal/ui/` with TTY-aware progress output and dependency tree rendering.

**Changes:**
- Create `internal/ui/progress.go` — `Progress` struct with `Start()`, `Update()`, `Done()`, `Fail()` methods. Writes status lines to stderr using `\r` carriage return on TTY, simple line-per-phase when not TTY. No external deps.
- Create `internal/ui/tree.go` — `RenderTree()` function that takes resolved dependencies and renders a box-drawing tree (├──, └──, │) to a writer.
- Create `internal/ui/progress_test.go` — test progress output in non-TTY mode (buffer).
- Create `internal/ui/tree_test.go` — test tree rendering against expected output.
- Add `golang.org/x/term` as direct dependency in `go.mod` for TTY detection.

**Success Criteria:**
- `go test ./internal/ui/...` passes
- Progress struct produces correct output for both TTY and non-TTY modes
- Tree renderer produces correctly formatted dependency trees

## Phase 2: `craft add` Command

**Goal:** Implement `craft add [alias] <url>` with dependency verification and optional install.

**Changes:**
- Create `internal/cli/add.go` — `addCmd` cobra command. Parses URL, derives alias from package identity if not provided, validates against existing deps, resolves to verify skills exist, updates manifest atomically.
- Register `addCmd` in `internal/cli/root.go`.
- Create `internal/cli/add_test.go` — tests: add new dep, add with alias, add duplicate (update version), add invalid URL, add with --install flag.
- Add `--install` flag to trigger full install pipeline after add.

**Success Criteria:**
- `craft add github.com/org/repo@v1.0.0` adds dependency to craft.yaml
- `craft add my-alias github.com/org/repo@v1.0.0` adds with custom alias
- Existing dependency is updated with confirmation message
- Invalid/unreachable deps produce clear error messages
- `go test ./internal/cli/...` passes

## Phase 3: `craft remove` Command

**Goal:** Implement `craft remove <alias>` with orphan cleanup.

**Changes:**
- Create `internal/cli/remove.go` — `removeCmd` cobra command. Validates alias exists, removes from manifest, removes from pinfile, identifies orphaned skills, removes orphaned skill directories from install target.
- Register `removeCmd` in `internal/cli/root.go`.
- Create `internal/cli/remove_test.go` — tests: remove existing dep, remove non-existent alias (error), remove with orphan cleanup, remove last dependency.
- Add `--target` flag for install path override (for cleanup).

**Success Criteria:**
- `craft remove <alias>` removes from craft.yaml and craft.pin.yaml
- Orphaned skills are cleaned from install target
- Non-existent alias produces error listing available aliases
- `go test ./internal/cli/...` passes

## Phase 4: Multi-Agent Detection & Progress Integration

**Goal:** Enhance agent detection for interactive multi-agent prompts. Wire progress UI into install/update commands.

**Changes:**
- Modify `internal/agent/detect.go` — add `DetectAll()` returning all detected agents instead of erroring. Keep `Detect()` for backward compat.
- Update `internal/agent/detect_test.go` — add multi-agent tests.
- Modify `internal/cli/install.go` — integrate progress UI (phases: resolving, fetching, installing), add tree display on completion. When multi-agent detected and TTY, prompt for choice.
- Modify `internal/cli/update.go` — integrate progress UI and tree display.
- Improve error messages across install.go and update.go with hint patterns.

**Success Criteria:**
- Multi-agent detection prompts interactively on TTY, errors with hint on non-TTY
- Install/update show progress phases and dependency tree on completion
- All existing tests pass
- `go build ./...` passes

## Phase 5: Error Messages & README

**Goal:** Polish error messages across all commands and write comprehensive README.

**Changes:**
- Audit and improve error messages in: install.go, update.go, add.go, remove.go, validate.go — ensure all follow `error: <msg>\n  hint: <action>` pattern.
- Create/update `README.md` — sections: Overview, Installation, Quick Start, Commands (init, validate, install, update, add, remove, cache clean, version), craft.yaml Reference, craft.pin.yaml Reference, Agent Support, Known Limitations.
- Run full test suite: `go test ./...`
- Run `go vet ./...`
- Build: `go build ./cmd/craft`

**Success Criteria:**
- All error messages include actionable hints
- README covers all 8 commands with examples
- Full test suite passes
- Build succeeds cleanly
