# Craft Resolution Engine — Implementation Plan

## Overview

Implement the Resolution Engine for craft — the dependency resolution, fetching, caching, and installation layer that transforms craft from a local validation tool into a fully functional package manager. This adds `craft install` and `craft update` commands backed by go-git, a global cache, SSH/token authentication, MVS resolution, and agent-aware skill installation.

## Current State Analysis

Workflow 1 established:
- **Types**: `manifest.Manifest` with `Dependencies map[string]string`, `pinfile.Pinfile` with `Resolved map[string]ResolvedEntry`, `skill.Frontmatter` — all with parsers and validators (`internal/manifest/`, `internal/pinfile/`, `internal/skill/`)
- **CLI skeleton**: Cobra root with init/validate/version commands (`internal/cli/root.go:18-22`)
- **Validation runner**: Multi-phase local validation with structured errors (`internal/validate/runner.go`)
- **Skill discovery**: Recursive SKILL.md directory scanner (`internal/init/discover.go:25-75`)
- **Patterns**: Atomic file writes (temp+rename), `io.Reader`/`io.Writer` interfaces for parsers, sorted map keys for determinism, same-package white-box tests

Key gaps the Resolution Engine fills:
- No dep URL parsing (validated by regex only, `internal/manifest/validate.go:19`)
- No pinfile write function (only read/validate exist)
- No git fetching, caching, or authentication
- No resolution algorithm, graph construction, or cycle detection
- No agent detection or skill installation
- No `install` or `update` CLI commands

## Desired End State

Running `craft install` in a directory with a valid `craft.yaml` containing dependencies:
1. Parses the manifest and reads any existing pinfile
2. For each new/changed dependency, fetches the git repo (using cache when available)
3. Recursively resolves transitive dependencies via MVS
4. Detects cycles, version conflicts, and skill name collisions
5. Computes integrity digests for all resolved dependencies
6. Writes a deterministic `craft.pin.yaml`
7. Installs skill directories to the detected agent path (or `--target` override)

Running `craft update [dep]` re-resolves to the latest available semver tags.

**Verification approach**: Unit tests with mocked git interfaces for resolution logic; `go test ./...` passes; `go build` succeeds; manual verification of install/update commands against test fixtures.

## What We're NOT Doing

- `craft add` / `craft remove` commands (Workflow 3)
- npm-style progress bars or dependency tree visualization (Workflow 3)
- `--project` flag for project-local installation (Workflow 3)
- Cache eviction / `craft cache clean` (post-MVP)
- Monorepo subpath support (post-MVP)
- OCI artifact transport (post-MVP)
- Shell-out to system git as fallback (post-MVP)
- Central registry or search/discovery (post-MVP)
- Modifying existing Workflow 1 types or validation logic

**Note**: Existing packages may be extended with new files (e.g., `pinfile/write.go`) and types may receive additive fields (e.g., `ResolvedEntry.Source` for transitive dep provenance). No breaking changes to existing APIs.

## Phase Status

- [x] **Phase 1: Foundation Types & Utilities** - Dep URL parser, pinfile writer, integrity digests, agent detection
- [x] **Phase 2: Git Fetching Layer** - go-git bare clone, cache, SSH/token auth behind GitFetcher interface
- [x] **Phase 3: Resolution Engine** - MVS algorithm, dependency graph, cycle detection, collision detection, auto-discovery
- [x] **Phase 4: Commands & Integration** - `craft install`, `craft update`, `--target` flag, end-to-end wiring
- [x] **Phase 5: Documentation** - Docs.md, README updates

## Phase Candidates

_(none — all scope items are assigned to phases)_

---

## Phase 1: Foundation Types & Utilities

Build the foundational types and utility functions that Phases 2–4 depend on. These are pure, self-contained components with no external dependencies (no go-git yet).

### Changes Required

