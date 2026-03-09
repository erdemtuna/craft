# Feature Specification: Flat Install Layout for Agent Directories

**Branch**: feature/flat-install-layout  |  **Created**: 2026-03-09  |  **Status**: Draft
**Input Brief**: Flatten global skill install paths so AI agents can discover them

## Overview

When craft installs skills globally for AI agents like Claude Code or GitHub Copilot, it currently writes them into deeply nested directory trees mirroring the source repository path — for example, `~/.claude/skills/github.com/lossyrob/phased-agent-workflow/paw-implement/`. These agents expect skills to appear as immediate children of their skills directory. The nested layout makes globally installed skills invisible to agent discovery.

This feature introduces a flat directory naming scheme for global installs. Each skill's composite key (host/owner/repo/skill) is transformed into a single flat directory name using `--` as a component separator and replacing dots with dashes — producing names like `github-com--lossyrob--phased-agent-workflow--paw-implement`. The flat format is unique, collision-safe, and places every skill as a direct child of the agent's skills root.

Project-scoped installs (`forge/` directory) remain unchanged. The nested composite-key layout is well-suited for vendored dependencies where agent discovery is not a concern. This separation keeps the change surgical: global installs get flat layout, project installs keep nested layout.

The `list` and `tree` commands for global scope will display skills using their original composite key format (e.g., `github.com/org/repo/skill`) rather than the flat directory name, preserving readability.

## Objectives

- Enable AI agents to discover globally installed skills by placing them as direct children of the skills root directory
- Prevent skill name collisions across different source repositories through a deterministic flat naming scheme
- Maintain the existing nested layout for project-scoped installs, preserving vendoring semantics
- Keep removal operations correct by deriving flat directory names from pinfile composite keys
- Display globally installed skills in `list`/`tree` output using readable composite key format

## User Scenarios & Testing

### User Story P1 – Global Skill Installation with Agent Discovery

Narrative: A developer runs `craft get github.com/acme/tools@v1.0.0` to install a skill for their AI agent. The skill appears as a flat directory under the agent's skills root, and the agent can immediately discover and use it.

Independent Test: After `craft get`, verify the skill exists at `~/.claude/skills/github-com--acme--tools--my-skill/SKILL.md` (not nested under `github.com/acme/tools/`).

Acceptance Scenarios:
1. Given a user runs `craft get github.com/acme/tools@v1.0.0`, When the install completes, Then skills appear as flat directories directly under the agent's skills root
2. Given a user runs `craft install -g`, When the install completes, Then all global skills use flat directory names
3. Given a skill `github.com/org/repo/my-skill`, When installed globally, Then the directory name is `github-com--org--repo--my-skill`

### User Story P2 – Global Skill Update Preserves Flat Layout

Narrative: A developer updates a globally installed skill to a newer version. The updated files appear in the same flat directory without creating nested paths.

Independent Test: After `craft update -g`, verify the skill directory is still flat and contains updated files.

Acceptance Scenarios:
1. Given a globally installed skill at version v1.0.0, When the user runs `craft update -g`, Then the updated skill overwrites the same flat directory
2. Given multiple globally installed skills, When the user runs `craft update -g alias`, Then only the target skill is updated and all directories remain flat

### User Story P3 – Global Skill Removal Cleans Flat Directory

Narrative: A developer removes a globally installed skill. The flat directory is deleted completely, with no orphaned parent directories.

Independent Test: After `craft remove -g alias`, verify the flat directory no longer exists and no empty parent directories were created.

Acceptance Scenarios:
1. Given a globally installed skill, When the user runs `craft remove -g alias`, Then the flat skill directory is removed
2. Given a global removal, When cleanup completes, Then no empty intermediate directories remain (there are none to begin with in flat layout)

### User Story P4 – Project Install Unchanged

Narrative: A developer runs `craft install` in a project. Skills are vendored into `forge/` using the existing nested composite-key layout, unaffected by the flat layout change.

Independent Test: After `craft install`, verify skills exist at `forge/github.com/org/repo/skill/` (nested, not flat).

Acceptance Scenarios:
1. Given a project with dependencies, When the user runs `craft install`, Then skills appear under `forge/` in nested composite-key paths
2. Given `craft remove alias` in project scope, Then nested directories and their empty parents are cleaned up as before

### User Story P5 – Global List and Tree Display

Narrative: A developer runs `craft list -g` or `craft tree -g` to see their globally installed skills. Skills are displayed using the original composite key format for readability, not the flat directory name.

Independent Test: After installing skills globally, `craft list -g` shows `github.com/org/repo/skill` (not `github-com--org--repo--skill`).

