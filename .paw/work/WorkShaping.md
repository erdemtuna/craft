# WorkShaping: craft — Agent Skills Package Manager

## Problem Statement

The [Agent Skills specification](https://agentskills.io/specification) defines how to **write** skills (a directory with a `SKILL.md` file) but provides no standard for **distributing, versioning, or declaring dependencies between** skills across repositories. Skill authors who build on skills from other repos are forced to vendor code, document dependencies in READMEs, or hope users manually install the right versions.

[Discussion #210](https://github.com/agentskills/agentskills/discussions/210) proposes an RFC for a manifest + pinfile system — analogous to Go modules — to solve this. **craft** is a standalone CLI tool that implements this RFC, using YAML as the serialization format for ecosystem consistency with `SKILL.md` frontmatter.

### Who benefits

- **Skill authors** who publish packages with cross-repo dependencies — they can declare deps formally instead of writing install instructions
- **Skill consumers** who install packages — they get reproducible, one-command installation with integrity verification
- Both roles are often the same person

## Work Breakdown

### Core Functionality

1. **`craft init`** — Interactive setup wizard that walks users through creating a `craft.yaml` manifest. Prompts for package name, version, description, license, and skill directory paths. Infers sensible defaults (name from directory, version 0.1.0).

2. **`craft install`** — The heart of the tool. Reads `craft.yaml`, resolves all dependencies (recursively), applies minimum version selection, detects cycles and name collisions, writes `craft.pin.yaml`, and installs skill directories to the appropriate agent path.

3. **`craft add <dep>`** — Adds a dependency to `craft.yaml`. Accepts git URL format: `github.com/org/repo@v1.0.0`. Resolves the dependency to verify it exists and has valid skills, updates `craft.yaml`, and optionally runs install.

4. **`craft remove <dep>`** — Removes a dependency from `craft.yaml` and cleans up installed skills that are no longer needed.

5. **`craft update [dep]`** — Updates all dependencies (or a specific one) to the latest compatible version. Rewrites `craft.pin.yaml` with new commits and integrity digests.

6. **`craft validate`** — Comprehensive pre-flight validation:
   - `craft.yaml` schema correctness
   - All skill paths in `skills[]` contain valid `SKILL.md` files
   - SKILL.md frontmatter validation (name format, required fields)
   - Dependency URL format validity
   - `craft.pin.yaml` integrity verification (if pinfile exists)
   - Skill name collision detection across the full dependency tree

### Supporting Functionality

7. **Agent-aware installation** — Auto-detects the user's AI agent and installs skills to the correct path:
   - Claude Code: `~/.claude/skills/`
   - GitHub Copilot: `~/.copilot/skills/` (or plugin structure)
   - Override with `--target <path>` flag

8. **Global cache** — Caches fetched git repositories at `~/.craft/cache/` to avoid redundant downloads across projects. Keyed by git URL + commit SHA.

9. **Private repo auth** — Supports SSH keys (via ssh-agent) and environment variable tokens (`GITHUB_TOKEN`, `CRAFT_TOKEN`) for accessing private repositories.

10. **npm-style output** — Progress indicators during fetch/resolve, dependency tree visualization after install, clear error messages with actionable suggestions.

## Dependency Resolution Algorithm

Implements **Minimum Version Selection (MVS)**, identical to Go modules:

1. Read `craft.yaml` from the package root
2. For each dependency, resolve git URL + ref to a specific commit via go-git
3. Fetch the dependency's `craft.yaml` (if it exists) and recursively resolve its dependencies
4. If a dependency has no `craft.yaml`, auto-discover all directories containing `SKILL.md` as exported skills
5. Detect cycles — abort with error listing the cycle path
6. Detect version conflicts — for semver tags, apply MVS (pick the minimum version satisfying all constraints); for conflicting commit SHAs, error with guidance
7. Detect skill name collisions — if two packages export a skill with the same name, abort listing both sources
8. Write `craft.pin.yaml` with resolved commits and SHA-256 integrity digests
9. Install skill directories flat into the agent's skill path

## Artifacts

### `craft.yaml` — Package Manifest

The manifest uses YAML for consistency with the SKILL.md ecosystem (which uses YAML frontmatter) and to support comments (e.g., pinning reasons). The RFC defines `skills.json`; we use `craft.yaml` as a branded, YAML-based alternative — the semantics (fields, resolution, identity) are identical.

```yaml
schema_version: 1
name: code-quality
version: 1.0.0
description: Automated code review and linting skills.
license: MIT

skills:
  - ./skills/lint-check
  - ./skills/review-pr

dependencies:
  git-operations: github.com/example/git-skills@v1.0.0
  style-guides: github.com/other-org/style-skills@v2.3.1  # pinned — v2.4.0 has breaking changes
```

### `craft.pin.yaml` — Pinfile

Also YAML, for ecosystem consistency and to allow human-readable comments when reviewing pinfile diffs in PRs. Machine-generated; not hand-edited.

```yaml
pin_version: 1

resolved:
  github.com/example/git-skills@v1.0.0:
    commit: a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2
    integrity: sha256-Xk9jR2mN5pQ8vW3yB7cF1dA4hL6tS0uE9iO2wR5nM3s=
    skills:
      - git-commit
      - git-branch
      - git-operations

  github.com/other-org/style-skills@v2.3.1:
    commit: f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5
    integrity: sha256-Lm8kP3qN7rT2wX5yA9bD4eG1hJ6oS0uC3iF2vR5nK7s=
    skills:
      - python-style
      - js-style
```

## Edge Cases & Expected Handling

| Edge Case | Handling |
|-----------|----------|
| Dependency has no `craft.yaml` | Auto-discover all `SKILL.md` files recursively, treat each parent dir as a skill |
| Two deps export same skill name | Error with clear message listing both sources and suggestions (exclude one, rename) |
| Circular dependency (A→B→A) | Error listing the full cycle path |
| Dependency URL points to non-existent repo | Error with "repository not found" and auth hint (is it private?) |
| Dependency version tag doesn't exist | Error listing available tags |
| `craft.pin.yaml` exists but doesn't match `craft.yaml` | Warning + prompt to re-resolve |
| Network unavailable during install | Use cache if available; error with clear message if not cached |
| Skill directory in `skills[]` doesn't contain `SKILL.md` | Validation error during `craft validate` or `craft install` |
| `SKILL.md` has invalid frontmatter | Validation error with specific field and line info |
| Monorepo URL with subpath (e.g., `repo/path@v1`) | Not supported in MVP; error with message "monorepo paths not yet supported" |

## Architecture Sketch

```
craft (CLI binary)
├── cmd/                    # CLI command handlers (cobra)
│   ├── init.go
│   ├── install.go
│   ├── add.go
│   ├── remove.go
│   ├── update.go
│   └── validate.go
├── internal/
│   ├── manifest/           # craft.yaml parsing, validation, serialization
│   │   ├── manifest.go     # Types + read/write
│   │   └── schema.go       # Schema validation
│   ├── pinfile/             # craft.pin.yaml parsing, generation, integrity
│   │   ├── pinfile.go       # Types + read/write
│   │   └── integrity.go     # SHA-256 digest computation
│   ├── resolve/             # Dependency resolution engine
│   │   ├── resolver.go      # MVS algorithm
│   │   ├── graph.go         # Dependency graph + cycle detection
│   │   └── collision.go     # Skill name collision detection
│   ├── fetch/               # Git repository fetching
│   │   ├── fetcher.go       # go-git clone/fetch operations
│   │   ├── cache.go         # Global cache management (~/.craft/cache/)
│   │   └── auth.go          # SSH + token auth
│   ├── discover/            # Skill discovery in repos
│   │   ├── discover.go      # Find SKILL.md files, parse frontmatter
│   │   └── skillmd.go       # SKILL.md YAML frontmatter parser
│   ├── install/             # Skill installation to agent paths
│   │   ├── installer.go     # Copy skills to target directory
│   │   └── agents.go        # Agent detection (Claude, Copilot)
│   └── ui/                  # Output formatting
│       ├── progress.go      # Progress bars
│       └── tree.go          # Dependency tree display
├── go.mod
├── go.sum
└── main.go
```

### Key Interfaces (for testability)

```go
// GitFetcher abstracts git operations for testing
type GitFetcher interface {
    ResolveRef(url string, ref string) (commit string, error)
    FetchFiles(url string, commit string, paths []string) (map[string][]byte, error)
    ListTags(url string) ([]string, error)
}

// SkillDiscoverer finds skills in a directory tree
type SkillDiscoverer interface {
    Discover(root fs.FS) ([]Skill, error)
}

// Installer places skills into the target directory
type Installer interface {
    Install(skills []ResolvedSkill, target string) error
    DetectAgent() (AgentType, string, error)
}
```

## Technology Choices

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | Single binary, excellent git ecosystem, Go modules as reference |
| Serialization | YAML (`craft.yaml` + `craft.pin.yaml`) | Consistent with SKILL.md frontmatter, supports comments, human-friendly. RFC semantics preserved; format is implementation detail (like `go.sum` vs JSON) |
| Git library | go-git | Pure Go, no external deps, portable |
| CLI framework | cobra | Go standard for CLI tools, used by kubectl, gh, etc. |
| Resolution | MVS | Deterministic, no solver, aligns with RFC |
| Cache location | `~/.craft/cache/` | Global, shared across projects |
| Auth | SSH + env tokens | Covers personal + CI workflows |
| Output | npm-style verbose | Progress bars, dependency trees |
| Testing | Unit + integration | Interfaces for mocking, fixture repos for integration |

## Critical Analysis

### Value Assessment

**High value**: This fills a genuine gap in the Agent Skills ecosystem. The RFC has community support (linked to 6+ issues with active discussion). A working implementation would be the first tool to solve the distribution problem.

### Build vs. Modify Tradeoffs

- **Build from scratch** (chosen): No existing tool does this. `skills-ref` is a validator only, not a package manager. Starting fresh allows clean architecture aligned with the RFC.
- **Extend skills-ref**: Possible but would add package management concerns to a validation tool. Different responsibility.

### Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| RFC changes before acceptance | Rework needed | Follow discussion closely; keep manifest parsing isolated |
| go-git SSH limitations | Some private repos inaccessible | Document limitations; consider hybrid fallback in future |
| Agent path detection fragile | Skills installed to wrong location | Always offer `--target` override; detect conservatively |
| MVS unfamiliar to npm users | Confusion about "why didn't I get the latest?" | Clear docs and `craft update` command |
| Skill name collisions common | Frustrating errors | Good error messages with resolution suggestions |

## Implementation Strategy

The MVP is too large for a single PAW workflow (~3,000–4,000 lines across ~25–30 files). It is split into **3 sequential workflows**, each producing a mergeable PR and a usable tool at that stage.

### Workflow 1: Foundation (local-only tool)

**Branch:** `craft-foundation` → `main`

**Scope:**
- Go project scaffolding (module, Cobra CLI skeleton, project structure)
- `craft.yaml` manifest parsing, validation, serialization
- `craft.pin.yaml` pinfile parsing, serialization
- `SKILL.md` frontmatter parser (YAML frontmatter extraction + validation)
- `craft init` — interactive setup wizard, generates `craft.yaml`
- `craft validate` — schema correctness, skill path validation, frontmatter validation, dependency URL format, pinfile integrity, name collision detection (local skills only)
- `craft version` — print tool version
- Unit tests for all parsers and validators

**What you get:** A working `craft` binary that can create and validate manifests. Zero network dependencies — pure parsing and file I/O. Proves the data model is sound.

**Estimated size:** ~1,000–1,500 lines

### Workflow 2: Resolution Engine (network-capable tool)

**Branch:** `craft-resolution` → `main` (after Workflow 1 merges)

**Scope:**
- go-git integration (clone, fetch, tag listing, file reading at specific commits)
- Global cache (`~/.craft/cache/`) — store, lookup, invalidation
- SSH + token authentication (`GITHUB_TOKEN`, `CRAFT_TOKEN`)
- MVS dependency resolution algorithm
- Dependency graph construction, cycle detection, version conflict resolution
- Skill name collision detection across full dependency tree (transitive deps)
- Auto-discovery for dependencies without `craft.yaml`
- Agent detection (Claude Code, Copilot) + skill installation
- `craft install` — full resolve → pin → install pipeline
- `craft update [dep]` — re-resolve to latest tags, rewrite pinfile
- `--target <path>` flag override
- Unit tests (mocked git) + integration tests (fixture repos)

**What you get:** The core tool — `craft install` resolves, pins, and installs skill dependencies from git repos to the right agent path.

**Estimated size:** ~1,200–1,800 lines

### Workflow 3: Package Operations & Polish

**Branch:** `craft-polish` → `main` (after Workflow 2 merges)

**Scope:**
- `craft add <dep>` — add dependency to `craft.yaml`, verify it resolves, optionally install
- `craft remove <dep>` — remove dependency, clean up orphaned installed skills
- npm-style UI — progress bars during fetch/resolve, dependency tree visualization after install
- Improved error messages with actionable suggestions
- Integration test suite against real fixture repos
- README with usage examples, installation instructions, known limitations

**What you get:** The complete MVP — all 6 commands working with polished UX.

**Estimated size:** ~800–1,200 lines

### Sequencing Rationale

Each workflow builds on the previous one's merged code:

```
Workflow 1 (Foundation)  →  Workflow 2 (Resolution)  →  Workflow 3 (Polish)
    craft init                  craft install               craft add
    craft validate              craft update                craft remove
    craft version               (git, cache, MVS)           (UI, tests, docs)
    (parsers, types)
```

- **Workflow 1** has zero external dependencies — the fastest, safest starting point
- **Workflow 2** is the hardest part, but by then manifest/pinfile types are battle-tested
- **Workflow 3** composes existing functionality — lowest risk, highest UX impact

Each PR stays reviewable (~1,000–1,500 lines) and each produces a shippable, testable tool at that stage.

## Scope Boundaries

### Definitely In (MVP)

- `craft.yaml` and `craft.pin.yaml` read/write
- Six CLI commands: init, install, update, validate, add, remove
- MVS dependency resolution with cycle and collision detection
- go-git based fetching with global cache
- Agent-aware installation (Claude Code, Copilot)
- SSH + token authentication
- npm-style progress output
- Unit + integration tests

### Explicitly Out (MVP)

- Monorepo subpath support (URL format designed for it, not implemented)
- OCI artifact support
- Central registry / search / discovery
- Skill composition semantics
- `.well-known/agent-skills.json` discovery
- Signature verification for packages
- Plugin system / extensibility
- GUI / TUI beyond progress bars

### Deferred (Post-MVP)

- Monorepo URL support
- `craft list` — show installed skills
- `craft info <dep>` — show dependency details
- `craft pin` — regenerate pinfile without installing
- `--json` flag for machine-readable output
- Hybrid git fallback (shell out for edge cases)
- OCI transport

## Open Questions for Downstream Stages

1. ~~**Cobra vs. stdlib `flag`**~~: Resolved — use Cobra.
2. ~~**Integrity digest scope**~~: Resolved — follow RFC (hash skill content only).
3. **Cache eviction**: When/how should old cached repos be cleaned up? Go uses `go clean -modcache`. Add `craft cache clean` post-MVP.
4. **`craft update` semantics**: Resolved — update to latest available semver tag (like `go get -u`).
5. **Config file**: Should `~/.craft/config.yaml` exist for global defaults (default agent, auth tokens, cache path)?

## Sanity Check Review

### 🔴 Resolved: Name Collision

**"pact" collides with pact-foundation/pact-cli** (contract testing, widely used in enterprise).
**Resolution:** Renamed to **`craft`** — to build skillfully; a craft is also a practiced skill. Captures the "agents building capabilities progressively" metaphor.

### 🟡 Agent Paths Need Refinement

The WorkShaping listed simplified paths. Actual agent skill paths are more nuanced:

**Claude Code** (confirmed):
- Global: `~/.claude/skills/<skill-name>/SKILL.md`
- Project-specific: `.claude/skills/<skill-name>/SKILL.md` (in repo root)
- Priority: project-specific → global → built-in

**GitHub Copilot CLI** (confirmed):
- Plugins: `~/.copilot/plugins/<plugin-name>/` with `plugin.json` manifest
- Plugin skills are referenced from `plugin.json`'s `"skills"` field
- User-level: `~/.copilot/skills/` (standalone skills outside plugins)

**Decision needed:** `craft install` should support:
- Claude Code: install to `~/.claude/skills/` (global) by default
- Copilot: install to `~/.copilot/skills/` (standalone) by default — plugin.json integration is a post-MVP concern
- `--target <path>` for manual override
- `--project` flag to install to project-specific path (`.claude/skills/`, `.github/skills/`)

### 🟡 go-git SSH Limitations Are Real

Research confirms go-git has notable limitations vs. git CLI:
- No `~/.ssh/config` ProxyJump support
- No hardware token (YubiKey) or smartcard support
- No agent forwarding
- Opaque error messages for auth failures
- Lags behind OpenSSH for new key types and security patches

**Mitigation:** Document known limitations prominently. For MVP, accept these constraints. Post-MVP, consider a `--git-cli` flag that shells out to the system git for auth operations only.

### 🟡 `craft update` Semantics Undefined

The RFC doesn't define version constraints beyond exact versions. "Updates to the latest compatible version" is ambiguous:
- The RFC uses `@v1.0.0` — exact version, not a range
- There are no `^`, `~`, or `>=` constraints in the spec
- So what does `craft update` actually do?

**Proposed semantics:** `craft update` fetches the latest semver tag from the dependency repo and updates to it (latest available). If the user wants to pin, they use an exact version. This is simple and matches Go's `go get -u` behavior.

### 🟢 Resolved: Integrity Digest Scope

Open question #2 asked about hashing scope. The RFC is explicit: "SHA-256 digest of the concatenated contents of all files in the dependency's skills directories, sorted by path." Follow the RFC — hash only skill content, not the entire repo.

### 🟢 Resolved: Cobra Is the Right Choice

Open question #1 about Cobra vs. stdlib — Cobra is the obvious choice for 6 subcommands. It provides help generation, shell completion, and subcommand routing for free. Used by kubectl, gh, hugo, etc. Not worth debating.

### 🟢 Missing: `craft version` Command

Standard CLI convention. Add `craft version` (or `craft --version`) to print the tool version. Trivial but expected.

### 🟢 Missing: `craft cache clean` Command

The global cache at `~/.craft/cache/` will grow unbounded. Need at least a `craft cache clean` command (like `go clean -modcache`). Can be post-MVP but should be designed-for.

## Session Notes

### Key Decisions Made

- **Name "craft"** — renamed from "pact" due to collision with pact-foundation contract testing CLI
- **Go over Rust/Python/TS** — natural fit given Go modules inspiration and single-binary requirement
- **Agent-aware over configurable** — prioritize UX; auto-detect Claude/Copilot, `--target` as escape hatch
- **MVS over latest-matching** — deterministic, simpler, aligns with RFC and Go's proven approach
- **go-git over shell-git** — keep the single-binary promise; accept SSH limitations (documented)
- **Global cache over per-project** — avoid redundant fetches, follow Go's proven model
- **Defer monorepo** — design URL format for it, but don't build resolution logic in MVP
- **Auto-discover over error** — deps without `craft.yaml` get SKILL.md scanned; maximizes backward compat
- **npm-style output** — progress bars and tree views over Go's minimal style
- **Personal project** — no upstream contribution overhead in initial development
- **YAML over JSON** — SKILL.md uses YAML frontmatter; comments are valuable in manifests; RFC semantics preserved regardless of format (like go.sum isn't JSON either)
- **`craft.yaml` + `craft.pin.yaml`** — branded file names instead of generic `skills.json`/`skills.lock`; "pin" conveys exactly what the pinfile does (pinned versions)
- **Cobra** — resolved as the CLI framework (was listed as open question, now decided)
- **Follow RFC on integrity digest** — hash skill content only, not entire repo
- **3 sequential PAW workflows** — Foundation (local-only), Resolution Engine (network), Polish (UX + remaining commands). Each produces a mergeable PR and usable tool.
- **`craft update` = latest tag** — simple semantics matching Go's approach
