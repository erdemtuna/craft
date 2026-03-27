# Subpath Skill Selection Implementation Plan

## Overview

Add subpath-based skill selection to craft so consumers can cherry-pick specific skills from large packages. The feature introduces a structured dependency format in `craft.yaml` (`url` + `select` list), extends `DepURL` with `#subpath` fragment support, and adds resolver-level skill filtering. Interactive skill preview during `craft add` and new-skill discovery during `craft update` round out the user experience.

The implementation follows a bottom-up strategy: foundational type changes first (highest risk, widest impact), then parser, resolver, CLI commands, and finally interactive UI and update notifications.

## Current State Analysis

- **Dependencies are strings**: `Manifest.Dependencies` is `map[string]string` — values are DepURL strings. This type appears in 14 files (~45 individual references).
- **DepURL has no subpath**: The parser splits on `@` only; no `#` fragment handling exists.
- **All skills are installed**: `discoverSkillsForDep()` returns all exported skills. There is no consumer-side filter mechanism.
- **`craft add` has no preview**: Dependencies are added without showing what skills a package contains.
- **`craft update` has no awareness of selections**: It re-resolves but doesn't compare available vs. selected skill sets.

Key constraints from research:
- The codebase uses `gopkg.in/yaml.v3` with `*yaml.Node`-based serialization for full output control (write.go). Custom unmarshalers via `*yaml.Node` parameter are well-supported.
- `DiscoverSkills()` in `discover.go` is already exported and reusable for skill preview.
- The fetcher's `ListTree` cache is per-instance and warms automatically across discovery and resolution calls.

## Desired End State

- A `craft.yaml` with structured dependencies (`url` + `select`) resolves and installs only the selected subset of skills
- `craft get url#subpath` installs a single skill from a multi-skill package
- `craft add` interactively presents available skills for selection in TTY environments
- `craft update` informs users about newly available skills in selectively installed packages
- All existing `craft.yaml` files continue to parse and function identically (backward compatible)
- All existing tests pass; new tests cover structured deps, subpath parsing, filtering, and merging

**Verification approach**: Each phase is verified with `go build ./... && go test ./...`. The final state is verified end-to-end by creating a test `craft.yaml` with mixed string and structured deps, running `craft install`, and confirming only selected skills are present.

## What We're NOT Doing

- Full PURL adoption (`pkg:type/...` format) — deferred per WorkShaping decision
- Glob patterns in `select` (e.g., `skills/doc*`) — explicit paths only
- `exclude` field (inverse of select) — `select` only
- OCI registry support — deferred to PURL adoption
- `select` propagation to transitive dependencies — consumer `select` applies only to direct deps
- Percent-encoding in subpath fragments — skill paths are simple alphanumeric
- Schema version bump — structured format is a compatible extension of schema version 1
- Changes to pinfile format beyond recording filtered skill sets — no new fields needed

## Phase Status

- [x] **Phase 1: Core Type System** - Introduce `DependencySpec`, migrate all consumers from `map[string]string`
- [x] **Phase 2: DepURL Subpath Parsing** - Add `#subpath` fragment support to the URL parser
- [x] **Phase 3: Resolver Filtering** - Filter discovered skills by `select` list, merge selects across MVS
- [x] **Phase 4: CLI `craft get` Subpath** - Parse `#subpath` from CLI argument, create structured dep
- [x] **Phase 5: Interactive `craft add` Preview** - Skill discovery, multi-select UI, `--all` flag
- [ ] **Phase 6: `craft update` Discovery** - Detect and report newly available skills
- [ ] **Phase 7: Documentation** - Docs.md, README updates, CHANGELOG entry

## Phase Candidates

<!-- No candidates identified — all phases are committed -->

---

## Phase 1: Core Type System

**Objective**: Replace `map[string]string` with `map[string]DependencySpec` across the entire codebase. After this phase, structured deps can be declared in YAML but `Select` is not yet used by the resolver. All existing behavior is preserved.

This is the highest-risk phase because it touches 14 files mechanically. It must be done first so subsequent phases can build on the new type.

### Changes Required

