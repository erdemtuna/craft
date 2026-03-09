# Feature Specification: Consumer Flow & Global Skills Management

**Branch**: feature/consumer-flow-global-skills  |  **Created**: 2026-03-09  |  **Status**: Draft
**Input Brief**: Enable pure consumers to install skills from any repo with a single command (`craft get`), add global scope (`-g`) to existing commands, and change project installs to vendor into `forge/` instead of writing to agent directories.

## Overview

Today, developers who discover a useful set of AI agent skills on GitHub face an awkward path to install them. They must first run `craft init` to create a skill package manifest — even though they aren't authoring skills — then `craft add` the dependency, then `craft install`. This three-step author workflow imposes unnecessary ceremony on someone who simply wants skills available in their Claude Code or GitHub Copilot agent.

The manual alternative — cloning a repo and copying files to `~/.claude/skills/` — is faster but sacrifices everything craft offers: version pinning, integrity verification, transitive dependency resolution, and an update path. Users shouldn't have to choose between convenience and safety.

This feature introduces a clear separation between two personas. **Consumers** get a single-command entry point (`craft get`) that fetches, resolves, verifies, and installs skills globally to their agent. **Authors** continue using `craft init`, `craft add`, and `craft install` for package development, but `craft install` now vendors dependencies into a project-local `forge/` directory instead of writing directly to agent directories. Both personas manage their installed skills with the same verbs (`list`, `update`, `remove`, etc.) — distinguished only by a `--global` / `-g` flag.

The result is a tool that meets users where they are: consumers get a near-zero-ceremony install experience with full craft guarantees, while authors get a clean separation between their project's dependency tree and their global agent setup.

## Objectives

- Enable consumers to install skills from any repo in a single command without creating a project or running `craft init`
- Introduce global skill state (`~/.craft/craft.yaml` + `~/.craft/craft.pin.yaml`) so consumers can track, update, and remove their installed skills
- Separate project-scoped dependency installation (`forge/`) from global agent installation to prevent conflicts between the two contexts
- Provide consistent lifecycle management across both scopes using the same command verbs with a `-g` flag
- Maintain craft's core guarantees (version pinning, integrity verification, transitive resolution) in both consumer and author workflows

## User Scenarios & Testing

### User Story P1 – Consumer Installs Skills Globally

Narrative: A developer finds a skill package on GitHub. They run a single command to install its skills into their AI agent. The skills appear in their agent immediately, and craft tracks what was installed for future updates.

Independent Test: Run `craft get github.com/alice/skills@v1.0.0` and verify skills appear in the agent's skill directory.

Acceptance Scenarios:
1. Given a valid dependency URL with a tag ref, When the user runs `craft get <url>`, Then skills are installed to the detected agent directory, and `~/.craft/craft.yaml` and `~/.craft/craft.pin.yaml` are created/updated with the new dependency.
2. Given no `~/.craft/craft.yaml` exists, When the user runs `craft get <url>` for the first time, Then the global manifest is auto-created with `schema_version: 1`, `name: global`, and the dependency entry.
3. Given the dependency has transitive dependencies, When the user runs `craft get <url>`, Then all transitive dependencies are resolved, pinned, and their skills installed.
4. Given the dependency is already globally installed at a different version, When the user runs `craft get <url>@<new-version>`, Then the user is prompted whether they want to update, and if confirmed the version is updated in manifest, pinfile, and agent directory.
5. Given no agent directory (`~/.claude/` or `~/.copilot/`) exists, When the user runs `craft get`, Then an error is shown: "No agent detected. Do you have Claude Code or GitHub Copilot installed?"
6. Given multiple agent directories exist and stdin is a TTY, When the user runs `craft get`, Then the user is prompted to choose one agent, all agents, or cancel.
7. Given multiple agent directories exist and stdin is not a TTY, When the user runs `craft get`, Then an error is shown suggesting `--target <path>`.

### User Story P2 – Consumer Manages Global Skills

Narrative: A developer who has previously installed skills via `craft get` wants to list what they have, update to newer versions, or remove skills they no longer need.

Independent Test: Run `craft list -g` after installing skills globally and verify the installed dependencies are listed.

Acceptance Scenarios:
1. Given skills are globally installed, When the user runs `craft list -g`, Then all globally tracked dependencies are listed with aliases and versions.
2. Given skills are globally installed, When the user runs `craft update -g`, Then all updatable dependencies are checked for newer versions and updated in global manifest, pinfile, and agent directory.
3. Given skills are globally installed, When the user runs `craft update -g <alias>`, Then only the specified dependency is updated.
4. Given a dependency is globally installed, When the user runs `craft remove -g <alias>`, Then the dependency is removed from the global manifest and pinfile, and its skill files are deleted from the agent directory.
5. Given skills are globally installed, When the user runs `craft tree -g`, Then the global dependency tree is displayed.
6. Given skills are globally installed, When the user runs `craft validate -g`, Then the global manifest and pinfile are validated.
7. Given skills are globally installed, When the user runs `craft outdated -g`, Then outdated global dependencies are reported.
8. Given skills are globally installed, When the user runs `craft install -g`, Then all globally tracked dependencies are re-installed to agent directories from pinned state.