- **`internal/resolve/depurl.go`** (new): Dependency URL parser — extract host, org, repo, and version from `host/org/repo@vMAJOR.MINOR.PATCH` format. Produce git clone URLs for both HTTPS (`https://host/org/repo.git`) and SSH (`git@host:org/repo.git`). Reuse the validated regex pattern from `internal/manifest/validate.go:19`.
- **`internal/resolve/depurl_test.go`** (new): Table-driven tests covering valid URLs, edge cases (dots in org/repo names, multi-segment hosts), and invalid inputs.
- **`internal/resolve/types.go`** (new): Core resolution types — `ResolvedDep` (URL, alias, commit SHA, integrity, discovered skill names, source package URL for provenance, list of skill directory paths), `GraphNode` (package identity + edges), `AgentType` enum (ClaudeCode, Copilot, Unknown). Include a `PackageIdentity() string` method on the dep URL type that returns `host/org/repo` without version — used by MVS to identify same-package entries at different versions.
- **`internal/pinfile/write.go`** (new): Serialize `Pinfile` to YAML with deterministic field ordering (pin_version first, resolved entries sorted by URL key). Follow the same yaml.Node pattern established in `internal/manifest/write.go:14-47`.
- **`internal/pinfile/write_test.go`** (new): Round-trip test (write → parse → compare), deterministic ordering test, error path test with errWriter.
- **`internal/integrity/digest.go`** (new): Compute SHA-256 integrity digest from a set of file paths and contents. Accept `map[string][]byte` (path → content), sort by path, concatenate, hash, return `sha256-<base64>` string. Follow the RFC specification.
- **`internal/integrity/digest_test.go`** (new): Known-input/known-output tests, empty input test, ordering determinism test.
- **`internal/agent/detect.go`** (new): Agent detection — check `~/.claude/` and `~/.copilot/` directory existence with deterministic precedence (Claude Code first, then Copilot). Return `AgentType` and default install path. When multiple agents detected, use first match. Error with detected-agent listing when none found. Accept a home directory parameter for testability.
- **`internal/agent/detect_test.go`** (new): Tests with temp directories simulating agent markers, including multi-agent precedence and no-agent error.

### Success Criteria

#### Automated Verification:
- [ ] Tests pass: `go test ./internal/resolve/... ./internal/pinfile/... ./internal/integrity/... ./internal/agent/...`
- [ ] Build succeeds: `go build ./...`

#### Manual Verification:
- [ ] Dep URL parser correctly decomposes `github.com/example/skills@v1.0.0` into HTTPS and SSH clone URLs
- [ ] Pinfile write produces YAML matching the format in `testdata/pinfiles/valid.yaml`
- [ ] Integrity digest produces stable output for identical inputs

---

## Phase 2: Git Fetching Layer

Implement git operations behind a `GitFetcher` interface, with a go-git implementation backed by a global cache. This phase introduces the first external dependency (go-git).

### Changes Required

- **`go.mod`**: Add `github.com/go-git/go-git/v5` dependency. Run `go mod tidy` to pull transitive deps.
- **`internal/fetch/fetcher.go`** (new): Define `GitFetcher` interface with three methods:
  - `ResolveRef(url, ref string) (commitSHA string, err error)` — resolve a tag/branch to a commit
  - `ListTags(url string) ([]string, error)` — list available tags from remote
  - `ListTree(url, commitSHA string) ([]string, error)` — list all file paths in the repo tree at a specific commit (needed for auto-discovery of SKILL.md in repos without craft.yaml)
  - `ReadFiles(url, commitSHA string, paths []string) (map[string][]byte, error)` — read file contents at a specific commit
  
  Implement `GoGitFetcher` struct satisfying this interface using go-git's bare clone and in-memory worktree operations. The fetcher coordinates with cache (check cache first, clone/fetch on miss, store result).
- **`internal/fetch/cache.go`** (new): Global cache at `~/.craft/cache/`. Cache bare git repos keyed by a sanitized version of the repository URL (e.g., `github.com/org/repo` → `github.com-org-repo/`). On cache hit, open existing bare repo and fetch latest; on miss, bare clone. Atomic directory operations (clone to temp within cache root for same-filesystem guarantee, rename on success, deferred cleanup on failure). Accept a configurable cache root for testability. Include integrity verification on read: after checking out files for a resolved dependency, verify computed digest against the expected pinfile digest; on mismatch, invalidate the cache entry, re-fetch from network, and warn about potential corruption. Define explicit failure matrix: (cache valid → use), (cache corrupted → re-fetch + warn), (cache miss + online → fetch + store), (cache miss + offline → error with suggestion).
- **`internal/fetch/auth.go`** (new): Authentication provider — check `CRAFT_TOKEN` (highest priority), then `GITHUB_TOKEN`, then SSH agent. Return appropriate go-git transport auth object. Clear error messages when auth fails ("authentication failed — set GITHUB_TOKEN or configure SSH keys").
- **`internal/fetch/fetcher_test.go`** (new): Unit tests using the `GitFetcher` interface with a mock implementation for resolution tests. Integration-style test that clones a known public repo (small fixture repo) — guarded by build tag or env var to skip in CI without network.
- **`internal/fetch/cache_test.go`** (new): Tests with temp cache directories — store, lookup, miss, concurrent-safe atomic writes, integrity verification (valid cache, corrupted cache triggers re-fetch), offline fallback (cache hit succeeds, cache miss errors).
- **`internal/fetch/auth_test.go`** (new): Tests for token precedence (CRAFT_TOKEN > GITHUB_TOKEN), SSH fallback, no-auth error message.

