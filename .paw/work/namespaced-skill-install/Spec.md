# Feature Specification: Namespaced Skill Installation

**Branch**: feature/namespaced-skill-install  |  **Created**: 2026-03-09  |  **Status**: Draft
**Input Brief**: Allow same-name skills from different dependencies by namespacing install paths with host/owner/repo.

## Overview

Craft is a skill package manager that resolves, installs, and manages skills from multiple git-hosted dependencies. Today, all dependency skills install into a flat directory structure under the install target — each skill occupies `<target>/<skill-name>/`. When two independent dependencies export a skill with the same name, craft hard-errors with a collision, preventing installation entirely.

This is a real-world problem: the PAW workflow repository and Anthropic's skills repository both export a skill named `skill-creator`. Users who want both dependencies in their package cannot proceed. The collision check is overly strict — independent repositories will naturally have overlapping skill names, and users should be able to use both without conflict.

The solution is to namespace installed skills by their source repository identity. By using the dependency URL's `host/owner/repo` as a directory prefix, skills from different sources occupy distinct paths on disk. Two `skill-creator` skills coexist peacefully because they live under `github.com/lossyrob/phased-agent-workflow/skill-creator/` and `github.com/anthropics/skills/skill-creator/` respectively.

This approach mirrors Go module paths — a convention craft users already understand — and works uniformly for all git hosts (GitHub, GitLab, Bitbucket, self-hosted), for both direct and transitive dependencies.

## Objectives

- Enable users to depend on multiple repositories that export same-name skills without errors
- Provide a deterministic, globally unique install layout that cannot collide regardless of host, owner, or repo
- Maintain correct cleanup behavior when dependencies are removed
- Preserve existing behavior for local skills (which are not installed by craft)

## User Scenarios & Testing

### User Story P1 – Install dependencies with overlapping skill names

Narrative: A developer adds two dependencies to their craft package. Both export a skill called `skill-creator`. They run `craft install` and both dependencies' skills are installed without error, each under its own namespaced directory.

Independent Test: Run `craft install` with two dependencies that share a skill name and verify both skills exist on disk at distinct paths.

Acceptance Scenarios:
1. Given a craft.yaml with two dependencies that each export `skill-creator`, When the user runs `craft install`, Then both skills are installed under `<target>/<host>/<owner>/<repo>/skill-creator/` with no error.
2. Given a craft.yaml with two dependencies that have no overlapping skill names, When the user runs `craft install`, Then all skills are installed under their respective `<host>/<owner>/<repo>/` namespaces.
3. Given a craft.yaml with a single dependency, When the user runs `craft install`, Then skills are installed under `<target>/<host>/<owner>/<repo>/<skill-name>/`.

### User Story P2 – Remove a dependency with overlapping skill names

Narrative: A developer has two installed dependencies sharing a skill name. They remove one dependency and expect only that dependency's skills to be cleaned up, while the other dependency's identically-named skill remains intact.

Independent Test: Run `craft remove` for one dependency and verify the other dependency's same-name skill still exists at its namespaced path.

Acceptance Scenarios:
1. Given two installed dependencies both exporting `skill-creator`, When the user removes one dependency, Then only that dependency's `skill-creator` is deleted, and the other dependency's `skill-creator` remains.
2. Given a removed dependency was the only one from its owner, When `craft remove` completes, Then the empty `<host>/<owner>/<repo>/` directory tree is cleaned up.

### User Story P3 – Update a dependency preserves namespaced layout

Narrative: A developer updates a dependency to a newer version. The updated skills are re-installed under the same namespaced path.

Independent Test: Run `craft update` and verify skills remain at `<target>/<host>/<owner>/<repo>/<skill-name>/`.

Acceptance Scenarios:
1. Given an installed dependency at v1.0.0, When the user runs `craft update` and the dependency bumps to v2.0.0, Then skills are re-installed under the same `<host>/<owner>/<repo>/` namespace with updated content.

### User Story P4 – Add a dependency that would have previously collided

Narrative: A developer already has a dependency installed. They add a second dependency that exports a skill with the same name. The add succeeds without a collision error.

Independent Test: Run `craft add` for a dependency whose skills overlap with an existing dependency, and verify success.

