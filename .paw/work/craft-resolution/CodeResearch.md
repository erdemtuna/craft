---
date: 2026-03-06T18:14:00Z
git_commit: 77bd118
branch: feature/craft-resolution
repository: erdemtuna/craft
topic: "Resolution Engine — existing codebase map for Workflow 2 planning"
tags: [research, codebase, manifest, pinfile, skill, cli, resolution]
status: complete
last_updated: 2026-03-06
---

# Research: Craft Resolution Engine — Codebase Map

## Research Question

What existing types, parsers, patterns, and infrastructure does the Resolution Engine (Workflow 2) build upon? Document all integration points, extension surfaces, and conventions.

## Summary

The craft codebase from Workflow 1 provides a complete local-only package management foundation: manifest/pinfile/SKILL.md types with parsers, validators, and serializers; a validation runner; an interactive init wizard; and a Cobra CLI skeleton with three commands (init, validate, version). The Resolution Engine will add new packages (resolve, fetch, install, integrity) and register two new CLI commands (install, update), building on the existing types without modifying them. Key integration surfaces are the `Manifest.Dependencies` map (input), the `Pinfile`/`ResolvedEntry` types (output), and the `skill.ParseFrontmatter` function (discovery). No interfaces exist — Workflow 2 will introduce them for testability.

## Documentation System

- **Framework**: Plain markdown (no docs framework)
- **Docs Directory**: N/A (single README.md at repo root)
- **Navigation Config**: N/A
- **Style Conventions**: Brief README with command table, YAML examples, inline code references
- **Build Command**: N/A
- **Standard Files**: `README.md` at repo root (craft:1-96)

## Verification Commands

- **Test Command**: `go test ./...` (standard Go test runner, all packages)
- **Lint Command**: None configured
- **Build Command**: `go build -o craft ./cmd/craft`
- **Type Check**: Implicit via `go build` / `go vet` (Go compiler)
- **Environment Note**: Go binary at `$HOME/go/bin/go`, must prepend to PATH. Harmless warning about GOPATH==GOROOT.

## Detailed Findings

### Entry Point & CLI Skeleton

The CLI uses Cobra with a root command and three subcommands registered via `init()`.

- **Entry point**: `cmd/craft/main.go:9-12` — calls `cli.Execute()`, exits 1 on error
- **Root command**: `internal/cli/root.go:10-16` — `craft` with `SilenceUsage: true, SilenceErrors: true`
- **Command registration**: `internal/cli/root.go:18-22` — `init()` adds `versionCmd`, `initCmd`, `validateCmd`
- **Execute function**: `internal/cli/root.go:25-31` — prints errors to stderr, returns error

**Subcommand pattern** (all follow the same structure):
- `internal/cli/version.go:8-16` — `cobra.Command` with `Run` (not `RunE`), uses `cmd.Printf` for output
- `internal/cli/init.go:10-24` — `cobra.Command` with `RunE`, gets `os.Getwd()`, creates wizard, calls `wizard.Run()`
- `internal/cli/validate.go:13-48` — `cobra.Command` with `RunE`, gets `os.Getwd()`, creates runner, prints results to stderr

**New commands (install, update) should follow this pattern**: `RunE` function, `os.Getwd()` for root, delegate to internal package, print results. The `--target` flag would be added via `cmd.Flags().StringVar()` on the install and update commands.

### Manifest Types & Parsing

- **Manifest struct**: `internal/manifest/types.go:6-30`
  - `SchemaVersion int` — always 1
  - `Name string` — lowercase alphanumeric with hyphens
  - `Version string` — strict semver MAJOR.MINOR.PATCH
  - `Description string` — optional
  - `License string` — optional
  - `Skills []string` — relative paths to skill directories
  - `Dependencies map[string]string` — **key**: alias, **value**: URL `host/org/repo@vMAJOR.MINOR.PATCH`
  - `Metadata map[string]string` — extensibility

