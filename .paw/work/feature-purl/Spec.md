# Feature Specification: Subpath Skill Selection

**Branch**: feature/purl  |  **Created**: 2026-03-27  |  **Status**: Draft
**Input Brief**: Enable consumers to cherry-pick individual skills from large packages instead of installing the entire exported set

## Overview

Craft currently operates on an all-or-nothing model for skill dependencies: when a consumer depends on a package, every exported skill in that package is resolved and installed. This works well for small, focused packages, but falls short for larger collections — monorepos with dozens of skills, curated skill libraries, or organizational skill bundles. A consumer who needs only a PDF-generation skill from a 20-skill productivity suite has no way to express that intent today.

Subpath skill selection introduces a mechanism for consumers to declare exactly which skills they want from a dependency. In the manifest (`craft.yaml`), a dependency can now be either a simple URL string (existing behavior, installs all skills) or a structured object with a URL and a `select` list specifying skill directory paths. This preserves full backward compatibility while giving consumers precise control.

The feature extends to the CLI as well. `craft get` gains fragment syntax (`#subpath`) for quick single-skill installs from the command line, and `craft add` presents an interactive preview of available skills when adding a multi-skill package, letting users choose which skills to include. The syntax for subpath fragments uses `#` — deliberately aligned with the PURL specification (ECMA-427) so that future PURL adoption remains a smooth migration path rather than a redesign.

## Objectives

- Enable consumers to install a specific subset of skills from any dependency, reducing unnecessary skill clutter in their agent environments
- Preserve full backward compatibility — existing `craft.yaml` files and workflows continue to work without modification
- Provide an interactive skill discovery experience during `craft add` so users can see what a package offers before deciding what to install
- Surface newly available skills during `craft update` so users stay informed without being nagged
- Align subpath syntax with PURL's `#subpath` convention (ECMA-427) to keep future PURL adoption viable

## User Scenarios & Testing

### User Story P1 – Selective Dependency Installation
Narrative: A developer maintains a workflow that depends on a large organizational skill package containing 15 skills. They only need 3 of them. They declare `select` in their `craft.yaml` and run `craft install`. Only the 3 selected skills are resolved, integrity-checked, and installed.

Independent Test: Add a structured dependency with `select: [skills/docx, skills/pdf]` to `craft.yaml`, run `craft install`, and verify only `docx` and `pdf` skills are installed to the forge directory.

Acceptance Scenarios:
1. Given a `craft.yaml` with a structured dependency selecting 2 of 5 available skills, When `craft install` is run, Then only the 2 selected skills are installed and the pinfile records only those skills
2. Given a `craft.yaml` with a simple string dependency (no select), When `craft install` is run, Then all exported skills are installed (unchanged behavior)
3. Given a structured dependency where a selected path does not match any skill in the package, When `craft install` is run, Then resolution fails with an error identifying the invalid path
4. Given a structured dependency with `select: []` (empty list), When `craft install` is run, Then all exported skills are installed (same as omitting select)

### User Story P2 – Quick Single-Skill Install via CLI
Narrative: A user discovers a specific skill they want from a large package. They run `craft get` with a `#subpath` fragment to install just that one skill globally without creating a manifest.

Independent Test: Run `craft get github.com/acme/skills@v1.0.0#skills/docx` and verify only the `docx` skill is installed to the agent's skill directory.

Acceptance Scenarios:
1. Given a package URL with `#skills/docx` fragment, When `craft get <url>` is run, Then only the `docx` skill is installed globally
2. Given a package URL without a fragment, When `craft get <url>` is run, Then all exported skills are installed (unchanged behavior)
3. Given a `#subpath` that matches no skill in the package, When `craft get <url>` is run, Then the command fails with a clear error

### User Story P3 – Interactive Skill Preview on Add
Narrative: A developer runs `craft add` for a new dependency. Before committing to the dependency, they see a list of all available skills and interactively choose which ones to include. The manifest is written with the appropriate format based on their selection.

Independent Test: Run `craft add acme github.com/acme/skills@v1.0.0` in a TTY and verify an interactive skill list is presented for selection.

Acceptance Scenarios:
1. Given a TTY terminal and a package with 5 skills, When `craft add` is run, Then an interactive skill list is displayed with selection controls
2. Given the user selects 2 of 5 skills in the interactive prompt, When the selection is confirmed, Then `craft.yaml` is written with a structured dependency containing `select: [path1, path2]`
3. Given the user selects all skills in the interactive prompt, When the selection is confirmed, Then `craft.yaml` is written with a simple string dependency (no select)
4. Given a non-TTY environment (CI/CD), When `craft add` is run, Then the interactive prompt is skipped and all skills are installed
5. Given the `--all` flag, When `craft add --all` is run, Then the interactive prompt is skipped and all skills are installed

