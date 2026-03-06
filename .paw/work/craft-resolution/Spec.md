# Feature Specification: Craft Resolution Engine

**Branch**: feature/craft-resolution  |  **Created**: 2026-03-06  |  **Status**: Draft
**Input Brief**: Workflow 2 of 3 — add network-capable dependency resolution, caching, authentication, and installation to the craft CLI.

## Overview

The craft CLI currently operates as a local-only tool: it can create and validate manifests, but cannot fetch, resolve, or install dependencies from remote git repositories. The Resolution Engine transforms craft from a validation tool into a fully functional package manager by adding the ability to read dependency declarations from `craft.yaml`, fetch them from git repositories, resolve version constraints using Minimum Version Selection, and install skill directories to the user's AI agent of choice.

When a skill author runs `craft install`, the tool reads the manifest, fetches each dependency via go-git (using SSH keys or environment tokens for private repos), recursively resolves transitive dependencies, detects cycles and name collisions across the full dependency tree, computes integrity digests, writes a deterministic `craft.pin.yaml`, and copies skill directories into the appropriate agent path. A global cache at `~/.craft/cache/` avoids redundant network fetches across projects.

The `craft update` command complements install by re-resolving dependencies to their latest available semver tags, giving users a simple way to pick up new versions without manually editing URLs. Both commands support a `--target` flag to override the default agent installation path.

This workflow delivers the core value proposition of craft: reproducible, one-command installation of skill dependencies from git repositories, with integrity verification and intelligent caching.

## Objectives

- Enable users to install skill dependencies from remote git repositories with a single command
- Provide deterministic, reproducible dependency resolution using Minimum Version Selection (MVS)
- Cache fetched repositories globally to avoid redundant network operations
- Support private repository access via SSH keys and environment tokens
- Auto-detect the user's AI agent and install skills to the correct path
- Detect and report dependency conflicts, cycles, and skill name collisions clearly
- Allow updating dependencies to latest available versions
- Maintain the single-binary, zero-external-dependency promise (pure Go via go-git)

## User Scenarios & Testing

### User Story P1 – Install Dependencies

Narrative: A skill author has declared dependencies in `craft.yaml` and wants to install them. They run `craft install` and expect all dependencies to be fetched, resolved, pinned, and installed to their agent's skill directory automatically.

Independent Test: Running `craft install` in a directory with a valid `craft.yaml` containing dependencies produces a `craft.pin.yaml` and copies skill directories to the detected agent path.

Acceptance Scenarios:
1. Given a `craft.yaml` with two dependencies pointing to public repos, When the user runs `craft install`, Then both dependencies are fetched, resolved, pinned in `craft.pin.yaml`, and their skill directories are installed to the detected agent path
2. Given a `craft.yaml` with dependencies and an existing `craft.pin.yaml` that matches, When the user runs `craft install`, Then pinned versions are used without re-resolving from network, and skills are installed from cache
3. Given a `craft.yaml` where one dependency was added since the last `craft.pin.yaml`, When the user runs `craft install`, Then only the new dependency is resolved from network; existing pinned entries are preserved
4. Given a `craft.yaml` with a dependency that has transitive dependencies, When the user runs `craft install`, Then all transitive dependencies are resolved, pinned, and installed

### User Story P2 – Dependency Resolution with Conflict Detection

Narrative: A skill author has dependencies with overlapping transitive dependencies at different versions. They expect craft to select the correct version using MVS and report any unresolvable conflicts clearly.

Independent Test: Running `craft install` with dependencies that share a transitive dependency at different versions selects the minimum version satisfying all constraints.

Acceptance Scenarios:
1. Given package A depends on C@v1.0.0 and package B depends on C@v1.2.0, When the user runs `craft install`, Then C@v1.2.0 is selected (minimum version satisfying both)
2. Given a dependency graph containing a cycle (A→B→A), When the user runs `craft install`, Then the tool exits with an error listing the full cycle path
3. Given two dependencies that export a skill with the same name, When the user runs `craft install`, Then the tool exits with an error listing both sources and suggesting resolution

### User Story P3 – Private Repository Access

Narrative: A skill author has dependencies in private repositories. They configure authentication via SSH keys or environment tokens and expect `craft install` to fetch them transparently.

Independent Test: Running `craft install` with `GITHUB_TOKEN` set fetches a dependency from a private repository.

Acceptance Scenarios:
1. Given a private dependency and `GITHUB_TOKEN` set in the environment, When the user runs `craft install`, Then the dependency is fetched using token authentication via HTTPS
2. Given a private dependency and an SSH key available via ssh-agent, When the user runs `craft install`, Then the dependency is fetched using SSH authentication
3. Given a private dependency with no authentication configured, When the user runs `craft install`, Then the tool exits with a clear error suggesting authentication methods