### User Story P3 – Author Vendors Dependencies to `forge/`

Narrative: A skill author runs `craft install` in their project. Dependencies are vendored into a local `forge/` directory for reference and development, rather than being installed to the global agent directory.

Independent Test: Run `craft install` in a project with dependencies and verify skill files appear in `forge/` and not in `~/.claude/skills/`.

Acceptance Scenarios:
1. Given a project with dependencies in `craft.yaml`, When the user runs `craft install`, Then dependencies are resolved, `craft.pin.yaml` is written, and dependency skill files are written to `forge/` in the project root.
2. Given `forge/` is not in `.gitignore`, When the user runs `craft install`, Then `forge/` is auto-added to `.gitignore`.
3. Given a project with dependencies, When the user runs `craft add --install <url>`, Then the dependency is added to `craft.yaml` and `craft install` is triggered, vendoring to `forge/`.
4. Given a project with dependencies, When the user runs `craft install`, Then no files are written to `~/.claude/skills/` or `~/.copilot/skills/`.

### User Story P4 – Consumer Uses `craft get` with Multiple URLs

Narrative: A developer wants to install skills from several repos at once, similar to `go install pkg1@v1 pkg2@v2`.

Independent Test: Run `craft get <url1> <url2>` and verify both packages' skills are installed globally.

Acceptance Scenarios:
1. Given two valid dependency URLs, When the user runs `craft get <url1> <url2>`, Then both are resolved, pinned, and installed to the agent directory.
2. Given one valid and one invalid URL, When the user runs `craft get <url1> <invalid>`, Then an error is reported and neither dependency is installed (atomic behavior).

### User Story P5 – Consumer Previews Before Installing

Narrative: A developer wants to see what `craft get` would do before committing to it.

Independent Test: Run `craft get --dry-run <url>` and verify no files are written but a preview is shown.

Acceptance Scenarios:
1. Given a valid dependency URL, When the user runs `craft get --dry-run <url>`, Then the resolved dependency tree and target agent directory are displayed, but no files are written to disk.

### Edge Cases

- `craft get` with a dependency URL missing `@ref` produces an error with usage hint: "Missing version — use `@vX.Y.Z`, `@branch:<name>`, or `@<commit-sha>`"
- `craft get` in a directory with a `craft.yaml` still operates on the global manifest (not the project manifest)
- `craft remove -g` on the last dependency results in an empty global manifest (not deleted)
- `craft install -g` with no global manifest produces an error: "No global skills installed. Use `craft get` to install skills."
- `craft install` with no dependencies in project `craft.yaml` skips `forge/` creation
- `craft get` with the same URL and same version as already installed reports "already installed" and exits cleanly

## Requirements

### Functional Requirements

- FR-001: `craft get [alias] <url>[@ref] [url[@ref]...]` command that resolves, pins, and installs skills to agent directories from global state (Stories: P1, P4, P5)
- FR-002: Global manifest auto-creation at `~/.craft/craft.yaml` on first `craft get`, with `schema_version: 1`, `name: global`, and empty skills list (Stories: P1)
- FR-003: Global pinfile at `~/.craft/craft.pin.yaml` tracking pinned commits and SHA-256 integrity digests for globally installed dependencies (Stories: P1)
- FR-004: `--global` / `-g` flag on `list`, `update`, `remove`, `install`, `tree`, `validate`, and `outdated` commands that redirects them to operate on `~/.craft/craft.yaml` and `~/.craft/craft.pin.yaml` (Stories: P2)
- FR-005: `craft install` (project scope) vendors resolved dependency skill files to `forge/` directory in the project root instead of agent directories (Stories: P3)
- FR-006: Auto-add `forge/` to `.gitignore` on first `craft install` if not already present (Stories: P3)
- FR-007: `craft add --install` triggers vendoring to `forge/` (not agent directory installation) (Stories: P3)
- FR-008: `craft get` prompts user to confirm update when the dependency is already globally installed at a different version (Stories: P1)
- FR-009: `craft get` accepts multiple dependency URLs in a single invocation, resolving and installing all atomically (Stories: P4)
- FR-010: `craft get --dry-run` previews resolution and install targets without writing any files (Stories: P5)
- FR-011: `craft remove -g` deletes skill files from agent directories and removes the dependency from global manifest and pinfile (Stories: P2)
- FR-012: Agent detection for global commands — error if none found, auto-select if one, interactive prompt if multiple (TTY), error if multiple (non-TTY) (Stories: P1, P2)
- FR-013: `--target` flag on `craft get` and `craft install -g` to override agent auto-detection (Stories: P1, P2)
- FR-014: Alias auto-derivation from repository name for `craft get`, with optional override via positional argument (Stories: P1)

