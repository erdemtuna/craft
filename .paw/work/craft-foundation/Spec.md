# Feature Specification: Craft Foundation

**Branch**: feature/craft-foundation  |  **Created**: 2025-07-15  |  **Status**: Draft
**Input Brief**: Foundation layer for craft — an Agent Skills Package Manager CLI. Go project scaffolding, manifest/pinfile/frontmatter parsers, `craft init`, `craft validate`, `craft version` commands, and unit tests. Local-only with zero network dependencies.

## Overview

The Agent Skills specification defines how to write skills — a directory with a `SKILL.md` file — but provides no standard for distributing, versioning, or declaring dependencies between skills across repositories. Skill authors who build on skills from other repos must vendor code, document dependencies in READMEs, or hope users manually install the correct versions. craft is a CLI tool that solves this by implementing a manifest and pinfile system analogous to Go modules, using YAML as the serialization format for ecosystem consistency with SKILL.md frontmatter.

This specification covers the **foundation layer** — the first of three incremental workflows that together deliver a complete package manager. The foundation establishes the project structure, core data model (manifest and pinfile formats), skill discovery (SKILL.md frontmatter parsing), and three user-facing commands: `craft init` for interactive project setup, `craft validate` for comprehensive pre-flight validation, and `craft version` for installation verification.

The foundation is deliberately local-only with zero network dependencies. All operations work against the local filesystem — reading, writing, and validating YAML files and skill directories. This constraint keeps the first delivery small, testable, and immediately useful: a skill author can create a manifest, organize their skills, and validate everything is correctly structured before any network-dependent commands are introduced in subsequent workflows.

By the end of this workflow, a user can run `craft init` in their skill repository, answer a few prompts to generate a `craft.yaml` manifest, populate the skills array with paths to SKILL.md-containing directories, and run `craft validate` to confirm everything is well-formed — a complete local authoring workflow.

## Objectives

- Enable skill authors to formally declare their package metadata and exported skills through a standardized manifest file
- Provide interactive project bootstrapping that infers sensible defaults and guides users through manifest creation
- Validate manifest correctness, skill directory structure, SKILL.md frontmatter, dependency URL format, and pinfile structure in a single command
- Establish the core data model (manifest and pinfile types) that subsequent workflows will build upon for dependency resolution and installation
- Deliver a single-binary CLI tool with zero external runtime dependencies

## User Scenarios & Testing

### User Story P1 – Initialize a New Skill Package

Narrative: A skill author has a repository containing one or more agent skills. They want to create a `craft.yaml` manifest to formally declare their package. They run `craft init`, answer prompts for name, version, description, and license, and the tool generates a valid manifest file with discovered skill paths included.

Independent Test: Run `craft init` in a directory containing skill subdirectories, answer all prompts, and verify a valid `craft.yaml` is created with the correct skills listed.

Acceptance Scenarios:
1. Given an empty directory, When the user runs `craft init` and provides all prompted values, Then a `craft.yaml` is created with the provided values and `schema_version: 1`
2. Given a directory with subdirectories containing SKILL.md files, When the user runs `craft init`, Then the tool auto-discovers those directories and includes them in the `skills` array
3. Given the user accepts default values for all prompts, Then the manifest uses the current directory name as package name and `0.1.0` as version
4. Given a `craft.yaml` already exists in the directory, When the user runs `craft init`, Then the tool warns about the existing file and asks for confirmation before overwriting

### User Story P1 – Validate Package Correctness

Narrative: A skill author has a `craft.yaml` and wants to verify everything is correct before committing or sharing. They run `craft validate` and receive a clear report of any issues — schema errors, missing SKILL.md files, malformed frontmatter, invalid dependency URLs, or pinfile structural problems.

Independent Test: Run `craft validate` on a well-formed package with valid skills and verify it reports success with zero errors.