- **`internal/manifest/types.go`**: Define `DependencySpec` struct with `URL string` and `Select []string` fields. Implement `UnmarshalYAML(*yaml.Node)` to handle both scalar (string → `DependencySpec{URL: value}`) and mapping (object → full struct) YAML nodes. Change `Manifest.Dependencies` from `map[string]string` to `map[string]DependencySpec`.

- **`internal/manifest/write.go`**: Replace `addStringMap()` call for dependencies with a new `addDependencies()` function. When `Select` is empty, write as scalar value (preserving clean output for simple deps). When `Select` is non-empty, write as mapping with `url` and `select` keys. Follow existing `yaml.Node` serialization patterns.

- **`internal/manifest/validate.go`**: Update dependency iteration to use `dep.URL` instead of `url`. Add select path validation: reject paths with leading `/` (absolute), containing `..` (traversal), or empty strings. Add `"strings"` import.

- **`internal/manifest/parse.go`**: No changes needed — the custom `UnmarshalYAML` on `DependencySpec` is called automatically by the YAML library.

- **Mechanical consumer updates (~45 lines across 14 files)**: Every reference to `m.Dependencies` values as `string` must change to use `.URL`. Files: `resolver.go` (lines 282-296), `add.go`, `get.go`, `update.go`, `remove.go`, `list.go`, `tree.go`, `outdated.go`, `install.go`, `validate/runner.go`, and test files. The CodeResearch §3 documents each line precisely. **Critical note for `update.go:161`**: When updating a dependency URL (e.g., version bump), the existing `Select` must be preserved — construct `DependencySpec{URL: newURL, Select: depSpec.Select}`, not just replace the URL string.

- **Test file updates**: All test files that construct `Manifest` literals with `Dependencies: map[string]string{...}` must change to `map[string]DependencySpec{...}`. Files: `write_test.go`, `parse_test.go`, `validate_test.go`, `remove_test.go`, `update_test.go`. Assertions on dependency values change from `m.Dependencies["x"]` to `m.Dependencies["x"].URL`.

- **New tests**: Round-trip test for structured deps (parse YAML with `url` + `select`, write back, verify identical output). Round-trip test for mixed manifest (some string deps, some structured deps). Validation tests for invalid select paths.

### Success Criteria

#### Automated Verification
- [ ] `go build ./...` compiles cleanly
- [ ] `go test ./...` — all existing tests pass (with updated type literals)
- [ ] New round-trip tests pass for both string and structured dep formats
- [ ] New validation tests pass for select path edge cases

#### Manual Verification
- [ ] Create a `craft.yaml` with a structured dep (`url` + `select`), verify it parses without error
- [ ] Verify `craft validate` accepts the new format
- [ ] Verify simple string deps in existing manifests are unaffected

---

## Phase 2: DepURL Subpath Parsing

**Objective**: Extend `ParseDepURL` to accept `#subpath` fragments. The `Subpath` field is parsed and stored but not yet used by any consumer. This is a low-risk, isolated change.

### Changes Required

- **`internal/resolve/depurl.go`**: Add `Subpath string` field to `DepURL` struct. In `ParseDepURL`, split on `#` before the existing `@` split — extract fragment, strip from raw string, continue with existing parsing. Add `normalizeSubpath()` helper: trim leading `./`, trim trailing `/`, reject empty-after-trim as no subpath. Update `String()` to append `#subpath` when present in the reconstructed form.

- **`internal/resolve/depurl_test.go`**: Add table-driven test cases: URL with tag + subpath, URL with commit + subpath, URL with branch + subpath, URL with empty fragment (`url#` → no subpath), URL with nested subpath (`skills/nested/docx`). Test that `PackageIdentity()` excludes subpath. Test that `String()` reconstructs correctly with subpath. Test that `WithVersion()` drops subpath (version bumps produce clean URLs).

### Success Criteria

#### Automated Verification
- [ ] `go test ./internal/resolve/...` — all parser tests pass including new subpath cases
- [ ] Existing `ParseDepURL` tests still pass (no regression)

#### Manual Verification
- [ ] Parse `github.com/acme/skills@v1.0.0#skills/docx` — verify `Subpath` is `skills/docx` and `PackageIdentity()` is `github.com/acme/skills`

---

## Phase 3: Resolver Filtering