Acceptance Scenarios:
1. Given globally installed skills, When the user runs `craft list -g`, Then skills display as `github.com/org/repo/skill`
2. Given globally installed skills, When the user runs `craft tree -g`, Then the tree shows composite key paths

### Edge Cases
- Composite key with dots in non-host segments (e.g., `github.com/my.org/repo/skill`) → flat key: `github-com--my-org--repo--skill`
- Skill names containing single dashes (e.g., `paw-implement`) → preserved in flat key; `--` separator remains unambiguous
- Mixed casing in composite keys → casing preserved in flat key
- Empty skill map → `InstallFlat()` succeeds with no-op (same as `Install()`)

## Requirements

### Functional Requirements

- FR-001: A `FlatKey()` function converts composite keys to flat directory names by replacing `/` with `--` and `.` with `-`, preserving casing (Stories: P1, P2, P3)
- FR-002: An `InstallFlat()` method installs skills using flat directory names with the same atomic staging, security validation, and overwrite semantics as `Install()` (Stories: P1, P2)
- FR-003: Global install commands (`craft install -g`, `craft get`, `craft update -g`) use `InstallFlat()` for writing skills to agent directories (Stories: P1, P2)
- FR-004: Global remove (`craft remove -g`) uses `FlatKey()` to derive directory names for cleanup, skipping parent directory cleanup (Stories: P3)
- FR-005: Project install commands continue using `Install()` with nested composite-key layout (Stories: P4)
- FR-006: Project remove continues using nested paths with parent directory cleanup (Stories: P4)
- FR-007: `craft list -g` and `craft tree -g` display skills using original composite key format from the pinfile (Stories: P5)

### Key Entities

- **Composite Key**: The `host/owner/repo/skill` string used internally (e.g., `github.com/org/repo/my-skill`)
- **Flat Key**: The transformed directory name for global installs (e.g., `github-com--org--repo--my-skill`)

### Cross-Cutting / Non-Functional

- Path traversal protection applies equally to flat keys (same validation as nested keys)
- Flat install behavior must remain consistent with nested install behavior (atomicity, security validation, overwrite semantics)

## Success Criteria

- SC-001: After global install, every skill directory is a direct child of the agent skills root (no nested subdirectories) (FR-002, FR-003)
- SC-002: `FlatKey()` output is deterministic — same input always produces same output (FR-001)
- SC-003: Two skills with the same leaf name from different repos produce different flat keys (FR-001)
- SC-004: After global remove, the flat skill directory no longer exists (FR-004)
- SC-005: Project installs produce nested paths under `forge/` unchanged from current behavior (FR-005)
- SC-006: `craft list -g` displays composite key format, not flat directory names (FR-007)
- SC-007: All existing tests continue to pass without modification (FR-005, FR-006)

## Assumptions

- GitHub does not allow `--` in organization or repository names, making `--` a collision-safe separator
- Dots in organization, repository, and skill names are extremely rare on GitHub, minimizing ambiguity risk from dot-to-dash conversion
- AI agents (Claude Code, GitHub Copilot) discover skills by scanning immediate children of their skills directory
- No existing global installs use the nested layout — this is a clean-slate change with no migration needed

## Scope

In Scope:
- `FlatKey()` pure function and `InstallFlat()` method in installer
- Wiring global install/update commands to use `InstallFlat()`
- Wiring global remove to use `FlatKey()` for cleanup
- Updating `list -g` and `tree -g` display to use composite key format
- New tests for `FlatKey()` and `InstallFlat()`
- Documentation updates (README.md, E2E_REAL_WORLD_TEST.md)

Out of Scope:
- Changes to project-scoped install/remove behavior (`forge/` layout)
- Migration of hypothetical pre-existing nested global installs
- Changes to pinfile format or resolution logic
- Changes to `Install()` function

## Dependencies

- Existing `Install()` function — `InstallFlat()` delegates to it
- Pinfile composite keys — used by remove to derive flat directory names
- Agent detection (`internal/agent/`) — determines global install target paths

## Risks & Mitigations

- **Dot-to-dash ambiguity**: Two different composite keys could theoretically produce the same flat key if they differ only in dots vs dashes. Impact: skill collision. Mitigation: This is extremely unlikely in practice; GitHub org/repo names rarely contain dots.
- **`InstallFlat()` divergence from `Install()`**: If `InstallFlat()` is implemented as a copy rather than a wrapper, future changes to `Install()` might not be reflected. Mitigation: Implement `InstallFlat()` as a thin wrapper that transforms keys then delegates to `Install()`.

## References

- WorkShaping: .paw/work/flat-install-layout/WorkShaping.md
