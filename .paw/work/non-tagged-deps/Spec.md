# Feature Specification: Non-Tagged Repository Dependencies

**Branch**: feature/non-tagged-deps  |  **Created**: 2026-03-09  |  **Status**: Draft
**Input Brief**: Enable craft to consume skill packages from repositories that haven't published semver git tags, using commit SHA or branch-tracking refs.

## Overview

Craft's dependency system currently requires all repositories to publish strict semver git tags (`vMAJOR.MINOR.PATCH`). The dependency URL format enforces this at the syntax level, and the entire resolution pipeline — tag listing, semver comparison, Minimum Version Selection — is built around tagged versions. This means useful skill repositories maintained without formal releases are completely inaccessible to craft users.

Many third-party skill repositories on GitHub are maintained without tagging releases. Their authors publish and iterate on skills by pushing commits to branches, never cutting formal version tags. Craft users who want to consume these skills are forced into manual copy-paste workflows — the exact anti-pattern craft was designed to eliminate. This is the single biggest barrier to third-party skill adoption.

This feature introduces a tiered dependency model. Tagged dependencies retain their full rigor and reproducibility guarantees. Non-tagged dependencies — commit-pinned and branch-tracked — operate under explicitly weaker "best-effort" guarantees, with warnings that communicate the tradeoff. The dependency URL syntax is extended to express all three ref types naturally within the existing `@`-notation, and the resolution pipeline gains ref-type-aware routing that preserves existing behavior for tagged deps while supporting the new modes.

The design is intentionally incremental: existing tagged dependency behavior is completely unchanged, and non-tagged support layers on top without modifying the core semver resolution path.

## Objectives

- Enable users to depend on skill packages from repositories without semver tags (Rationale: removes the primary adoption barrier for third-party skills)
- Provide commit-pinning for reproducible, frozen dependency snapshots (Rationale: users who want deterministic builds can lock to an exact commit)
- Provide branch-tracking for dependencies that should follow upstream development (Rationale: teams iterating together need latest-commit semantics)
- Communicate reproducibility tradeoffs clearly through warnings (Rationale: users must understand the weaker guarantees of non-tagged deps)
- Preserve full rigor for existing tagged dependencies (Rationale: no regressions for current users)
- Detect and surface conflicts when the same package is referenced with incompatible ref types (Rationale: prevent silent resolution ambiguity)

## User Scenarios & Testing

### User Story P1 – Add a Commit-Pinned Dependency

Narrative: A developer wants to use skills from a repository that has no tags. They know a specific commit SHA that contains the skills they need. They add a commit-pinned dependency, install it, and the skills are available.

Independent Test: Run `craft add` with a commit SHA ref, then `craft install`, and verify the skills from that exact commit are installed.

Acceptance Scenarios:
1. Given a valid repository and commit SHA, When the user runs `craft add skills <repo>@<sha>`, Then the dependency is added to `craft.yaml` with the commit ref, and a warning about weaker guarantees is displayed.
2. Given a commit-pinned dependency in `craft.yaml`, When the user runs `craft install`, Then the exact commit's skills are fetched, integrity is computed, and the pinfile records the commit SHA with `ref_type: commit`.
3. Given a commit-pinned dependency, When the user runs `craft update`, Then the commit-pinned dependency is silently skipped (no-op) because a commit pin is a deliberate freeze.
4. Given an invalid or nonexistent commit SHA, When the user runs `craft add`, Then the command fails with a clear "commit not found" error.
5. Given a short SHA (≥7 characters), When the user runs `craft add`, Then craft resolves it to the full 40-character SHA before storing.

### User Story P2 – Track a Branch Dependency

Narrative: A developer wants to consume skills from a repository's `main` branch, always getting the latest version. They add a branch-tracked dependency, install it, and can later update to get new commits.

Independent Test: Run `craft add` with a `branch:main` ref, `craft install`, then after upstream changes run `craft update`, and verify the pinfile updates to the new commit.

Acceptance Scenarios:
1. Given a valid repository and branch name, When the user runs `craft add skills <repo>@branch:<name>`, Then the dependency is added to `craft.yaml` with the branch ref, and a warning about weaker guarantees is displayed.
2. Given a branch-tracked dependency in `craft.yaml`, When the user runs `craft install`, Then the branch HEAD is resolved to a commit SHA, skills are fetched from that commit, and the pinfile records the commit SHA with `ref_type: branch`.
3. Given a branch-tracked dependency, When the user runs `craft update`, Then the branch HEAD is re-resolved, and if it differs from the pinned commit, the pinfile is updated with the new commit, integrity, and discovered skills.
4. Given a branch-tracked dependency where the branch has been deleted, When the user runs `craft install` with an existing pinfile, Then install succeeds using the previously pinned commit SHA. When the user runs `craft update`, Then update fails with a clear "branch not found" error.
5. Given a branch name that looks like a hex string (e.g., `deadbeef`), When the user uses the `branch:` prefix, Then it is correctly treated as a branch name, not a commit SHA.