- **Parse functions**: `internal/manifest/parse.go:13-36`
  - `Parse(r io.Reader) (*Manifest, error)` — reads all bytes, yaml.Unmarshal
  - `ParseFile(path string) (*Manifest, error)` — opens file, delegates to Parse

- **Validation**: `internal/manifest/validate.go:23-60`
  - `Validate(m *Manifest) []error` — validates schema_version, name, version, skills, dependency URLs
  - **Dep URL regex** `internal/manifest/validate.go:19`: `^[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+@v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`
  - **Name regex** `internal/manifest/validate.go:11`: `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`
  - **Semver regex** `internal/manifest/validate.go:15`: `^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`
  - `ValidateName(name string) error` — reusable name checker (`internal/manifest/validate.go:64-75`)
  - `ValidateVersion(ver string) error` — reusable version checker (`internal/manifest/validate.go:79-87`)

- **Serialization**: `internal/manifest/write.go:14-47`
  - `Write(m *Manifest, w io.Writer) error` — builds ordered yaml.Node tree, uses encoder with indent 2
  - Helper functions: `addField`, `addStringSlice`, `addStringMap` (sorted keys)

**Resolution Engine integration**: The resolver reads `Manifest.Dependencies` to determine what to fetch. The dep URL format `host/org/repo@vMAJOR.MINOR.PATCH` must be parsed into components (host+org+repo → git URL, version → git tag ref).

### Pinfile Types & Parsing

- **Pinfile struct**: `internal/pinfile/types.go:6-13`
  - `PinVersion int` — always 1
  - `Resolved map[string]ResolvedEntry` — **key**: dependency URL from manifest, **value**: resolved state

- **ResolvedEntry struct**: `internal/pinfile/types.go:16-25`
  - `Commit string` — full git commit SHA (40 hex chars)
  - `Integrity string` — SHA-256 digest string (format: `sha256-<base64>`)
  - `Skills []string` — discovered skill names

- **Parse functions**: `internal/pinfile/parse.go:12-35` — same pattern as manifest (Parse + ParseFile)

- **Validation**: `internal/pinfile/validate.go:9-31`
  - `Validate(p *Pinfile) []error` — checks pin_version, commit/integrity/skills presence per entry

**Resolution Engine integration**: The resolver writes `Pinfile` with `ResolvedEntry` values after resolution. Existing pinfile entries (when manifest deps match) can be reused to skip re-resolution.

### SKILL.md Frontmatter Types & Parsing

- **Frontmatter struct**: `internal/skill/types.go:5-15`
  - `Name string` — skill identifier
  - `Description string` — optional
  - `Extra map[string]interface{}` — forward-compat, tagged `yaml:"-"` (custom parsing)

- **ParseFrontmatter**: `internal/skill/frontmatter.go:16-46`
  - Two-pass parsing: first into `map[string]interface{}` for extras, then into struct
  - Reads `---` delimited YAML block from start of file

- **extractFrontmatter**: `internal/skill/frontmatter.go:61-97`
  - Line-by-line scanner, expects `---` first line, reads until closing `---`
  - Returns raw YAML string between delimiters

- **ParseFrontmatterFile**: `internal/skill/frontmatter.go:49-57` — opens file, delegates to ParseFrontmatter

- **ValidateFrontmatter**: `internal/skill/validate.go:11-26`
  - Validates name is present and matches `manifest.ValidateName` rules

**Resolution Engine integration**: When auto-discovering skills in dependencies without `craft.yaml`, the resolver will use `ParseFrontmatter` (from `io.Reader`, not file-based) to extract skill names from SKILL.md content read from git objects. The `ParseFrontmatter(r io.Reader)` signature supports this directly.

### Validation Runner

- **Runner struct**: `internal/validate/runner.go:16-19` — holds `Root string`
- **Run method**: `internal/validate/runner.go:28-55` — orchestrates all checks sequentially
- **Checks performed**:
  1. `checkManifest` (`internal/validate/runner.go:59-93`) — parse + validate craft.yaml
  2. `checkSkills` (`internal/validate/runner.go:97-218`) — path safety, existence, SKILL.md, frontmatter, duplicate detection
  3. `checkPinfile` (`internal/validate/runner.go:223-288`) — parse + validate pinfile, consistency with manifest deps
  4. `checkNameCollisions` (`internal/validate/runner.go:291-302`) — duplicate skill names across paths