### User Story P4 – Discover New Skills on Update
Narrative: A developer has selectively installed 3 skills from a package. When they run `craft update`, the upstream version has added 2 new skills. The update succeeds for the selected skills and informs the user about the newly available ones.

Independent Test: Run `craft update` on a dependency whose upstream has added new skills, and verify the output mentions the new skills with instructions on how to add them.

Acceptance Scenarios:
1. Given a selectively installed dependency where upstream has added new skills, When `craft update` is run, Then selected skills are updated and new skills are listed as available
2. Given a selectively installed dependency where upstream has not added new skills, When `craft update` is run, Then the update proceeds normally with no additional output
3. Given a dependency installed without selection (all skills), When `craft update` is run, Then behavior is unchanged from current

### User Story P5 – Multi-Alias Select Merging
Narrative: A team's `craft.yaml` has two aliases pointing to the same package with different selections (e.g., `docs` selects `docx, pdf` and `data` selects `xlsx`). Resolution merges the selections and produces a single pinfile entry.

Independent Test: Create two manifest entries for the same package with different `select` lists, run `craft install`, and verify all selections are installed with one merged pinfile entry.

Acceptance Scenarios:
1. Given two manifest entries for the same package with different select lists, When `craft install` is run, Then the union of all selected skills is installed
2. Given two manifest entries where one has select and the other has no select (all), When `craft install` is run, Then all skills are installed (empty select = all wins)

### Edge Cases
- Structured dependency with `url` field but no `select` field: installs all skills (same as string format)
- Select path with leading `./` prefix: normalized to relative path (stripped)
- Select path with trailing `/`: normalized (stripped)
- Select path with `..` components: rejected by validation as path traversal
- Select path with leading `/`: rejected by validation as absolute path
- Package has no `craft.yaml` (auto-discovered skills): select filtering still works against discovered paths
- Select path matches a directory that exists but contains no `SKILL.md`: rejected with an error (same as non-matching path)
- Select/subpath overrides a package's own `skills` export list: if a consumer selects `skills/internal-tool` but the package only exports `skills/public-tool`, the consumer's selection wins — filtering applies against all discoverable skills in the repo, not just the package's declared exports
- Branch name containing `#` character: unsupported, documented limitation

## Requirements

### Functional Requirements