Acceptance Scenarios:
1. Given an existing dependency exporting `skill-creator`, When the user runs `craft add` for a second dependency also exporting `skill-creator`, Then the add succeeds and manifest is updated.

### Edge Cases

- Empty parent directories (`<host>/<owner>/`, `<host>/`) are cleaned up after skill removal when no other skills remain under them.
- A dependency that exports zero skills still resolves and installs without error (no namespace directory created).
- Skills with path-separator characters in their names are rejected by existing path traversal checks (existing behavior, unchanged).
- Non-GitHub hosts (GitLab, Bitbucket, self-hosted) produce valid namespace paths (e.g., `gitlab.com/org/repo/skill-name/`).

## Requirements

### Functional Requirements

- FR-001: Dependency skills are installed under `<target>/<host>/<owner>/<repo>/<skill-name>/` where host, owner, and repo are parsed from the dependency URL. (Stories: P1)
- FR-002: The cross-dependency collision detection (`detectCollisions`) is removed from the resolver — same-name skills across different dependencies are permitted. (Stories: P1, P4)
- FR-003: Skill removal constructs cleanup paths using `<host>/<owner>/<repo>/<skill-name>/` and removes empty ancestor directories up to the target root. (Stories: P2)
- FR-004: Skill updates re-install under the same `<host>/<owner>/<repo>/` namespace. (Stories: P3)
- FR-005: The local skill duplicate name validation within a single package is preserved. (Stories: P1)
- FR-006: The `Install()` function signature remains unchanged — namespacing is achieved by using composite keys (`host/owner/repo/skillName`) in the existing skill map. (Stories: P1)

### Cross-Cutting / Non-Functional

- Existing path traversal security checks continue to function correctly with deeper directory nesting.
- Atomic install via staging directories continues to work with composite keys.

## Success Criteria

- SC-001: Two dependencies exporting the same skill name can be added, installed, and used without any error. (FR-001, FR-002)
- SC-002: After removing one of two dependencies that share a skill name, the remaining dependency's skill is intact and the removed dependency's skill is gone. (FR-003)
- SC-003: Empty ancestor directories are cleaned up after the last skill under them is removed. (FR-003)
- SC-004: All existing tests pass (updated for new paths) and new tests cover the namespaced layout. (FR-001 through FR-006)
- SC-005: Skills from non-GitHub hosts install under the correct host-prefixed path. (FR-001)

## Assumptions

- There are no existing craft users who need migration from the flat layout to the namespaced layout. (Confirmed during shaping.)
- The `DepURL` struct already exposes `Host`, `Org`, and `Repo` fields sufficient to construct the namespace prefix. (Verified in `internal/resolve/depurl.go`.)
- Local skills are not installed by craft and require no changes. (Verified in `internal/cli/install.go`.)

## Scope

In Scope:
- Namespaced install layout for all dependency skills (direct and transitive)
- Removal of `detectCollisions()` from resolver
- Updated cleanup logic in `craft remove`
- Test updates for new expected paths
- E2E test documentation updates

Out of Scope:
- Local skill installation changes (local skills stay in-repo)
- Alias-based symlinks for shorter references (see [issue #27](https://github.com/erdemtuna/craft/issues/27))
- Pinfile structure changes (existing structure is sufficient)
- Migration tooling for existing flat layouts

## Dependencies

- `internal/resolve/depurl.go` — `ParseDepURL()` and `DepURL` struct for extracting host/owner/repo
- Existing path traversal security in `internal/install/installer.go`

## Risks & Mitigations

- **Deeper directory nesting may affect downstream tool discovery**: Tools consuming installed skills may expect a flat `<target>/*/SKILL.md` pattern. Mitigation: This is the consumer's responsibility; craft's job is conflict-free installation. Document the new layout.
- **Composite key with `/` in map keys could cause confusion**: Developers might mistake the composite key for a simple skill name. Mitigation: The `Install()` function treats it as a path naturally via `filepath.Join` — no special handling needed.

## References

- WorkShaping: `.paw/work/namespaced-skill-install/WorkShaping.md` (session artifact)
- E2E test scenario: `E2E_REAL_WORLD_TEST.md`
- Alias symlinks follow-up: [issue #27](https://github.com/erdemtuna/craft/issues/27)
