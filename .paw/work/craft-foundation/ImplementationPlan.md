# Craft Foundation Implementation Plan

## Overview

Implement the foundation layer for craft — an Agent Skills Package Manager CLI. This greenfield Go project establishes the core data model (manifest, pinfile, frontmatter types), three parsers with validation, and three user-facing commands (`craft init`, `craft validate`, `craft version`). All operations are local-only with zero network dependencies.

## Current State Analysis

The repository is empty — a single `README.md` with a one-line description. No Go module, no source files, no CI configuration. All project structure, dependencies, and code will be created from scratch.

**Key constraints**:
- Single statically-linked binary, zero runtime dependencies
- All output to stdout, errors to stderr, exit 0/non-zero conventions
- YAML as the serialization format (ecosystem consistency with SKILL.md frontmatter)
- Forward compatibility: unknown fields in craft.yaml must be preserved
- This is Workflow 1 of 3 — types must be designed with extensibility for future dependency resolution and installation commands

## Desired End State

A working `craft` CLI binary with three commands:
- `craft version` prints the version string and exits
- `craft init` interactively creates a valid `craft.yaml` with auto-discovered skill paths
- `craft validate` runs all validation checks (schema, skill paths, frontmatter, dependency URLs, pinfile consistency, name collisions) and reports all errors with actionable messages

**Verification approach**: `go test ./...` passes with comprehensive coverage of all parsers, validators, and command logic. `go build ./cmd/craft` produces a working binary. Manual verification of each command against Spec.md acceptance scenarios.

## What We're NOT Doing

- Network access of any kind (git fetching, cloning, tag listing)
- `craft install`, `craft add`, `craft remove`, `craft update` commands (Workflows 2–3)
- Dependency resolution algorithm (MVS) — Workflow 2
- Pinfile generation/writing — this workflow only reads and validates existing pinfiles
- SHA-256 integrity digest computation — Workflow 2
- Global cache management — Workflow 2
- Agent detection and skill installation — Workflow 2
- Progress bars and dependency tree visualization — Workflow 3
- CI/CD pipeline setup
- Flag-based non-interactive mode for `craft init`
- Configuration file (~/.craft/config.yaml)
- Monorepo subpath support

## Phase Status

- [x] **Phase 1: Project Scaffolding, Core Types & Version Command** — Go module, directory structure, Cobra CLI skeleton, type definitions, working `craft version`
- [x] **Phase 2: Parsers & Per-Type Validation** — Manifest, pinfile, and frontmatter parsing with individual schema/format validation and comprehensive unit tests (depends on Phase 1)
- [x] **Phase 3: craft validate Command** — Orchestrate all validation checks with error collection, skill path validation, name collision detection, pinfile consistency, and dependency URL format checking (depends on Phase 2)
- [x] **Phase 4: craft init Command** — Interactive prompts with defaults, skill directory auto-discovery, manifest generation, overwrite protection, and TTY detection (depends on Phases 1–2)
- [x] **Phase 5: Documentation** — Docs.md technical reference and README.md update (depends on Phases 1–4)

## Phase Candidates

<!-- No candidates identified — all requirements map to defined phases -->

---

## Phase 1: Project Scaffolding, Core Types & Version Command

### Changes Required

- **`go.mod`**: Initialize module `github.com/erdemtuna/craft` with Go 1.24+. Add dependencies: `github.com/spf13/cobra`, `gopkg.in/yaml.v3`
- **`cmd/craft/main.go`**: Entry point — call root command `Execute()`
- **`internal/cli/root.go`**: Cobra root command for `craft` with usage description, stderr for errors. Subcommands: init, validate, version (init and validate as stubs returning "not implemented" for this phase)
- **`internal/cli/version.go`**: `craft version` command — prints version string from a package-level variable to stdout, exits 0. Version set via `internal/version/version.go` constant (overridable via `-ldflags` in future builds)
- **`internal/version/version.go`**: Package exposing `Version` constant (default `"dev"`)
- **`internal/manifest/types.go`**: `Manifest` struct with YAML tags — fields: `SchemaVersion`, `Name`, `Version`, `Description`, `License`, `Skills` ([]string), `Dependencies` (map[string]string), `Metadata` (map[string]string). Preserve unknown fields for forward compatibility
- **`internal/pinfile/types.go`**: `Pinfile` struct — fields: `PinVersion`, `Resolved` map keyed by dependency URL, each entry containing `Commit`, `Integrity`, `Skills` ([]string)
- **`internal/skill/types.go`**: `Frontmatter` struct — fields: `Name`, `Description` (optional), plus raw extra fields for forward compatibility
- **Directory structure**:
  ```
  cmd/craft/
  internal/cli/
  internal/version/
  internal/manifest/
  internal/pinfile/
  internal/skill/
  ```
- **Tests**:
  - `internal/cli/version_test.go`: Verify version command output contains version string
  - `internal/version/version_test.go`: Verify default version value