### Success Criteria

#### Automated Verification:
- [ ] Tests pass: `go test ./internal/fetch/...`
- [ ] Build succeeds: `go build ./...`

#### Manual Verification:
- [ ] `GoGitFetcher.ResolveRef` resolves a tag from a public GitHub repo to a commit SHA
- [ ] Cache stores bare repo on first fetch, reuses on second
- [ ] Token auth constructs correct HTTPS basic auth credentials

---

## Phase 3: Resolution Engine

Implement the MVS dependency resolution algorithm, dependency graph with cycle detection, transitive resolution, auto-discovery, and skill name collision detection. This is the core algorithmic phase.

### Changes Required

- **`internal/resolve/graph.go`** (new): Dependency graph data structure — directed graph with nodes (package URL) and edges (dependency relationships). Methods: `AddNode`, `AddEdge`, `DetectCycles` (DFS-based cycle detection returning the full cycle path as `[]string`). Topological sort for deterministic resolution order.
- **`internal/resolve/graph_test.go`** (new): Tests for acyclic graph, single cycle, nested cycles, diamond dependencies, self-loop.
- **`internal/resolve/resolver.go`** (new): Resolution orchestrator implementing the MVS algorithm.
  - Accept `GitFetcher` interface (from Phase 2), root `Manifest`, and optional existing `Pinfile`
  - **Pinfile reuse policy**: `craft install` may short-circuit via pinfile reuse when manifest entry is unchanged; `craft update` always bypasses reuse for targeted dependencies (or all for full update)
  - For each dependency in manifest: check if pinfile has a matching entry with unchanged URL (skip re-resolution for install); otherwise resolve ref → commit via fetcher
  - Recursively resolve transitive deps: read dependency's `craft.yaml` (via `ReadFiles`), parse with `manifest.Parse`, recurse
  - Build dependency graph; run cycle detection before proceeding
  - Apply MVS: use `PackageIdentity()` from depurl parser to identify same-package entries at different versions; select the highest (minimum version satisfying all constraints)
  - Auto-discover skills: if dependency has no `craft.yaml`, use `ListTree` to find SKILL.md files, then `ReadFiles` to read their content
  - Detect skill name collisions across all resolved packages — error messages include provenance (source URL, commit, skill path)
  - Compute integrity digests for each resolved dependency (all files in all skill directories, sorted by relative path)
  - **Selective update boundary**: when updating a single alias, re-resolve only that dependency and its transitive closure; preserve all other direct deps and their transitive closures from existing pinfile
  - Write transitive deps to pinfile using their own dep URL as key, with `Source` field indicating the parent that declared them
  - Return `[]ResolvedDep` and assembled `Pinfile`
- **`internal/resolve/discover.go`** (new): Skill discovery for in-memory file trees from git. Accept file listing (from `ListTree`) and file contents (from `ReadFiles`), identify directories containing `SKILL.md`, parse frontmatter from content bytes using `skill.ParseFrontmatter(io.Reader)`. Filter `ListTree` output for paths ending in `SKILL.md`, then read those files.
- **`internal/resolve/discover_test.go`** (new): Tests with in-memory file maps simulating repos with/without craft.yaml, with/without SKILL.md files.
- **`internal/resolve/resolver_test.go`** (new): Comprehensive tests using mock `GitFetcher`:
  - Simple single-dependency resolution
  - Transitive dependency (A → B → C)
  - Diamond dependency (A → B, A → C, B → D@v1, C → D@v2 → MVS picks v2)
  - Cycle detection (A → B → A)
  - Skill name collision across packages
  - Auto-discovery for deps without manifest
  - Pinfile reuse (existing pinned entry skipped)
  - Empty dependencies (no-op)