- FR-001: The `DepURL` parser accepts an optional `#subpath` fragment after the version/ref component, storing it in a `Subpath` field (Stories: P2)
- FR-002: The manifest supports two dependency formats — a string (existing) and a structured object with `url` (required) and `select` (optional list of subpath strings) (Stories: P1, P3, P5)
- FR-003: Manifest parsing uses a custom YAML unmarshaler to transparently handle both string and object dependency formats (Stories: P1, P3)
- FR-004: Manifest validation rejects select paths that are absolute, contain `..`, or are otherwise invalid (Stories: P1)
- FR-005: Manifest serialization preserves format fidelity — simple dependencies write as strings, structured dependencies write as objects (Stories: P1, P3)
- FR-006: The resolver filters discovered skills against the `select` list, scanning all discoverable skills in the repository (not just the package's declared exports), returning only matching skills for integrity computation and installation (Stories: P1, P2)
- FR-007: The resolver fails with an error if any selected path does not match a discovered skill in the package (Stories: P1, P2)
- FR-008: When multiple manifest entries reference the same package identity, MVS selects one version and unions their `select` lists. If any entry has an empty select, the merged result is "all skills" (Stories: P5)
- FR-009: The pinfile records the filtered skill set (skills and skill paths) in a single merged entry per package identity (Stories: P1, P5)
- FR-010: `craft add` fetches available skills and presents an interactive selection prompt when in a TTY environment with a multi-skill package (Stories: P3)
- FR-011: `craft add` accepts an `--all` flag to skip interactive selection and install all skills (Stories: P3)
- FR-012: `craft add` auto-detects non-TTY environments and defaults to installing all skills without prompting (Stories: P3)
- FR-013: `craft get` parses `#subpath` from the URL argument and creates a structured dependency with the subpath as a single-element `select` list (Stories: P2)
- FR-014: `craft update` compares the full set of available skills against the selected set and informs the user about newly available skills (Stories: P4)
- FR-015: Integrity digests are computed over only the selected skill files, so the digest changes when the selection changes (Stories: P1)

### Key Entities

- **DependencySpec**: A manifest-level type representing a dependency as either a simple URL string or a structured object with `url` and `select` fields
- **Subpath**: An optional fragment component of a DepURL, representing a relative path within a package (e.g., `skills/docx`)
- **Select list**: An ordered list of subpath strings in a structured dependency that filters which skills are installed from a package

### Cross-Cutting / Non-Functional

- Backward compatibility: All existing `craft.yaml` files with string-only dependencies must parse and function identically to current behavior
- Schema version: No bump — the structured format is a compatible extension of schema version 1
- Path normalization: Leading `./`, trailing `/` are stripped; paths are matched case-sensitively
- PURL alignment: The `#` fragment separator and relative subpath format align with ECMA-427 Section 5.6.7

## Success Criteria

- SC-001: A structured dependency with `select` listing 2 of 5 available skills results in exactly 2 skills installed, with matching pinfile entries (FR-001, FR-002, FR-006, FR-009)
- SC-002: An existing `craft.yaml` with string-only dependencies produces identical resolution and installation results as before this change (FR-002, FR-003, FR-005)
- SC-003: A `craft get` command with `#subpath` installs exactly one skill from a multi-skill package (FR-001, FR-013)
- SC-004: Running `craft add` in a TTY on a package with 3+ skills presents an interactive selection prompt; selecting a subset writes a structured dependency with `select` (FR-010, FR-011)
- SC-005: A selected path that matches no skill in the package causes resolution to fail with a descriptive error (FR-007)
- SC-006: Two manifest entries selecting different skills from the same package produce a single pinfile entry with the union of selected skills (FR-008, FR-009)
- SC-007: `craft update` on a selectively installed dependency where upstream added a new skill outputs an informational message naming the new skill (FR-014)
- SC-008: Running `craft add` with `--all` or in a non-TTY environment skips the interactive prompt and installs all skills (FR-011, FR-012)

## Assumptions

- Skill directory paths within packages use simple alphanumeric names with hyphens and slashes — no percent-encoding support is needed for the initial implementation
- Git branch names containing `#` are extremely rare; the `#` fragment separator will not conflict with practical branch naming
- The interactive selection UI in `craft add` can be implemented with existing terminal UI patterns in the codebase (`internal/ui/`) or standard Go terminal libraries
- Transitive dependency skill selection is not needed — `select` applies only to direct dependencies; transitive dependencies are resolved in full per their own `craft.yaml`

## Scope

In Scope:
- `#subpath` fragment support in DepURL parser
- Structured dependency format (`url` + `select`) in `craft.yaml`
- Custom YAML unmarshaler for string-or-object dependency values
- Manifest validation for select paths
- Resolver-level skill filtering by select paths
- Select list merging for multi-alias same-package references
- Interactive skill preview/selection in `craft add`
- `--all` flag and non-TTY detection for `craft add`
- `#subpath` support in `craft get`
- Informational new-skill notification in `craft update`
- Pinfile recording of filtered skill sets

Out of Scope:
- Full PURL adoption (`pkg:type/...` format) — deferred to future work
- Glob patterns in `select` (e.g., `skills/doc*`) — explicit paths only
- `exclude` field (inverse of select) — `select` only
- OCI registry support — deferred to PURL adoption
- `select` propagation to transitive dependencies
- Percent-encoding in subpath fragments
- Schema version bump

## Dependencies

- `gopkg.in/yaml.v3` custom `UnmarshalYAML` support via `*yaml.Node` (already in use)
- `golang.org/x/term` for TTY detection in `craft add` interactive mode (check if already a dependency)
- Existing `internal/resolve.DiscoverSkills()` function for skill preview in `craft add`

## Risks & Mitigations

- **Wide type change across codebase**: Changing `Dependencies map[string]string` to `map[string]DependencySpec` touches 10+ files. Mitigation: `DependencySpec.URL` is a drop-in replacement for the string value in all read paths; mechanical refactor.
- **Custom YAML unmarshaler edge cases**: YAML anchors, aliases, or unexpected node types interacting with the custom unmarshaler. Mitigation: Comprehensive test coverage for parse edge cases.
- **Select path matching ambiguity**: Different path normalizations (`./`, trailing `/`) could cause false mismatches. Mitigation: Normalize both select paths and discovered paths using the same function before comparison.
- **Interactive UI complexity**: Building a multi-select terminal UI. Mitigation: Start with simple numbered list + space-to-toggle pattern; enhance later if needed.

## References

- Issue: https://github.com/agentskills/agentskills/discussions/210#discussioncomment-16284365
- PURL Specification: https://github.com/package-url/purl-spec (ECMA-427)
- Work Shaping: .paw/work/feature-purl/WorkShaping.md
- Research: .paw/work/feature-purl/SpecResearch.md