### Success Criteria

#### Automated Verification
- [ ] `go build ./cmd/craft` compiles without errors
- [ ] `go test ./...` passes
- [ ] `go vet ./...` reports no issues

#### Manual Verification
- [ ] `./craft version` prints a version string and exits 0
- [ ] `./craft --help` shows usage with init, validate, version subcommands listed
- [ ] `./craft init` and `./craft validate` respond with stub messages (not panics)

---

## Phase 2: Parsers & Per-Type Validation

### Changes Required

- **`internal/manifest/parse.go`**: `Parse(r io.Reader) (*Manifest, error)` — YAML parsing with `gopkg.in/yaml.v3`, unknown field preservation via a raw node approach or inline map. `ParseFile(path string) (*Manifest, error)` convenience wrapper
- **`internal/manifest/validate.go`**: `Validate(m *Manifest) []error` — validate schema_version is 1, name is present and matches `[a-z][a-z0-9]*(-[a-z0-9]+)*` (1–128 chars), version matches semver `MAJOR.MINOR.PATCH`, skills array is non-empty, dependency URL format `host/org/repo@version` for each entry
- **`internal/manifest/write.go`**: `Write(m *Manifest, w io.Writer) error` — serialize manifest to YAML with consistent field ordering (schema_version, name, version, description, license, skills, dependencies, metadata)
- **`internal/pinfile/parse.go`**: `Parse(r io.Reader) (*Pinfile, error)` — YAML parsing. `ParseFile(path string) (*Pinfile, error)` convenience wrapper
- **`internal/pinfile/validate.go`**: `Validate(p *Pinfile) []error` — validate pin_version is 1, resolved map entries have required fields (commit, integrity, skills). Covers structural half of FR-005; consistency checking deferred to Phase 3
- **`internal/skill/frontmatter.go`**: `ParseFrontmatter(r io.Reader) (*Frontmatter, error)` — extract YAML block between `---` delimiters at file start, parse YAML, return structured frontmatter. Handle: no delimiters (error), malformed YAML (error with parse details), missing delimiters (error)
- **`internal/skill/validate.go`**: `ValidateFrontmatter(fm *Frontmatter) []error` — name is present, name matches naming convention (same as package name format)
- **Tests** (colocated `_test.go` files in each package):
  - `internal/manifest/parse_test.go`: Valid manifest, missing required fields, extra unknown fields preserved, malformed YAML
  - `internal/manifest/validate_test.go`: Valid name/version, invalid name formats (uppercase, spaces, leading/trailing hyphens), invalid semver, empty skills array, valid/invalid dependency URLs
  - `internal/manifest/write_test.go`: Round-trip parse→write→parse produces equivalent manifest
  - `internal/pinfile/parse_test.go`: Valid pinfile, missing fields, malformed YAML
  - `internal/pinfile/validate_test.go`: Valid structure, missing commit/integrity/skills fields
  - `internal/skill/frontmatter_test.go`: Valid frontmatter, no delimiters, missing name, malformed YAML, extra fields accepted
  - `internal/skill/validate_test.go`: Valid skill name, invalid formats
- **`testdata/`**: YAML fixture files for parser tests — valid and invalid manifests, pinfiles, and SKILL.md files

### Success Criteria

#### Automated Verification
- [ ] `go test ./...` passes — all parser and validation tests green
- [ ] `go vet ./...` reports no issues

#### Manual Verification
- [ ] Manifest round-trip: parse a craft.yaml → serialize → parse again produces identical struct
- [ ] Each validation rule produces an actionable error message identifying the specific field and expected format
- [ ] Frontmatter parser handles edge cases: no delimiters, malformed YAML, missing name — each with distinct error messages

---

## Phase 3: craft validate Command

### Changes Required

- **`internal/validate/runner.go`**: `Runner` struct orchestrating all validation checks. Accepts a root directory path. Collects all errors across checks (does not stop at first error). Returns structured results with per-check error categorization
  - Check 1: Parse and validate craft.yaml schema (manifest package)
  - Check 2: For each skill path — verify directory exists, verify SKILL.md exists, parse frontmatter, validate frontmatter (skill package)
  - Check 3: Validate dependency URL format for each dependency entry (manifest package)
  - Check 4: If craft.pin.yaml exists — parse, validate structure, check consistency with manifest dependencies (completes FR-005 consistency aspect: each manifest dep has a pin entry and vice versa). If no dependencies but pinfile exists — warning
  - Check 5: Name collision detection — collect all skill names from frontmatter across all skill paths, report duplicates with both conflicting paths
  - Check 6: Skill path safety — paths must be relative and within the package root (no `../` escapes)
  - Check 7: Symlink cycle detection — gracefully handle circular symlinks in skill directory paths without infinite loops, reporting a clear error
