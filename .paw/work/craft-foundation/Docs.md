# Craft Foundation

## Overview

Craft is a CLI package manager for Agent Skills — the directory-with-SKILL.md convention defined by the [Agent Skills specification](https://agentskills.io/specification). This foundation layer establishes the core data model, parsers, and three user-facing commands that enable skill authors to create manifests, organize skills, and validate their packages locally — all with zero network dependencies.

The tool reads and writes `craft.yaml` manifests (package identity, exported skills, dependency declarations), validates `craft.pin.yaml` pinfiles (resolved dependency state), and parses SKILL.md YAML frontmatter (skill identity). Three commands are available: `craft init` for interactive package setup, `craft validate` for comprehensive pre-flight validation, and `craft version` for installation verification.

## Architecture and Design

### Package Structure

```
cmd/craft/main.go          Entry point — calls cli.Execute()
internal/
  cli/                      Cobra command definitions (root, init, validate, version)
  version/                  Version constant (overridable via -ldflags)
  manifest/                 craft.yaml types, parse, validate, write
  pinfile/                  craft.pin.yaml types, parse, validate
  skill/                    SKILL.md frontmatter extraction and validation
  validate/                 Validation orchestrator (Runner) with error collection
  init/                     Interactive wizard and skill directory discovery
testdata/                   Parser fixtures and integration test packages
```

### Design Decisions

**YAML for all formats**: craft.yaml, craft.pin.yaml, and SKILL.md frontmatter all use YAML. This provides ecosystem consistency (SKILL.md already uses YAML frontmatter) and supports inline comments for pinning reasons and other annotations.

**Forward compatibility via unknown field tolerance**: The manifest parser silently accepts unknown YAML fields rather than rejecting them. This means a manifest written by a newer craft version (with additional fields) can still be parsed by an older version. Frontmatter achieves the same via a two-pass parse that captures extra fields in an `Extra` map.

**Collect-all-errors validation**: The `validate` command collects every error across all checks rather than stopping at the first failure. This gives users a complete picture of what needs fixing in a single run. Errors carry structured metadata (category, path, field, message, suggestion) for actionable output.

**Single Scanner pattern for init**: The wizard uses one `bufio.Scanner` throughout the interactive flow to avoid buffering issues when reading from pipes or test mocks. The overwrite prompt and value prompts share the same scanner instance.

**Pinfile optionality**: The pinfile is optional for validation. Since `craft install` (Workflow 2) generates the pinfile, requiring it in Workflow 1 would make `craft validate` unusable on new packages. When present, consistency with manifest dependencies is verified bidirectionally.

**Pinfile keyed by URL, not alias**: The pinfile `resolved` map uses dependency URLs (e.g., `github.com/example/skills@v1.0.0`) as keys rather than aliases. This ensures pinfile entries are unambiguous even if aliases change.

### Integration Points

The foundation types and parsers are designed to be extended by Workflows 2 and 3:

- `manifest.Manifest` struct will be used by the resolver to read dependency declarations
- `pinfile.Pinfile` will be written (not just read) by the install command
- `skill.ParseFrontmatter` will be used to discover skill names in fetched dependencies
- `validate.Runner` will be extended with additional checks (e.g., integrity verification)
- `manifest.ValidateName` is reused by `skill.ValidateFrontmatter` — same naming rules

## User Guide

### Prerequisites

- Go 1.24+ (for building from source)
- A directory containing one or more skill subdirectories with SKILL.md files

### Basic Usage

**Initialize a new package:**
```bash
cd my-skills-repo
craft init
```
Prompts for package name (default: directory name), version (default: 0.1.0), description, and license. Auto-discovers skill directories containing SKILL.md files.

**Validate a package:**
```bash
craft validate
```
Runs all checks: manifest schema, skill paths, SKILL.md frontmatter, dependency URL format, pinfile consistency, and skill name collisions. Reports all errors with actionable fix suggestions.

**Check version:**
```bash
craft version
```

### craft.yaml Format

```yaml
schema_version: 1
name: my-package          # required, lowercase alphanumeric with hyphens
version: 1.0.0            # required, strict MAJOR.MINOR.PATCH
description: My skills.   # optional
license: MIT              # optional

skills:                   # required, at least one path
  - ./skills/my-skill
  - ./skills/other-skill

dependencies:             # optional
  alias: github.com/org/repo@v1.0.0

metadata:                 # optional, arbitrary key-value pairs
  author: me
```

### SKILL.md Frontmatter

Each skill directory must contain a SKILL.md file starting with YAML frontmatter:

```markdown
---
name: my-skill            # required, same naming rules as package names
description: What it does # optional but recommended
---

# My Skill
...
```

### Validation Checks

| Check | What it validates |
|-------|-------------------|
| Schema | schema_version=1, name format, semver version, non-empty skills |
| Skill paths | Directory exists, contains SKILL.md, no path escapes |
| Frontmatter | Valid YAML, required `name` field, naming convention |
| Dependencies | URL format: `host/org/repo@vMAJOR.MINOR.PATCH` |
| Pinfile | Structure, consistency with manifest (if present) |
| Collisions | No duplicate skill names across paths |
| Symlinks | Graceful handling of circular symlinks |

## API Reference

### Key Components

**manifest.Parse/ParseFile** — Parse craft.yaml into a `Manifest` struct. Tolerates unknown fields.

**manifest.Validate** — Returns `[]error` with all schema violations. Checks schema_version, name format, semver version, non-empty skills, and dependency URL format.

**manifest.Write** — Serialize a `Manifest` to YAML with consistent field ordering (schema_version, name, version, description, license, skills, dependencies, metadata).

**manifest.ValidateName/ValidateVersion** — Standalone validators for name and version strings, reusable outside manifest context.

**pinfile.Parse/ParseFile** — Parse craft.pin.yaml into a `Pinfile` struct.

**pinfile.Validate** — Structural validation: pin_version=1, required fields (commit, integrity, skills) on each resolved entry.

**skill.ParseFrontmatter/ParseFrontmatterFile** — Extract YAML between `---` delimiters, parse into `Frontmatter` struct with extra fields captured.

**skill.ValidateFrontmatter** — Validate required `name` field and naming convention.

**validate.NewRunner(root).Run()** — Run all validation checks, returns `*Result` with `Errors` and `Warnings` slices. `Result.OK()` returns true if no errors.

**initcmd.DiscoverSkills(root)** — Recursively find skill directories, returns sorted relative paths.

## Testing

### How to Test

```bash
# Run all tests
go test ./...

# Build the binary
go build -o craft ./cmd/craft

# Test init (in a directory with skill subdirectories)
./craft init

# Test validate (in a directory with craft.yaml)
./craft validate

# Test version
./craft version
```

### Edge Cases

- Empty `skills` array → validation error (must have at least one)
- Skill path with `../` escape → safety error
- Duplicate skill names → collision error listing both paths
- Malformed SKILL.md frontmatter → specific parse error
- Missing closing `---` delimiter → clear error
- Pre-release versions like `1.0.0-alpha` → rejected (strict semver only)
- Dependency URL without `v` prefix → rejected
- Pinfile exists but no dependencies → warning (not error)
- Circular symlinks in skill paths → graceful error (no infinite loop)
- Non-TTY environment for `craft init` → clear error message

## Limitations and Future Work

- **No network access**: All operations are local-only. `craft install`, `craft add`, `craft remove`, `craft update` are planned for Workflows 2–3.
- **No pinfile generation**: This workflow only reads and validates existing pinfiles. Generation happens in `craft install` (Workflow 2).
- **No non-interactive init**: `craft init` requires a TTY. A `--non-interactive` flag with `--name`, `--version` etc. is a future enhancement.
- **Map serialization order**: Dependencies and metadata maps serialize in Go's non-deterministic map iteration order. This doesn't affect correctness but means consecutive writes may produce different key orderings.
- **No monorepo subpath support**: Dependencies reference entire repositories, not subdirectories within them.