### Success Criteria

#### Automated Verification:
- [ ] Tests pass: `go test ./internal/resolve/...`
- [ ] Build succeeds: `go build ./...`

#### Manual Verification:
- [ ] MVS correctly selects highest required version in diamond dependency scenario
- [ ] Cycle detection error message includes full cycle path
- [ ] Collision error lists both conflicting source packages and skill name

---

## Phase 4: Commands & Integration

Wire everything together into the `craft install` and `craft update` CLI commands. This phase creates the end-to-end pipeline and integration tests.

### Changes Required

- **`internal/install/installer.go`** (new): Skill installer — copy resolved skill directories to target path as `<target>/<skill-name>/`. Accept resolved deps (with file contents from git) and target directory. Create directories, write files atomically (temp+rename pattern). Handle the case where target already exists (overwrite with fresh content).
- **`internal/install/installer_test.go`** (new): Tests with temp directories — install creates correct structure, overwrites existing, handles empty skills list.
- **`internal/cli/install.go`** (new): `craft install` Cobra command with `--target` flag.
  - Pipeline: parse manifest → load existing pinfile (if any) → create `GoGitFetcher` with cache and auth → run resolver → write pinfile atomically → detect agent (or use `--target`) → install skills → print summary
  - Error handling: wrap all errors with actionable context
  - Exit with "no dependencies to install" message for empty deps map
- **`internal/cli/update.go`** (new): `craft update [alias]` Cobra command with `--target` flag.
  - If alias provided: re-resolve only that dependency and its transitive closure to latest semver tags via `ListTags`; preserve other pinned entries
  - If no alias: re-resolve all dependencies to latest tags
  - **Manifest mutation**: update `craft.yaml` dependency URLs to reflect the new version tags (using atomic write via existing `manifest.Write` + temp+rename). This preserves manifest-as-truth and keeps the validate runner's consistency check valid.
  - Same pipeline as install for pinfile write and skill installation
  - Report "all dependencies up to date" when nothing changed
- **`internal/cli/root.go`**: Add `rootCmd.AddCommand(installCmd)` and `rootCmd.AddCommand(updateCmd)` in the `init()` function
- **`internal/pinfile/types.go`**: Add `Source string \`yaml:"source,omitempty"\`` field to `ResolvedEntry` — empty for direct dependencies, set to the parent dependency URL for transitive entries
- **`internal/validate/runner.go`**: Update `checkPinfile` consistency check to tolerate transitive pinfile entries (entries with non-empty `Source` field are not required to match manifest dependency URLs)
- **`internal/cli/install_test.go`** (new): Integration tests using test fixtures — mock git fetcher wired into the command, verify pinfile output (both direct and transitive entries) and installed directory structure
- **`internal/cli/update_test.go`** (new): Integration tests — update single dep, update all deps, already-at-latest scenario

### Success Criteria

#### Automated Verification:
- [ ] Tests pass: `go test ./...` (full suite including all new packages)
- [ ] Build succeeds: `go build -o craft ./cmd/craft`
- [ ] Lint clean: `go vet ./...`

#### Manual Verification:
- [ ] `craft install --help` shows usage with `--target` flag
- [ ] `craft update --help` shows usage with optional `[alias]` arg
- [ ] Running `craft install` with no deps prints "no dependencies to install"
- [ ] Running `craft install` in a directory without `craft.yaml` fails with a clear error

---

## Phase 5: Documentation

### Changes Required

- **`.paw/work/craft-resolution/Docs.md`**: Technical reference covering resolution algorithm, cache layout, authentication, agent detection, integrity format (load `paw-docs-guidance` for template)
- **`README.md`**: Update command table to include `install` and `update`. Add dependency declaration and installation examples. Add authentication section (GITHUB_TOKEN, CRAFT_TOKEN, SSH). Add cache section (~/.craft/cache/).

### Success Criteria

- [ ] README accurately documents all 5 commands (init, validate, version, install, update)
- [ ] README includes authentication and cache documentation
- [ ] Docs.md provides complete technical reference for the resolution engine

---

## References

- Issue: none
- Spec: `.paw/work/craft-resolution/Spec.md`
- Research: `.paw/work/craft-resolution/CodeResearch.md`
- WorkShaping: `.paw/work/WorkShaping.md`