- **`internal/validate/errors.go`**: Structured error types with category, path, field, message, and suggestion fields for actionable output formatting
- **`internal/cli/validate.go`**: Replace stub with real implementation — instantiate `Runner`, execute, format errors to stderr with actionable messages, exit 0 on success or 1 on any errors. Report error count summary
- **Tests**:
  - `internal/validate/runner_test.go`: Integration-level tests using `testdata/` fixture directories:
    - Valid package with skills → success
    - Missing SKILL.md in declared path → specific error
    - Invalid dependency URL → format error
    - Pinfile inconsistency with manifest → consistency error
    - Duplicate skill names → collision error with both paths listed
    - Skill path escaping root (`../`) → safety error
    - Multiple errors across checks → all reported together
    - No craft.yaml present → clear "no manifest found" error
    - Pinfile exists but no dependencies → warning
  - `internal/cli/validate_test.go`: Command execution returns correct exit codes
- **`testdata/`**: Additional fixture directories for validate integration tests — valid packages, packages with specific errors

### Success Criteria

#### Automated Verification
- [ ] `go test ./...` passes — all validation runner and command tests green
- [ ] `go vet ./...` reports no issues

#### Manual Verification
- [ ] `craft validate` in a directory with valid craft.yaml and skills reports success, exits 0
- [ ] `craft validate` with multiple intentional errors reports ALL errors (not just the first), exits 1
- [ ] Each error message includes: what failed, where (file/field/path), and how to fix it
- [ ] `craft validate` with no craft.yaml reports "no manifest found" error
- [ ] Symlink cycles in skill paths handled gracefully (no infinite loop, clear error)
- [ ] `craft validate --help` prints usage and exits 0

---

## Phase 4: craft init Command

### Changes Required

- **`internal/init/wizard.go`**: Interactive wizard orchestrating the init flow:
  - TTY detection — if stdin is not a terminal, exit with clear error message
  - Overwrite protection — if craft.yaml exists, prompt for confirmation before proceeding
  - Prompt for: name (default: current directory name, lowercased and sanitized), version (default: `0.1.0`), description (default: empty), license (default: empty)
  - Input validation on each prompt — reject invalid name/version formats with inline feedback
  - Auto-discover skill directories via `DiscoverSkills` and display found paths for confirmation
  - Build `Manifest` struct with schema_version 1 and provided values
  - Serialize to craft.yaml via manifest `Write` function
  - Handle read-only filesystem gracefully (clear error on write failure)
- **`internal/init/discover.go`**: `DiscoverSkills(root string) ([]string, error)` — recursively walk directory tree from root, find directories containing a `SKILL.md` file, return relative paths. Skip `.git`, `.paw`, `node_modules`, and hidden directories. Handle symlink cycles via `filepath.WalkDir` with seen-inode tracking or symlink-aware walking
- **`internal/cli/init.go`**: Replace stub with real implementation — instantiate wizard, execute, handle errors
- **Tests**:
  - `internal/init/discover_test.go`: Discovers nested skill dirs, skips hidden dirs and .git, handles empty directory, returns relative paths sorted
  - `internal/init/wizard_test.go`: Test wizard logic with simulated stdin/stdout — mock reader/writer to verify prompt flow, default values, validation feedback, overwrite protection flow. Test non-TTY detection
  - `internal/cli/init_test.go`: Command integration with fixture directories
- **`testdata/`**: Fixture directories for init tests — directories with skill subdirs, empty dirs, existing craft.yaml

### Success Criteria

#### Automated Verification
- [ ] `go test ./...` passes — all init tests green
- [ ] `go vet ./...` reports no issues

#### Manual Verification
- [ ] `craft init` in a directory with skill subdirs prompts for values, discovers skills, generates valid craft.yaml
- [ ] Default values work correctly (dir name for package name, 0.1.0 for version)
- [ ] Invalid name input (uppercase, special chars) rejected with inline guidance
- [ ] Existing craft.yaml triggers overwrite confirmation prompt
- [ ] Generated craft.yaml passes `craft validate`
- [ ] Non-TTY environment (piped input) produces clear error about interactive requirement
- [ ] `craft init --help` prints usage and exits 0

---

## Phase 5: Documentation

### Changes Required

- **`.paw/work/craft-foundation/Docs.md`**: Technical reference covering implementation architecture, package structure, type definitions, parser APIs, validation pipeline, command behaviors, testing approach, and verification instructions (load `paw-docs-guidance` for template)
- **`README.md`**: Update with project description, installation instructions (go install), usage examples for all three commands, craft.yaml format reference, SKILL.md frontmatter requirements, and contributing basics

### Success Criteria

- [ ] Docs.md accurately reflects implemented architecture and provides sufficient detail for future workflow contributors
- [ ] README.md provides clear getting-started path for new users
- [ ] All code examples in documentation are accurate and tested

---

## References

- Issue: none
- Spec: `.paw/work/craft-foundation/Spec.md`
- Research: `.paw/work/craft-foundation/CodeResearch.md`
- WorkShaping: `.paw/work/WorkShaping.md`