### Key Entities

- **Global Manifest**: `~/.craft/craft.yaml` — tracks globally installed dependencies, auto-created on first `craft get`
- **Global Pinfile**: `~/.craft/craft.pin.yaml` — pins exact commits and integrity digests for global dependencies
- **Forge Directory**: `./forge/` — project-local vendored dependency skills, gitignored, reproduced from `craft.pin.yaml`
- **Agent Directory**: `~/.claude/skills/` or `~/.copilot/skills/` — where agents load skills from, managed exclusively by global commands

### Cross-Cutting / Non-Functional

- All global operations must work without a project `craft.yaml` in the current directory
- Project operations must not write to agent directories
- Global operations must not write to project `forge/` directory
- Agent choice is never persisted — always prompt on multiple detection (use `--target` to skip)
- `forge/` is always gitignored, never committed

## Success Criteria

- SC-001: A user with no `craft.yaml` in their current directory can run `craft get <url>` and have skills installed to their agent directory within a single command (FR-001, FR-002, FR-003)
- SC-002: Running `craft list -g`, `craft update -g`, `craft remove -g`, `craft tree -g`, `craft validate -g`, `craft outdated -g`, and `craft install -g` all operate on global state without requiring a project manifest (FR-004)
- SC-003: Running `craft install` in a project writes dependency files to `forge/` and does not write to any agent directory (FR-005)
- SC-004: The `forge/` entry appears in `.gitignore` after first `craft install` (FR-006)
- SC-005: Running `craft get` for an already-installed dependency prompts the user before updating (FR-008)
- SC-006: Running `craft get url1 url2` installs both atomically — either all succeed or none are written (FR-009)
- SC-007: Running `craft remove -g <alias>` removes skill files from the agent directory (FR-011)

## Assumptions

- The global manifest default name is `"global"` — this is a sentinel value, not user-configurable
- `Skills` field in the global manifest is an empty slice — global manifests don't export skills
- The existing `Manifest` struct and validation accept an empty `Skills` slice for global manifests
- `forge/` directory structure mirrors the agent install structure: `forge/host/org/repo/skill-name/`
- The global cache at `~/.craft/cache/` is shared between project and global operations (no duplication)
- Agent detection logic and `--target` flag are reused as-is from current implementation via `resolveInstallTargets`

## Scope

In Scope:
- New `craft get` command with all described behaviors
- `--global` / `-g` flag on `list`, `update`, `remove`, `install`, `tree`, `validate`, `outdated`
- `craft install` change to vendor to `forge/` instead of agent directories
- `craft add --install` behavior change to vendor to `forge/`
- Global manifest and pinfile auto-creation and management at `~/.craft/`
- Skill file deletion from agent directories on `craft remove -g`
- Auto-adding `forge/` to `.gitignore`
- `--dry-run` and `--target` flags on `craft get`
- Multiple URL support for `craft get`

Out of Scope:
- Changes to `craft init` (remains author-only, unchanged)
- Persisting agent choice across invocations
- Implicit "latest tag" resolution (version/ref always required)
- Central registry or search functionality
- `craft get` operating on project manifests (always global)
- Publishing or pushing skills to remotes

## Dependencies

- Existing `ParseDepURL` logic for URL parsing (unchanged)
- Existing `resolver.Resolve` for dependency resolution (reused for global context)
- Existing `agent.Detect` / `agent.DetectAll` for agent discovery (reused)
- Existing `installlib.Install` for atomic skill file writing (reused for global installs and forge vendoring)
- Existing `remove.go` cleanup logic for skill file deletion (adapted for global scope)

## Risks & Mitigations

- **Global manifest conflicts with project manifest**: A user might run `craft get` inside a project directory. Risk: confusing which manifest is being operated on. Mitigation: `craft get` always operates on `~/.craft/craft.yaml` regardless of current directory. Document clearly.
- **`forge/` name collision**: Another tool or convention might use `forge/`. Risk: low, name is distinctive. Mitigation: craft owns the directory and documents it.
- **Empty `Skills` validation failure**: Current validation may reject manifests with no skills. Risk: global manifest creation fails. Mitigation: ensure validation allows empty skills for global scope.
- **Uninstall leaves empty directories**: Removing skills may leave orphaned parent directories. Mitigation: clean up empty parent directories during removal (existing logic in `remove.go`).

## References

- WorkShaping: `.paw/work/WorkShaping-consumer-flow.md`