### User Story P4 – Update Dependencies

Narrative: A skill author wants to update their dependencies to pick up new versions. They run `craft update` to re-resolve to the latest available semver tags.

Independent Test: Running `craft update` when a dependency has a newer tag available updates the pinfile and re-installs.

Acceptance Scenarios:
1. Given a dependency pinned at v1.0.0 and v1.1.0 available in the remote, When the user runs `craft update`, Then the dependency is re-resolved to v1.1.0, `craft.yaml` is updated to reference `@v1.1.0`, pinfile is updated, and skills are re-installed
2. Given a specific dependency name, When the user runs `craft update <dep>`, Then only that dependency and its transitive closure are updated; other direct dependencies and their transitive closures remain at their pinned versions
3. Given all dependencies already at latest versions, When the user runs `craft update`, Then the manifest and pinfile remain unchanged and the tool reports "all dependencies up to date"

### User Story P5 – Agent-Aware Installation

Narrative: A skill author using Claude Code wants skills installed to `~/.claude/skills/` automatically, while a Copilot user expects `~/.copilot/skills/`. A user who wants a custom path uses `--target`.

Independent Test: Running `craft install` without `--target` installs skills to the detected agent's default path.

Acceptance Scenarios:
1. Given Claude Code is detected (e.g., `~/.claude/` exists), When the user runs `craft install`, Then skills are installed to `~/.claude/skills/<skill-name>/`
2. Given no known agent is detected, When the user runs `craft install`, Then the tool exits with an error suggesting `--target <path>`
3. Given `--target /custom/path`, When the user runs `craft install --target /custom/path`, Then skills are installed to `/custom/path/<skill-name>/`
4. Given both `~/.claude/` and `~/.copilot/` exist, When the user runs `craft install`, Then Claude Code takes precedence and skills are installed to `~/.claude/skills/<skill-name>/`

### User Story P6 – Global Cache

Narrative: A skill author works on multiple projects that share dependencies. They expect the second `craft install` to be fast because repositories are cached globally.

Independent Test: A second `craft install` for the same dependency in a different project directory uses the cached repository without network access.

Acceptance Scenarios:
1. Given a dependency was previously fetched and cached at `~/.craft/cache/`, When the user runs `craft install` in another project with the same dependency, Then the cached content is used without a network fetch
2. Given the cache does not contain the requested dependency, When the user runs `craft install`, Then the dependency is fetched from the network and stored in the cache
3. Given the network is unavailable but the dependency is cached, When the user runs `craft install`, Then the cached content is used and installation succeeds

### User Story P7 – Auto-Discovery for Dependencies Without Manifest

Narrative: A dependency repository does not have a `craft.yaml` but contains skill directories with `SKILL.md` files. The user expects craft to discover and install those skills automatically.

Independent Test: Running `craft install` with a dependency that has no `craft.yaml` discovers all `SKILL.md` files and treats their parent directories as skills.

Acceptance Scenarios:
1. Given a dependency repo with no `craft.yaml` but three directories containing `SKILL.md`, When the user runs `craft install`, Then all three skills are discovered, pinned, and installed
2. Given a dependency repo with no `craft.yaml` and no `SKILL.md` files, When the user runs `craft install`, Then the tool reports a warning that the dependency exports no skills

### Edge Cases

- Dependency URL points to a non-existent repository: error with "repository not found" and authentication hint
- Dependency version tag does not exist: error listing available tags from the remote
- Monorepo URL with subpath (e.g., `repo/path@v1`): error with "monorepo paths not yet supported"
- `craft.pin.yaml` exists but does not match `craft.yaml`: auto-re-resolve changed dependencies with a warning (no interactive prompt — CI-friendly)
- SKILL.md in a dependency has invalid frontmatter: error with specific field and source package info
- Network unavailable and dependency not cached: error with clear message suggesting cache or network fix
- Dependency repo is empty or has no commits: error with descriptive message
- Integrity digest mismatch on cached content: re-fetch from network, warn about potential corruption
- Empty dependencies map in `craft.yaml`: clean exit with "no dependencies to install" message
- Two concurrent `craft install` processes writing to the same cache: handled via atomic writes to prevent corruption

## Requirements

### Functional Requirements