Acceptance Scenarios:
1. Given a valid `craft.yaml` with valid skill directories containing properly formatted SKILL.md files, When the user runs `craft validate`, Then the tool reports success with zero errors
2. Given a `craft.yaml` with a skill path pointing to a directory without a SKILL.md file, When the user runs `craft validate`, Then the tool reports an error identifying the specific invalid path
3. Given a `craft.yaml` with an invalid dependency URL format, When the user runs `craft validate`, Then the tool reports an error with the expected URL format
4. Given a `craft.pin.yaml` whose entries do not match the current `craft.yaml` dependencies, When the user runs `craft validate`, Then the tool reports the inconsistency
5. Given multiple validation errors exist across different checks, When the user runs `craft validate`, Then all errors are reported together (not stopping at the first error)
6. Given a SKILL.md with invalid frontmatter (missing required fields or malformed YAML), When the user runs `craft validate`, Then the tool reports specific frontmatter errors identifying the skill path and the issue

### User Story P2 – Verify Tool Installation

Narrative: A user has installed craft and wants to confirm it's working and check which version they have.

Independent Test: Run `craft version` and verify it prints a version string.

Acceptance Scenarios:
1. Given craft is installed, When the user runs `craft version`, Then the tool prints the version string and exits with code 0

### User Story P2 – Author Skills with Valid Frontmatter

Narrative: A skill author creates SKILL.md files for their skills and wants to ensure the YAML frontmatter is well-formed. The frontmatter contains the skill name and description that craft uses for identification and collision detection. Running `craft validate` checks that all SKILL.md files in the package have valid frontmatter.

Independent Test: Run `craft validate` on a package containing SKILL.md files with various frontmatter states and verify correct/incorrect ones are identified.

Acceptance Scenarios:
1. Given a SKILL.md with valid YAML frontmatter containing at least a `name` field, When `craft validate` runs, Then the skill passes frontmatter validation
2. Given a SKILL.md with no frontmatter (no YAML `---` delimiters at file start), When `craft validate` runs, Then the tool reports a clear error about missing frontmatter for that skill path
3. Given a SKILL.md with frontmatter missing the required `name` field, When `craft validate` runs, Then the tool reports the specific missing field and skill path
4. Given a SKILL.md with a `name` that violates naming conventions, When `craft validate` runs, Then the tool reports the naming constraint violation

### Edge Cases

- Empty `skills` array in craft.yaml: validation error — a package must export at least one skill
- Skill path outside the repository root (e.g., `../other-repo/skill`): validation error — skill paths must be relative and within the package
- Duplicate skill names across different skill paths within the same package: validation error listing both paths that export the conflicting name
- craft.yaml with unknown/extra fields: silently accepted (forward compatibility) — only known fields are validated
- SKILL.md frontmatter with extra fields beyond name/description: accepted — only required fields are validated
- Package name with uppercase, spaces, or special characters: validation error with allowed format description
- Version string not following semantic versioning: validation error with expected format
- Circular symlinks in skill directory paths: handled gracefully without infinite loops, with a clear error
- Read-only filesystem: clear error when `craft init` cannot write the manifest file
- Non-interactive environment (no TTY): `craft init` detects and provides a clear error message
- `craft validate` run with no `craft.yaml` present: clear error indicating no manifest found
- `craft.pin.yaml` exists but `craft.yaml` has no dependencies: warning about unnecessary pinfile
- SKILL.md with malformed YAML (syntax error in frontmatter): error with parse details, not a crash
- Very deeply nested skill directory paths: accepted if valid, no artificial depth limit

## Requirements

### Functional Requirements