### User Story P3 – Mixed Ref-Type Conflict Detection

Narrative: A project has a direct dependency on `acme/tools@v1.2.0` and a transitive dependency requires `acme/tools@branch:main`. Craft detects this conflict and tells the user to resolve it.

Independent Test: Create a dependency graph where the same package appears with different ref types, run `craft install`, and verify an error is raised.

Acceptance Scenarios:
1. Given the same package referenced with a tag ref and a branch ref, When resolution runs, Then craft raises an error: "conflicting ref types for package X — resolve manually."
2. Given the same package referenced with a tag ref and a commit ref, When resolution runs, Then craft raises the same conflict error.
3. Given the same package referenced with a branch ref and a commit ref, When resolution runs, Then craft raises the same conflict error.

### User Story P4 – Validation Warnings for Non-Tagged Dependencies

Narrative: A user runs `craft validate` on a project that includes non-tagged dependencies. The validation output includes warnings for each non-tagged dep, informing them of the weaker guarantees.

Independent Test: Run `craft validate` on a project with branch-tracked and commit-pinned deps, and verify yellow warning messages appear for each.

Acceptance Scenarios:
1. Given a project with branch-tracked dependencies, When the user runs `craft validate`, Then a yellow-text warning is displayed for each branch dep noting weaker reproducibility.
2. Given a project with commit-pinned dependencies, When the user runs `craft validate`, Then a yellow-text warning is displayed for each commit dep.
3. Given a project with only tagged dependencies, When the user runs `craft validate`, Then no non-tagged warnings appear (existing behavior unchanged).

### Edge Cases

- A bare hex string ≥7 characters in the ref position is treated as a commit SHA; shorter strings or non-hex strings are treated as errors (not branch names).
- Branch names must use the `branch:` prefix to disambiguate from commit SHAs.
- A dependency URL with no ref at all (e.g., `github.com/acme/tools`) results in an error requiring an explicit ref.
- When `craft update` is run on a project with only commit-pinned deps, it completes successfully as a no-op with a clean exit.
- Transitive non-tagged dependencies receive the same warning treatment as direct non-tagged dependencies.
- Short SHAs are resolved to full 40-character SHAs; the full SHA is stored in the pinfile.
- Non-tagged deps still receive SHA-256 integrity digests in the pinfile; integrity verification applies per-install.

## Requirements

### Functional Requirements

- FR-001: The dependency URL syntax accepts commit SHA refs in the form `host/org/repo@<sha>` where `<sha>` is a hexadecimal string of 7 or more characters. (Stories: P1)
- FR-002: The dependency URL syntax accepts branch refs in the form `host/org/repo@branch:<name>`. (Stories: P2)
- FR-003: Existing semver tag refs (`host/org/repo@vX.Y.Z`) continue to work unchanged. (Stories: P1, P2, P3, P4)
- FR-004: A dependency URL with no ref produces an error requiring an explicit ref. (Stories: P1, P2)
- FR-005: Commit SHA refs are resolved by verifying the SHA exists in the repository; short SHAs are expanded to full 40-character SHAs. (Stories: P1)
- FR-006: Branch refs are resolved by looking up the branch HEAD commit SHA. (Stories: P2)
- FR-007: The pinfile records a `ref_type` field (`tag`, `commit`, or `branch`) for each pinned dependency. (Stories: P1, P2)
- FR-008: The pinfile stores the full resolved commit SHA for all ref types. (Stories: P1, P2)
- FR-009: `craft update` re-resolves branch deps to the latest branch HEAD and updates the pinfile if changed. (Stories: P2)
- FR-010: `craft update` silently skips commit-pinned deps (no-op). (Stories: P1)
- FR-011: `craft update` uses existing MVS behavior for tag deps (unchanged). (Stories: P3)
- FR-012: When the same package is referenced with different ref types during resolution, craft raises a conflict error requiring manual resolution. (Stories: P3)
- FR-013: `craft add` accepts non-tagged refs, auto-detects the ref type from the URL, and validates the ref exists before updating the manifest. (Stories: P1, P2)
- FR-014: `craft add` displays a yellow-text warning when adding a non-tagged dependency. (Stories: P1, P2)
- FR-015: `craft validate` displays yellow-text warnings for each non-tagged dependency in the project. (Stories: P4)
- FR-016: Non-tagged dependencies receive SHA-256 integrity digests in the pinfile, identical to tagged deps. (Stories: P1, P2)