- FR-001: Fetch dependency source code from git repositories using go-git (Stories: P1, P3)
- FR-002: Resolve dependency version refs to specific git commit SHAs (Stories: P1, P2)
- FR-003: Recursively resolve transitive dependencies by reading each dependency's `craft.yaml` (Stories: P1, P2)
- FR-004: Implement Minimum Version Selection — when multiple versions of the same dependency are required, select the minimum version satisfying all constraints (Stories: P2)
- FR-005: Detect circular dependencies and report the full cycle path (Stories: P2)
- FR-006: Detect skill name collisions across the full dependency tree and report both conflicting sources (Stories: P2)
- FR-007: Compute SHA-256 integrity digests of resolved skill content — the digest covers the concatenated contents of all files within all skill directories exported by the dependency, with file paths relative to the repository root, sorted lexicographically (Stories: P1)
- FR-008: Write resolved dependencies (both direct and transitive) to `craft.pin.yaml` using existing pinfile types, with transitive entries distinguished by a `source` field indicating the parent dependency that declared them (Stories: P1)
- FR-009: Read existing `craft.pin.yaml` and skip re-resolution for dependencies whose manifest entries have not changed (Stories: P1, P6)
- FR-010: Copy resolved skill directories to the target installation path as `<target>/<skill-name>/` (Stories: P1, P5)
- FR-011: Auto-detect the user's AI agent by checking for known directory markers — `~/.claude/` for Claude Code (installs to `~/.claude/skills/`), `~/.copilot/` for GitHub Copilot (installs to `~/.copilot/skills/`). When multiple agents are detected, prefer Claude Code (first match in priority order). (Stories: P5)
- FR-012: Support `--target <path>` flag on `craft install` and `craft update` to override auto-detected agent path (Stories: P5)
- FR-013: Authenticate to private repositories using SSH keys (via ssh-agent) (Stories: P3)
- FR-014: Authenticate to private repositories using environment tokens (`GITHUB_TOKEN`, `CRAFT_TOKEN`) via HTTPS — both tokens follow the same code path, `CRAFT_TOKEN` takes precedence if both are set (Stories: P3)
- FR-015: Cache fetched git repositories at `~/.craft/cache/` as bare clones keyed by repository URL — a single cache entry per repo serves all versions (Stories: P6)
- FR-016: Use cached repositories when available, falling back to network fetch only when cache misses; serve as offline fallback when network is unavailable (Stories: P6)
- FR-021: Verify integrity of cached content on read; automatically re-fetch from network on integrity mismatch (Stories: P6)
- FR-017: Auto-discover skills in dependency repos without `craft.yaml` by recursively scanning for `SKILL.md` files, treating each parent directory as a skill (Stories: P7)
- FR-018: Re-resolve dependencies to the latest available semver tag via `craft update`; update `craft.yaml` dependency URLs to reflect the new version tags (manifest mutation, analogous to Go's `go get -u` modifying `go.mod`) (Stories: P4)
- FR-019: Support selective update of a single dependency via `craft update <alias>` — re-resolve the targeted dependency and its transitive closure; preserve all other pinned entries unchanged (Stories: P4)
- FR-020: Register `install` and `update` subcommands in the existing Cobra CLI (Stories: P1, P4)

### Key Entities

- **ResolvedDependency**: A dependency with its resolved commit SHA, integrity digest, discovered skills, and source URL
- **DependencyGraph**: Directed graph of package dependencies used for cycle detection and MVS
- **CacheEntry**: A cached git repository identified by URL, stored at `~/.craft/cache/`
- **AgentType**: Enumeration of supported AI agents (ClaudeCode, Copilot, Unknown)

### Cross-Cutting / Non-Functional

- All git operations use go-git (pure Go) — no shelling out to system git
- Network errors are retried with clear messaging; cache serves as offline fallback
- All file writes use atomic temp-file-then-rename pattern (established in Workflow 1)
- Errors include actionable suggestions (e.g., "is this a private repo? Set GITHUB_TOKEN")

## Success Criteria

- SC-001: Running `craft install` on a manifest with public dependencies produces a valid `craft.pin.yaml` and installs skills to the correct agent path (FR-001, FR-002, FR-008, FR-010, FR-011)
- SC-002: Transitive dependencies are fully resolved — a dependency's own dependencies are fetched and installed (FR-003)
- SC-003: When two paths require different versions of the same dependency, MVS selects the minimum satisfying version (FR-004)
- SC-004: Circular dependencies produce a clear error listing the cycle path (FR-005)
- SC-005: Skill name collisions across transitive deps produce a clear error listing both sources (FR-006)
- SC-006: Integrity digests in `craft.pin.yaml` match SHA-256 of resolved skill content (FR-007)
- SC-007: Running `craft install` twice with no manifest changes does not re-fetch from network (FR-009, FR-015)
- SC-008: Private repos are accessible with `GITHUB_TOKEN` or SSH key authentication (FR-013, FR-014)
- SC-009: `craft update` re-resolves to latest available tag and updates pinfile (FR-018)
- SC-010: `--target /path` overrides auto-detected agent path (FR-012)
- SC-011: Dependencies without `craft.yaml` have their skills auto-discovered via SKILL.md scanning (FR-017)
- SC-012: All resolution logic is testable via interfaces — unit tests use mocked git operations (FR-001 through FR-021)
- SC-013: When network is unavailable, cached dependencies are used successfully for installation (FR-016)
- SC-014: `craft update <alias>` updates only the specified dependency; others remain at pinned versions (FR-019)
- SC-015: Cached content is integrity-verified on read; corrupted cache entries trigger automatic re-fetch (FR-021)

## Assumptions

- Go-git supports all authentication methods needed for typical GitHub/GitLab repos (SSH via ssh-agent, HTTPS via token). Known limitations (no ProxyJump, no hardware tokens) are documented but accepted for MVP.
- Agent detection relies on directory existence (`~/.claude/` for Claude Code, `~/.copilot/` for Copilot). This is a heuristic that may need refinement but is sufficient for MVP.
- The cache directory `~/.craft/cache/` is writable by the current user. Repositories are cached as bare clones keyed by URL (one entry per repo, serving all versions). No cache eviction or `craft cache clean` is included (deferred to post-MVP).
- Dependency URLs always point to the repository root. Monorepo subpath support is explicitly deferred.
- `craft update` updates to the latest available semver tag (not constraint-based ranges). This matches Go's `go get -u` semantics.
- The integrity digest format follows the RFC: `sha256-<base64>` where the hash covers concatenated contents of all files in the dependency's skill directories, sorted by path.

## Scope

In Scope:
- go-git clone, fetch, tag listing, and file reading at specific commits
- Global cache at `~/.craft/cache/` with store and lookup
- SSH and token-based authentication for private repos
- MVS dependency resolution with recursive transitive resolution
- Dependency graph construction, cycle detection, version conflict resolution
- Skill name collision detection across full dependency tree
- Auto-discovery for dependencies without `craft.yaml`
- Agent detection (Claude Code, Copilot) and flat skill installation
- `craft install` command (full resolve → pin → install pipeline)
- `craft update [dep]` command (re-resolve to latest tags)
- `--target <path>` flag override
- Unit tests with mocked git interfaces and integration tests with fixture repos

Out of Scope:
- `craft add`, `craft remove` commands (Workflow 3)
- npm-style progress bars and dependency tree visualization (Workflow 3)
- `--project` flag for project-local installation (Workflow 3)
- Cache eviction / `craft cache clean` (post-MVP)
- Monorepo subpath support (post-MVP)
- OCI artifact transport (post-MVP)
- Shell-out to system git as fallback (post-MVP)
- Central registry or discovery (post-MVP)

## Dependencies

- Existing Workflow 1 types: `manifest.Manifest`, `pinfile.Pinfile`, `pinfile.ResolvedEntry`, `skill.Frontmatter`
- Existing Workflow 1 parsers: `manifest.ParseFile`, `pinfile.ParseFile`, `skill.ParseFrontmatter`
- Existing Workflow 1 validation: `validate.Runner`, `manifest.Validate`, `pinfile.Validate`
- Existing Cobra CLI skeleton with root, init, validate, version commands
- New external dependency: `github.com/go-git/go-git/v5` (pure Go git implementation)
- New external dependency: `golang.org/x/crypto/ssh` (SSH authentication support, pulled in by go-git)

## Risks & Mitigations

- **go-git SSH limitations**: go-git lacks ProxyJump, hardware token, and agent forwarding support. Mitigation: document limitations prominently; accept for MVP; plan `--git-cli` fallback post-MVP.
- **go-git error messages**: Authentication failures from go-git can be opaque. Mitigation: wrap errors with actionable context ("authentication failed — is GITHUB_TOKEN set? Is the repo private?").
- **Large dependency trees**: Deep transitive resolution could be slow or hit rate limits. Mitigation: caching reduces repeat fetches; MVP targets small-to-medium dependency trees.
- **Cache corruption**: Interrupted downloads could leave partial cache entries. Mitigation: use atomic writes for cache storage; verify integrity on read.
- **Concurrent cache access**: Multiple `craft install` processes in CI could race on cache writes. Mitigation: use atomic temp-file-then-rename for cache operations; reads are safe on immutable committed content.
- **Agent path heuristic fragility**: Agent paths may change between versions. Mitigation: `--target` flag provides escape hatch; document known paths.

## References

- WorkShaping: `.paw/work/WorkShaping.md`
- Workflow 1 (Foundation): `.paw/work/craft-foundation/`
- Agent Skills specification: https://agentskills.io/specification
- RFC Discussion: https://github.com/agentskills/agentskills/discussions/210