- FR-001: Parse `craft.yaml` manifest files with all defined schema fields (schema_version, name, version, skills, description, license, dependencies, metadata) (Stories: P1-Init, P1-Validate)
- FR-002: Validate `craft.yaml` against schema rules — required fields present, correct types, format constraints on name and version (Stories: P1-Validate)
- FR-003: Serialize in-memory manifest representation to `craft.yaml` with proper YAML formatting and field ordering (Stories: P1-Init)
- FR-004: Parse `craft.pin.yaml` pinfile with all defined fields (pin_version, resolved map with commit, integrity, skills) (Stories: P1-Validate)
- FR-005: Validate pinfile structure (required fields including commit, integrity, and skills list; correct types) and, when a pinfile is present, consistency with manifest dependencies (each manifest dependency has a pinfile entry and vice versa) (Stories: P1-Validate)
- FR-006: Extract YAML frontmatter from SKILL.md files — the YAML block delimited by `---` markers at the start of the file (Stories: P2-Frontmatter, P1-Validate)
- FR-007: Validate SKILL.md frontmatter for required fields (name) and naming format constraints (Stories: P2-Frontmatter, P1-Validate)
- FR-008: Interactive `craft init` command — prompt for name (default: directory name), version (default: 0.1.0), description, license; auto-discover skill directories; generate valid `craft.yaml` (Stories: P1-Init)
- FR-009: `craft validate` command — run all validation checks (schema, skill paths, frontmatter, dependency URLs, pinfile structure, name collisions) and report all errors with actionable messages (Stories: P1-Validate)
- FR-010: `craft version` command — print the tool version and exit (Stories: P2-Version)
- FR-011: Validate dependency URL format (`host/org/repo@vMAJOR.MINOR.PATCH` pattern — `v` prefix required) without network access (Stories: P1-Validate)
- FR-012: Detect skill name collisions within the local package — multiple skill directories exporting the same `name` in their SKILL.md frontmatter (Stories: P1-Validate)
- FR-013: Package name validation — lowercase alphanumeric with hyphens, no leading/trailing hyphens, 1–128 characters (Stories: P1-Init, P1-Validate)
- FR-014: Version string validation — must conform to strict semantic versioning format (exactly MAJOR.MINOR.PATCH, no pre-release or build metadata suffixes) (Stories: P1-Init, P1-Validate)
- FR-015: `craft init` skill directory auto-discovery — recursively find directories containing SKILL.md files, skipping `.git`, `.paw`, `node_modules`, and hidden directories, and suggest them for the skills array (Stories: P1-Init)
- FR-016: `craft init` overwrite protection — detect existing `craft.yaml` and require explicit confirmation before overwriting (Stories: P1-Init)

### Key Entities

- **Manifest**: The `craft.yaml` file declaring package identity (name, version), exported skills (paths), optional dependencies (alias → git URL@version map), and optional metadata
- **Pinfile**: The `craft.pin.yaml` file recording resolved dependency state — each dependency mapped to a specific commit SHA, integrity digest, and discovered skills list
- **Skill**: A directory containing a SKILL.md file with YAML frontmatter; identified by the `name` field in its frontmatter
- **Dependency**: A reference to an external git repository at a specific version, declared as an alias → `host/org/repo@version` entry in the manifest's dependencies map

### Cross-Cutting / Non-Functional

- All validation errors include actionable messages identifying the specific field, path, or value that failed, with guidance on how to fix it
- The tool exits with code 0 on success and non-zero on any failure
- No network access is performed by any command in this workflow
- All commands provide help text via the standard `--help` flag
- Error output goes to stderr; normal output goes to stdout
- The binary has zero external runtime dependencies — single statically-linked executable

## Success Criteria

- SC-001: A user can run `craft init` in a directory and produce a valid `craft.yaml` through interactive prompts, with skill directories auto-discovered (FR-008, FR-003, FR-015)
- SC-002: Running `craft validate` on a valid package with correct skills, frontmatter, and optionally a matching pinfile reports zero errors and exits with code 0 (FR-002, FR-005, FR-007, FR-009)
- SC-003: Running `craft validate` on a package with known errors reports all errors with actionable messages — not just the first error encountered (FR-009, FR-002, FR-007, FR-011)
- SC-004: Skill name collisions within a package are detected and reported listing both conflicting skill paths (FR-012, FR-009)
- SC-005: SKILL.md files without valid frontmatter (missing delimiters, missing name field, malformed YAML) are each identified with specific error details (FR-006, FR-007)
- SC-006: All parsers (manifest, pinfile, frontmatter) handle well-formed input correctly and produce meaningful errors on malformed input (FR-001, FR-004, FR-006)
- SC-007: Unit tests cover all parsers, validators, and command logic (FR-001 through FR-016)
- SC-008: `craft version` displays the current version string and exits cleanly (FR-010)

## Assumptions