**Objective**: Make the resolver filter discovered skills against `select` lists and merge selects when MVS groups multiple entries for the same package. After this phase, `craft install` with a structured dep containing `select` installs only the selected skills.

### Changes Required

- **`internal/resolve/types.go`**: Add `Select []string` and `AllSkillPaths []string` fields to `ResolvedDep` struct. Both are transient (not persisted in pinfile). `Select` flows through the resolution pipeline and is consumed by `resolveOne()` to filter results. `AllSkillPaths` records the full set of discovered skill paths before filtering, enabling Phase 6's new-skill detection without a second fetch.

- **`internal/resolve/resolver.go`**:
  - In `collectDeps()`: When iterating `m.Dependencies`, thread `depSpec.Select` into `ResolvedDep.Select`.
  - In the MVS loop: After selecting the best version for a package identity, merge `Select` lists from all entries. If any entry has an empty `Select` (meaning "all"), the merged result is empty (all skills). Otherwise, deduplicate the union.
  - In `resolveOne()`: After skill discovery returns `names, paths, files`, store the full discovered set as `dep.AllSkillPaths` (new field on `ResolvedDep`) before filtering. Then apply filtering if `dep.Select` is non-empty. Implement `filterBySelect()` — match select paths against `dep.SkillPaths` with normalization (strip `./` prefix), return filtered names/paths/files. If any select path matches no discovered skill, return an error identifying the invalid path. Note: `AllSkillPaths` is needed by Phase 6 for new-skill discovery without a second fetch pass.
  - **Discovery scope (FR-006)**: When `dep.Select` is non-empty, discovery MUST scan all SKILL.md files in the repository tree (auto-discovery mode via `DiscoverSkills(ListTree)`), regardless of whether the dependency's `craft.yaml` declares a `skills` export list. This ensures the consumer's selection can override the package's exports. When `dep.Select` is empty, use the existing discovery path (manifest exports first, auto-discovery fallback) for backward compatibility.
  - **Integrity (FR-015)**: The filtering happens before `integrity.Digest(skillFiles)` is called, so the digest is automatically computed over only the selected skill files. No additional integrity changes needed.

- **`internal/resolve/resolver_test.go`**: Add tests: `TestFilterBySelect` (3 skills, select 1, verify output), `TestResolveSelectMerge` (two deps same package, different selects, verify union), `TestResolveSelectNotFound` (select path with no match → error), `TestResolveEmptySelect` (empty select → all skills), `TestResolveSelectOverridesExports` (dependency exports only `skills/public` but consumer selects `skills/internal` — verify `skills/internal` is discovered and installed).

### Success Criteria

#### Automated Verification
- [ ] `go test ./internal/resolve/...` — filter and merge tests pass
- [ ] `go test ./...` — full test suite passes

#### Manual Verification
- [ ] Create a test `craft.yaml` with `select: [skills/docx]` for a package with 3 skills, run `craft install`, verify only `docx` is installed
- [ ] Verify pinfile contains only the selected skill in `skills` and `skill_paths`
- [ ] Verify integrity digest matches the selected skill content only

---

## Phase 4: CLI `craft get` Subpath

**Objective**: `craft get github.com/acme/skills@v1.0.0#skills/docx` installs only the `docx` skill globally. Low risk — builds directly on Phases 2 and 3.

### Changes Required

- **`internal/cli/get.go`**: After `ParseDepURL(arg)`, check `parsed.Subpath`. If non-empty, construct a `DependencySpec` with `URL` set to the fragment-stripped URL (`parsed.PackageIdentity() + "@" + parsed.RefString()`) and `Select` set to `[]string{parsed.Subpath}`. If empty, use the existing simple `DependencySpec{URL: arg}` path.

- **`internal/cli/get_test.go`** (or add test cases): Test `craft get` with `#subpath` argument — verify the manifest entry is created with correct `Select` and only the targeted skill is installed.

### Success Criteria

#### Automated Verification
- [ ] `go test ./internal/cli/...` — get command tests pass with subpath
- [ ] `go test ./...` — full suite passes

#### Manual Verification
- [ ] Run `craft get github.com/<test-package>@v1.0.0#skills/docx` against a real or local test package, verify only `docx` is installed in the agent skills directory

---

