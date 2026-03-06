# ImplementationPlan: craft-polish

## Architecture Overview

Workflow 3 adds two commands (`craft add`, `craft remove`), an internal UI package for progress and tree rendering, multi-agent interactive detection, improved error messages, and README documentation. All new code follows existing patterns (cobra commands, atomic writes, MockFetcher for tests).

## Phase 1: UI Package — Progress & Tree Rendering

**Goal:** Create `internal/ui/` with TTY-aware progress output and dependency tree rendering.

**Changes:**
- Create `internal/ui/progress.go` — `Progress` struct with `Start()`, `Update()`, `Done()`, `Fail()` methods. Writes status lines to stderr using `\r` carriage return on TTY. Suppresses all progress output when stderr is not a TTY (CI-friendly, per NFR-2). No external deps for rendering.
- Create `internal/ui/tree.go` — `RenderTree()` function that takes resolved dependencies AND local skill paths, renders a box-drawing tree (├──, └──, │) to a writer. Local skills shown separately at the top of the tree output.
- Create `internal/ui/progress_test.go` — test progress output: verify TTY mode produces status lines, non-TTY mode produces no output.
- Create `internal/ui/tree_test.go` — test tree rendering against expected output including local skills section and empty local skills case.
- Add `golang.org/x/term` as direct dependency in `go.mod` for TTY detection (already transitive via go-git).

**Success Criteria:**
- `go test ./internal/ui/...` passes
- Progress struct produces status lines on TTY, no output on non-TTY
- Tree renderer produces correctly formatted dependency trees with local skills section at top
- Count progress format supports "Fetching dependency 2/5..." pattern

## Phase 2: `craft add` Command

**Goal:** Implement `craft add [alias] <url>` with dependency verification and optional install.

**Changes:**
- Create `internal/cli/add.go` — `addCmd` cobra command. Parses URL, derives alias from package identity if not provided, validates against existing deps. Validation strategy: load current manifest → add new dep in memory → call `Resolve()` with full manifest and `ForceResolve` for the new dep → on success, write manifest atomically. Updates version if dependency already exists (with confirmation message).
- Register `addCmd` in `internal/cli/root.go`.
- Create `internal/cli/add_test.go` — tests: add new dep, add with alias, add duplicate (update version), add invalid URL, add with --install flag, add without craft.yaml (error with "Run `craft init`" hint).
- Add `--install` flag to trigger full install pipeline after add. Note: in Phase 2, this runs the basic install pipeline; progress UI is retroactively added in Phase 4.

**Success Criteria:**
- `craft add github.com/org/repo@v1.0.0` adds dependency to craft.yaml
- `craft add my-alias github.com/org/repo@v1.0.0` adds with custom alias
- Existing dependency is updated with confirmation message
- Prints summary: skill names discovered, version resolved
- Invalid/unreachable deps produce clear error messages with hints
- Missing craft.yaml produces "Run `craft init` to create one" hint
- `go test ./internal/cli/...` passes

## Phase 3: `craft remove` Command

**Goal:** Implement `craft remove <alias>` with orphan cleanup.

**Changes:**
- Create `internal/cli/remove.go` — `removeCmd` cobra command. Validates alias exists, removes from manifest, removes from pinfile. Orphan detection algorithm: collect all skill names from remaining deps' pinfile entries, diff against removed dep's skills, delete only skills not in the remaining set. Removes orphaned skill directories from install target.
- Register `removeCmd` in `internal/cli/root.go`.
- Create `internal/cli/remove_test.go` — tests: remove existing dep, remove non-existent alias (error listing available aliases), remove with orphan cleanup, remove with shared skills (verify shared skills retained), remove last dependency.
- Add `--target` flag for install path override (for cleanup), matching install/update behavior.

**Success Criteria:**
- `craft remove <alias>` removes from craft.yaml and craft.pin.yaml
- Orphaned skills are cleaned from install target; shared skills are retained
- Non-existent alias produces error listing available aliases
- `go test ./internal/cli/...` passes

## Phase 4: Multi-Agent Detection & Progress Integration

**Goal:** Enhance agent detection for interactive multi-agent prompts. Wire progress UI into install/update commands.

**Changes:**
- Modify `internal/agent/detect.go` — add `DetectAll()` returning all detected agents instead of erroring. Keep `Detect()` for backward compat.
- Update `internal/agent/detect_test.go` — add multi-agent tests.
- Modify `internal/cli/install.go` — integrate progress UI (phases: resolving, fetching, installing), add tree display on completion to stderr (via `cmd.ErrOrStderr()`). When multi-agent detected and stdin is TTY, prompt for choice (Claude Code / Copilot / Both). No persistence for MVP — prompt each time. Refactor core install logic into reusable function for `craft add --install`.
- Modify `internal/cli/update.go` — integrate progress UI and tree display (stderr).
- Improve error messages across install.go and update.go with `hint:` patterns.
- Migrate existing `fix:` keyword in validate.go to `hint:` for consistency.

**Success Criteria:**
- Multi-agent detection prompts interactively on stdin TTY, errors with `--target` hint on non-TTY
- "Both" option installs to both agent paths
- Install/update show progress phases on stderr TTY and dependency tree on completion to stderr
- All existing tests pass
- `go build ./...` passes

## Phase 5: Error Messages & README

**Goal:** Polish error messages across all commands and write comprehensive README.

**Changes:**
- Audit and improve error messages in all commands. Specific patterns from Spec US-6:
  1. Missing `craft.yaml` → "Run `craft init` to create one" (add.go, install.go, update.go, remove.go)
  2. Repository not found → "Check the URL or set GITHUB_TOKEN for private repos" (install.go, add.go)
  3. No skills found → "Ensure the repo has SKILL.md files" (install.go, add.go)
  4. Alias not found → list available aliases (remove.go, update.go)
  5. Version tag doesn't exist → list available tags up to 10 (add.go, update.go — ListTags() exists in fetcher)
  6. Network errors → "Check your connection or run with cached data" (install.go, update.go, add.go)
- Ensure all use `hint:` keyword (not `fix:`), including migrating validate.go
- Update README.md — expand existing 166-line README (not replace). Add sections: Installation, Quick Start walkthrough, document 3 new commands (add, remove, cache clean), Agent Support details, Known Limitations (go-git SSH, monorepo paths). Preserve existing content structure.
- Run full test suite: `go test ./...`
- Run `go vet ./...`
- Build: `go build ./cmd/craft`

**Success Criteria:**
- All error messages include actionable hints
- README covers all 8 commands with examples
- Full test suite passes
- Build succeeds cleanly