### Key Entities

- **RefType**: Enumeration of dependency reference types — `tag`, `commit`, `branch`.
- **DepURL**: A parsed dependency URL including host, org, repo, ref value, and ref type.
- **PinnedDep**: A resolved and locked dependency entry in the pinfile, extended with `ref_type` metadata.

### Cross-Cutting / Non-Functional

- Warnings use yellow text formatting and are non-blocking (do not prevent command completion).
- Error messages for invalid refs, missing commits, deleted branches, and ref-type conflicts are clear and actionable.
- Backward compatibility: pinfiles without `ref_type` default to `tag` behavior.

## Success Criteria

- SC-001: A user can add, install, and use skills from a repository that has no semver tags, using either a commit SHA or branch name as the ref. (FR-001, FR-002, FR-005, FR-006, FR-013)
- SC-002: A commit-pinned dependency always resolves to the exact same commit across installs, regardless of repository changes. (FR-005, FR-008, FR-010)
- SC-003: A branch-tracked dependency can be updated to the latest branch HEAD via `craft update`, with the pinfile reflecting the new commit. (FR-006, FR-009)
- SC-004: When the same package appears with conflicting ref types, the user receives a clear error before any resolution proceeds. (FR-012)
- SC-005: Non-tagged dependencies are surfaced with warnings at `craft add` and `craft validate` time, communicating weaker guarantees. (FR-014, FR-015)
- SC-006: Existing tagged dependency workflows (add, install, update, validate) are completely unaffected. (FR-003, FR-011)
- SC-007: The pinfile records the provenance of each dependency via `ref_type`, enabling tooling to distinguish between tag, commit, and branch pins. (FR-007, FR-008)

## Assumptions

- The minimum commit SHA length accepted is 7 characters, matching git's default short SHA. Craft resolves short SHAs to full 40-character SHAs via the fetcher before storage.
- Branch names are always specified with the `branch:` prefix (e.g., `branch:main`). Bare non-hex strings without the prefix are not auto-detected as branches.
- A bare hex string of 7+ characters without a `branch:` prefix is always treated as a commit SHA.
- The `ref_type` field defaults to `tag` for backward compatibility with existing pinfiles that lack it.
- Non-tagged dependencies receive the same integrity verification as tagged dependencies (SHA-256 digest per install).
- Transitive non-tagged dependencies receive the same warning level as direct non-tagged dependencies.

## Scope

In Scope:
- Extended dependency URL parsing for commit SHA and branch refs
- Ref-type-aware resolution (commit verification, branch HEAD lookup)
- Pinfile `ref_type` metadata field
- Ref-type-specific update behavior (branch: re-resolve, commit: skip, tag: unchanged)
- Ref-type conflict detection during resolution
- Warning system for `craft add` and `craft validate`
- `craft add` support for non-tagged refs with auto-detection and validation
- Integrity verification for non-tagged deps

Out of Scope:
- `craft init` wizard integration for non-tagged refs (tracked separately as a GitHub issue)
- Automatic migration from non-tagged to tagged refs when a repo adds tags
- Default branch inference when no ref is provided
- Support for arbitrary git refs beyond branches and commits (e.g., `refs/notes/`, `refs/stash`)
- Monorepo or subdirectory scoping (already handled by auto-discovery)

## Dependencies

- Existing `depurl` parsing module
- Existing `fetcher` module with `ResolveRef` capability
- Existing pinfile format and types
- Existing `craft add`, `craft install`, `craft update`, and `craft validate` commands
- Existing MVS resolution pipeline (for tag deps, unchanged)

## Risks & Mitigations

- **Resolver complexity increase**: Adding ref-type branching to the resolver increases code complexity. Mitigation: Clear RefType-based routing with exhaustive tests for each path.
- **"Works on my machine" with branch deps**: Branch-tracked deps can resolve to different commits at different times. Mitigation: Pinfile locks to exact commit SHA; warnings communicate the tradeoff; `craft update` is the explicit upgrade path.
- **Pinfile format change**: Adding `ref_type` field changes the pinfile schema. Mitigation: Default to `tag` for backward compatibility; existing pinfiles work without modification.
- **Short SHA collisions**: A 7-character SHA prefix could theoretically collide in large repos. Mitigation: Fetcher resolves to full 40-char SHA; if ambiguous, the fetcher/git will error naturally.

## References

- WorkShaping: .paw/work/non-tagged-deps/WorkShaping.md