- SKILL.md YAML frontmatter is delimited by `---` markers at the very start of the file, following standard frontmatter conventions
- The `name` field is the only required field in SKILL.md frontmatter for this workflow; `description` is recommended but optional
- Skill names follow the same naming convention as package names: lowercase alphanumeric with hyphens
- Package names follow the pattern `[a-z][a-z0-9]*(-[a-z0-9]+)*` — lowercase, hyphen-separated segments, no leading/trailing hyphens
- `schema_version` in craft.yaml is always `1` for this initial release
- `pin_version` in craft.pin.yaml is always `1`
- Dependency URLs follow the format `host/org/repo@vMAJOR.MINOR.PATCH` — the version component requires a `v` prefix followed by strict semver (e.g., `github.com/example/skills@v1.0.0`)
- Version strings follow strict semantic versioning: exactly `MAJOR.MINOR.PATCH` with no pre-release or build metadata suffixes (e.g., `1.0.0`, `0.1.0`; `1.0.0-alpha` and `1.0.0+build` are rejected)
- `craft init` uses the current directory name (lowercased, sanitized) as the default package name
- `craft init` uses `0.1.0` as the default version
- Unknown fields in `craft.yaml` are preserved during round-trip parsing (forward compatibility)
- The pinfile is machine-generated and not expected to be hand-edited, but must be valid YAML
- The pinfile `resolved` map is keyed by the dependency URL (the value from the manifest dependencies map, e.g., `github.com/example/skills@v1.0.0`), not the alias
- The pinfile is optional — `craft validate` performs pinfile consistency checks only when `craft.pin.yaml` is present. A package with dependencies but no pinfile is valid (the pinfile is generated by `craft install`, which is not part of this workflow)
- Skill auto-discovery (`craft init`) skips `.git`, `.paw`, `node_modules`, and hidden directories (directories starting with `.`) during recursive traversal — these are standard non-skill infrastructure directories

## Scope

In Scope:
- Go project scaffolding with module initialization and Cobra CLI framework setup
- `craft.yaml` manifest: type definitions, YAML parsing, schema validation, serialization
- `craft.pin.yaml` pinfile: type definitions, YAML parsing, structural validation, manifest consistency checking
- SKILL.md YAML frontmatter: extraction, parsing, field validation
- `craft init` interactive command with default inference and skill auto-discovery
- `craft validate` comprehensive validation command (all checks, all errors reported)
- `craft version` command
- Unit tests for all parsers, validators, and command behavior
- Clear, actionable error messages for all failure modes

Out of Scope:
- Network access of any kind (git fetching, repository cloning, tag listing)
- `craft install`, `craft add`, `craft remove`, `craft update` commands (Workflows 2–3)
- Dependency resolution algorithm (Minimum Version Selection) (Workflow 2)
- Pinfile generation and writing (Workflow 2 — this workflow only reads and validates existing pinfiles)
- SHA-256 integrity digest computation and verification (Workflow 2)
- Global cache management (~/.craft/cache/) (Workflow 2)
- Agent detection and skill installation to agent paths (Workflow 2)
- Progress bars and dependency tree visualization (Workflow 3)
- `craft add` and `craft remove` commands (Workflow 3)
- Configuration file (~/.craft/config.yaml)
- Monorepo subpath support
- Flag-based non-interactive mode for `craft init` (future enhancement)

## Dependencies

- Go standard library (os, path/filepath, fmt, strings, etc.)
- Cobra CLI framework for command routing, help generation, and flag parsing
- YAML parsing library for Go (gopkg.in/yaml.v3)
- No external runtime dependencies — single statically-linked binary

## Risks & Mitigations

- **Schema evolution**: The craft.yaml schema may need changes as later workflows reveal requirements (e.g., new fields for resolution). Mitigation: `schema_version` field enables versioned parsing; unknown fields are preserved for forward compatibility
- **SKILL.md frontmatter variance**: Frontmatter format may vary across Agent Skills implementations. Mitigation: Use standard `---` delimiters; validate only the fields craft requires; accept extra fields gracefully
- **Interactive prompts in non-TTY environments**: `craft init` relies on interactive input that won't work in CI or piped contexts. Mitigation: Detect non-TTY and exit with clear error message; flag-based non-interactive mode deferred to future enhancement
- **Pinfile validation without generation**: This workflow validates existing pinfiles but cannot generate them, limiting integration testing of pinfile validation. Mitigation: Use fixture pinfile files in unit tests; full end-to-end pinfile testing in Workflow 2
- **Data model lock-in**: Types defined here must serve Workflows 2 and 3 without breaking changes. Mitigation: Design types with extensibility in mind; use interfaces where behavior will vary across workflows

## References

- WorkShaping: .paw/work/WorkShaping.md
- Agent Skills Specification: https://agentskills.io/specification
- RFC Discussion: https://github.com/agentskills/agentskills/discussions/210