- **Error types**: `internal/validate/errors.go:1-58`
  - `Category` type with 7 constants: schema, skill-path, frontmatter, dependency, pinfile, collision, safety
  - `Error` struct with Category, Path, Field, Message, Suggestion
  - `Warning` struct with Message
  - `Result` struct with `Errors []*Error`, `Warnings []*Warning`, `OK() bool`

**Resolution Engine integration**: The validate runner currently checks local skills only. Workflow 2's collision detection across transitive deps is a separate concern in the resolver, not in the validate runner. The validate runner's `checkPinfile` consistency check will become more relevant once `craft install` generates pinfiles.

### Init Wizard & Skill Discovery

- **Wizard**: `internal/init/wizard.go:15-27` — holds Root, In, Out, ErrOut
- **Wizard.Run**: `internal/init/wizard.go:40-154` — interactive flow with atomic file write (temp+rename)
- **DiscoverSkills**: `internal/init/discover.go:25-75` — recursive WalkDir, finds SKILL.md directories, returns sorted relative paths with `./` prefix

**Resolution Engine integration**: `DiscoverSkills` logic will be reused/adapted for auto-discovering skills in dependency repos that lack `craft.yaml`. The current implementation works on filesystem paths; Workflow 2 may need a variant that works on in-memory file trees from git objects.

### Test Infrastructure & Patterns

**Test data structure**: `testdata/` at repo root
- `testdata/manifests/` — standalone manifest YAML files (minimal, valid, with-extras)
- `testdata/pinfiles/` — standalone pinfile YAML files (valid)
- `testdata/skills/` — standalone SKILL.md files (valid, malformed, missing-name, no-frontmatter, with-extras)
- `testdata/packages/` — complete package directories for integration-style tests
  - `valid/` — valid package with two skills
  - `collision/` — package with duplicate skill names
  - `escape/` — package with path traversal attempt
  - `missing-skill/` — package with non-existent skill directory
  - `pinfile-mismatch/` — package where pinfile doesn't match manifest deps
  - `pinfile-no-deps/` — package with pinfile but no deps (warning case)

**Test helper**: `internal/validate/runner_test.go:11-14` — `testdataDir()` uses `runtime.Caller(0)` to locate testdata relative to source file

**Testing conventions observed**:
- Standard `testing` package, no test frameworks
- Table-driven tests for formatting (`TestErrorFormatting`: `internal/validate/runner_test.go:223-275`)
- `t.TempDir()` for filesystem tests requiring temp state (`TestSymlinkCycle`, `TestDuplicateSkillPath`)
- `testdata/` directory for static fixtures
- Tests in same package (white-box testing) — e.g., `package validate`, `package manifest`
- No mock interfaces — all tests use real file I/O or in-memory readers

### Dependency URL Parsing (Not Yet Implemented)

The dependency URL format `host/org/repo@vMAJOR.MINOR.PATCH` is validated by regex (`internal/manifest/validate.go:19`) but never parsed into components. The Resolution Engine will need a parser that extracts:
- **Git URL**: `https://host/org/repo.git` (for HTTPS) or `git@host:org/repo.git` (for SSH)
- **Version tag**: `vMAJOR.MINOR.PATCH` (the ref to resolve)
- **Alias**: The map key from `Manifest.Dependencies`

### Existing External Dependencies

- `github.com/spf13/cobra v1.10.2` — CLI framework (`go.mod:6`)
- `gopkg.in/yaml.v3 v3.0.1` — YAML parsing/serialization (`go.mod:7`)
- `github.com/inconshreveable/mousetrap v1.1.0` — indirect (Cobra dep) (`go.mod:11`)
- `github.com/spf13/pflag v1.0.9` — indirect (Cobra dep) (`go.mod:12`)