## Phase 5: Interactive `craft add` Preview

**Objective**: When `craft add` targets a multi-skill package in a TTY environment, present available skills for interactive selection. `--all` flag and non-TTY environments skip the prompt.

### Changes Required

- **`internal/ui/select.go`** (new file): Implement a `MultiSelect` function that presents a numbered list of items with toggle controls. Accepts a list of skill names/paths, returns selected indices. Pattern: print numbered list, user enters numbers or `all`, confirm selection. Keep implementation simple — numbered input rather than arrow-key navigation.

- **`internal/cli/add.go`**:
  - Add `--all` flag to `addCmd`
  - After adding the dependency to the manifest but before resolution, insert a preview flow:
    1. Create a fetcher instance (already done at line ~102)
    2. Use `DiscoverSkills()` from `resolve` package to discover available skills (requires `ListTree` + `ReadFiles` from fetcher)
    3. If multiple skills found AND TTY AND no `--all` flag: present `MultiSelect`
    4. If user selects a subset: set `DependencySpec.Select` to selected paths
    5. If user selects all or `--all` flag or non-TTY: leave `Select` empty (string format)
  - TTY detection: use `term.IsTerminal(int(os.Stdin.Fd()))`, following pattern from `get.go` line 118

- **`internal/ui/select_test.go`** (new file): Unit tests for `MultiSelect` with mock stdin/stdout

### Success Criteria

#### Automated Verification
- [ ] `go build ./...` compiles (new UI package)
- [ ] `go test ./internal/ui/...` — multi-select tests pass
- [ ] `go test ./...` — full suite passes

#### Manual Verification
- [ ] Run `craft add acme github.com/<test-package>@v1.0.0` in a terminal — verify interactive skill list appears
- [ ] Select a subset — verify `craft.yaml` contains structured dep with `select`
- [ ] Select all — verify `craft.yaml` contains simple string dep
- [ ] Run with `--all` flag — verify no prompt appears, all skills installed
- [ ] Pipe input (non-TTY) — verify no prompt, all skills installed

---

## Phase 6: `craft update` Discovery

**Objective**: When updating a selectively installed dependency, inform users about newly available skills upstream. Inform only — no prompting, no auto-install.

### Changes Required

- **`internal/cli/update.go`**: After resolution completes for each dependency, check if the dependency has a `Select` list (from manifest). If so:
  1. Get the full set of discovered skills from `dep.AllSkillPaths` (populated by Phase 3's resolver changes)
  2. Compare against the selected skills in `dep.SkillPaths`
  3. If new skills exist that aren't in the selection, print an informational message: `"ℹ New skills available in %s: %s\n  Use 'craft add' to include them"`
  - No second fetch pass needed — `AllSkillPaths` was stored during resolution in Phase 3.

- **`internal/cli/update_test.go`**: Add test verifying notification message appears when upstream has new skills. Add test verifying no message when skill set is unchanged.

### Success Criteria

#### Automated Verification
- [ ] `go test ./internal/cli/...` — update notification tests pass
- [ ] `go test ./...` — full suite passes

#### Manual Verification
- [ ] Update a selectively installed dependency where upstream added a skill — verify informational message appears
- [ ] Update a dependency with no new skills — verify no extra output

---

## Phase 7: Documentation

**Objective**: Document the subpath skill selection feature for users and maintainers.

### Changes Required

- **`.paw/work/feature-purl/Docs.md`**: Technical reference for the implementation — architecture, key decisions, testing approach (load `paw-docs-guidance` for template)
- **`README.md`**: Add documentation for the structured dependency format, `select` field, `craft get #subpath` syntax, and interactive `craft add` behavior. Follow existing README structure and conventions.
- **`CHANGELOG`** (if project uses one): Entry for the subpath skill selection feature

### Success Criteria

- [ ] Documentation accurately describes the feature
- [ ] README examples are correct and follow existing style
- [ ] No broken links or references

---

## References

- Issue: https://github.com/agentskills/agentskills/discussions/210#discussioncomment-16284365
- Spec: `.paw/work/feature-purl/Spec.md`
- Research: `.paw/work/feature-purl/SpecResearch.md`, `.paw/work/feature-purl/CodeResearch.md`