**New dependencies for Workflow 2**: `github.com/go-git/go-git/v5` (and its transitive deps including `golang.org/x/crypto/ssh`).

### Atomic File Write Pattern

Established in Workflow 1 (`internal/init/wizard.go:127-148`):
1. Write to `<path>.tmp`
2. On write error: close file, remove temp, return error
3. On close error: remove temp, return error
4. `os.Rename(tmp, target)` — atomic replace
5. On rename error: remove temp, return error

This pattern should be reused for pinfile writes and cache storage.

## Code References

- `cmd/craft/main.go:9-12` — Entry point, calls cli.Execute()
- `internal/cli/root.go:10-22` — Root Cobra command, subcommand registration
- `internal/cli/init.go:10-24` — Init command handler
- `internal/cli/validate.go:13-48` — Validate command handler
- `internal/cli/version.go:8-16` — Version command handler
- `internal/manifest/types.go:6-30` — Manifest struct definition
- `internal/manifest/parse.go:13-36` — Manifest parser (Parse + ParseFile)
- `internal/manifest/validate.go:11-87` — Manifest validation with regex patterns
- `internal/manifest/write.go:14-104` — Manifest YAML serializer with ordered fields
- `internal/pinfile/types.go:6-25` — Pinfile + ResolvedEntry struct definitions
- `internal/pinfile/parse.go:12-35` — Pinfile parser
- `internal/pinfile/validate.go:9-31` — Pinfile structural validation
- `internal/skill/types.go:5-15` — Frontmatter struct
- `internal/skill/frontmatter.go:16-106` — Frontmatter parser (two-pass, --- delimited)
- `internal/skill/validate.go:11-26` — Frontmatter validation
- `internal/validate/errors.go:1-58` — Error/Warning/Result types with 7 categories
- `internal/validate/runner.go:16-302` — Validation runner with 4 check phases
- `internal/init/discover.go:25-75` — Recursive skill directory discovery
- `internal/init/wizard.go:15-282` — Interactive init wizard with atomic writes
- `internal/version/version.go:7` — Version variable (ldflags override)

## Architecture Documentation

### Package Dependency Graph (Internal)

```
cmd/craft/main.go
  └── internal/cli
        ├── internal/init      (craft init)
        │     └── internal/manifest  (write manifest)
        ├── internal/validate  (craft validate)
        │     ├── internal/manifest  (parse + validate)
        │     ├── internal/pinfile   (parse + validate)
        │     └── internal/skill     (parse + validate frontmatter)
        └── internal/version   (craft version)
```

### Conventions

- **Package naming**: lowercase single words (`manifest`, `pinfile`, `skill`, `validate`)
- **File organization**: `types.go` for structs, `parse.go` for parsing, `validate.go` for validation, `write.go` for serialization
- **Error handling**: Functions return `[]error` for multi-error collection; validators never stop at first error
- **io.Reader/Writer**: All parsers accept `io.Reader`; serializers accept `io.Writer`; file-based variants are thin wrappers
- **Test placement**: Same package (white-box), suffix `_test.go`
- **YAML**: Uses `gopkg.in/yaml.v3` with node API for ordered output, struct tags for parsing

### Extension Surfaces for Workflow 2

1. **CLI registration**: Add `rootCmd.AddCommand(installCmd)` and `rootCmd.AddCommand(updateCmd)` in `internal/cli/root.go:18-22`
2. **Dep URL parsing**: Parse `Manifest.Dependencies` values using the known regex structure
3. **Pinfile writing**: Use `pinfile.Pinfile` and `pinfile.ResolvedEntry` types; need a `Write` function (analogous to `manifest.Write`)
4. **Skill discovery in git**: Adapt `ParseFrontmatter(io.Reader)` for reading SKILL.md from git tree objects
5. **Integrity computation**: New function to compute `sha256-<base64>` digest from skill file contents

## Open Questions

None — all integration points are clearly documented. The codebase provides clean extension surfaces without requiring modifications to existing code.
