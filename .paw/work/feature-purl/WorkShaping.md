# Work Shaping: Subpath-Based Skill Selection (PURL-Informed)

## Problem Statement

Craft currently installs **all** exported skills from a dependency — consumers cannot cherry-pick individual skills from a large repo. When a package exports 12+ skills, users get everything whether they need it or not.

This gap was surfaced by [travbale's comment on the agentskills RFC](https://github.com/agentskills/agentskills/discussions/210#discussioncomment-16284365), which proposed adopting [Package URLs (PURLs)](https://github.com/package-url/purl-spec) in craft. After exploration, the decision is to **not adopt PURL now** but to add subpath-based skill selection using PURL-compatible syntax, keeping PURL as a future option.

**Who benefits:** Consumers depending on large skill packages (monorepos, curated collections) who only need a subset.

## Decision: Why Not Full PURL (Yet)

| Factor | Assessment |
|--------|------------|
| Skills only come from git repos today | PURL's multi-transport value (OCI, npm) isn't needed yet |
| DepURL is deeply embedded (16+ files, 19+ parse sites) | Full replacement is high-cost, low-immediate-value |
| PURL's `#subpath` solves the real problem | We can adopt just the subpath concept with `#` fragment syntax |
| No `pkg:agentskill/` type exists in PURL ecosystem | Ecosystem recognition benefit is theoretical |
| User ergonomics | `github.com/org/repo@v1` is more readable than `pkg:github/org/repo@v1` |

**Door stays open:** The `#` fragment syntax is PURL-compatible. A future PURL adoption would extend the parser, not redesign it.

## Work Breakdown

### Core: Structured Dependencies with `select` (craft.yaml)

**Current format** (string-only):
```yaml
dependencies:
  acme: github.com/acme/skills@v1.0.0
```

**New format** (string OR structured object):
```yaml
dependencies:
  # Simple — install all skills (backward compatible)
  tools: github.com/example/tools@v1.0.0

  # Structured — install only selected skills
  acme:
    url: github.com/acme/skills@v1.0.0
    select:
      - skills/docx
      - skills/pdf
```

- `dependencies` values become `string | object`
- Object form has required `url` (DepURL string) and optional `select` (list of subpaths)
- When `select` is omitted or empty: install all exported skills (current behavior)
- When `select` is present: install only skills whose paths match the listed subpaths

### Core: DepURL Subpath Extension

Add a `Subpath` field to the `DepURL` struct for single-subpath references (e.g., in `craft get`):

```
github.com/acme/skills@v1.0.0#skills/docx
```

- Fragment separator: `#` (PURL-compatible)
- `ParseDepURL` extended to parse optional `#subpath`
- Single subpath per URL (for multi-select, use structured `select` in craft.yaml)

### Core: Resolver Changes

- `Manifest.Dependencies` type changes from `map[string]string` to `map[string]DependencySpec`
- `DependencySpec` is a type that can unmarshal from either a string or an object
- Resolver filters discovered skills against `select` list when present
- Multiple structured entries pointing to same package identity: fetch once, merge selections
- Pinfile records which subpaths were selected (for reproducibility)

### Supporting: Interactive Preview in `craft add`

When adding a dependency to a package with many skills:
```
$ craft add acme github.com/acme/big-package@v1.0.0
📦 acme has 12 skills:
  [ ] docx    [ ] pdf     [ ] xlsx
  [ ] pptx    [ ] csv     [ ] json
  ...
Install all 12? [Y/n/select]
> select
Select skills (space to toggle, enter to confirm):
  [x] docx
  [x] pdf
  [ ] xlsx
  ...
```

- Fetch the package, discover skills, present interactive selection
- If user selects all: write simple string format
- If user selects subset: write structured format with `select`
- Skip interactivity with `--all` flag

### Supporting: `craft get` Subpath Support

```
$ craft get github.com/acme/skills@v1.0.0#skills/docx
```

- Install a single skill from a multi-skill package
- Uses the DepURL `#subpath` extension
- Useful for quick, one-off installs without a manifest

## Edge Cases

| Scenario | Expected Handling |
|----------|-------------------|
| `select` path doesn't match any skill in package | Error: "skill path 'skills/foo' not found in acme" |
| `select` path matches a directory but it has no SKILL.md | Error: "no SKILL.md found at 'skills/foo'" |
| Dependency author removes a skill that consumer selects | Error at resolve time (pinfile integrity mismatch) |
| Multiple deps select different skills from same package | Merge selections, single fetch, install union |
| `select` with auto-discovered package (no craft.yaml) | Works — filter auto-discovered skills by path |
| Empty `select: []` | Treat as "all" (same as omitting `select`) |
| `craft get` with subpath on a package that has craft.yaml | Subpath overrides the package's own `skills` export list |

## Architecture

```
craft.yaml (user)
  │
  ├─ string dep ──────────────────► ParseDepURL ──► DepURL (no subpath)
  │                                                    │
  └─ structured dep ──► DependencySpec                 │
       ├─ url ──────────► ParseDepURL ──► DepURL       │
       └─ select ───────► []string (subpaths)          │
                                                       ▼
                                              Resolver.Collect()
                                                       │
                                              ┌────────┴────────┐
                                              │ Has select?      │
                                              │ Yes: filter      │
                                              │ No: all skills   │
                                              └────────┬────────┘
                                                       ▼
                                              ResolvedDep (with SkillPaths)
                                                       │
                                                       ▼
                                              Installer (unchanged)
```

## Codebase Fit

| Area | Impact |
|------|--------|
| `internal/resolve/depurl.go` | Add `Subpath` field, extend `ParseDepURL` for `#fragment` |
| `internal/manifest/types.go` | New `DependencySpec` type, custom YAML unmarshaler |
| `internal/manifest/parse.go` | Handle string-or-object deserialization |
| `internal/manifest/validate.go` | Validate `select` paths, validate `url` field |
| `internal/manifest/write.go` | Serialize structured deps back to YAML |
| `internal/resolve/resolver.go` | Filter skills by `select` during discovery |
| `internal/resolve/discover.go` | Accept subpath filter parameter |
| `internal/pinfile/types.go` | Record selected paths in `ResolvedEntry` |
| `internal/cli/add.go` | Interactive skill preview/selection UX |
| `internal/cli/get.go` | Parse `#subpath` from URL argument |
| `internal/cli/install.go` | Pass select filters to resolver |
| Tests (all above) | New test cases for structured deps, subpath parsing, filtering |

**Reuse opportunities:**
- `DepURL.Subpath` reuses existing `ParseDepURL` pipeline — minimal parser change
- Skill filtering can reuse existing `discoverSkillsForDep` with a path allowlist
- Interactive selection can reuse `internal/ui/` progress/tree formatting

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Manifest format change breaks existing craft.yaml files | **High** | String format remains valid — this is purely additive |
| Structured deps complicate validation logic | Medium | Custom YAML unmarshaler isolates complexity |
| Subpath matching ambiguity (relative vs absolute, trailing slash) | Medium | Normalize paths, document exact matching rules |
| Interactive preview slows down non-interactive CI/CD usage | Low | `--all` flag bypasses interactivity; detect non-TTY |
| Future PURL adoption still requires significant work | Low | Accepted tradeoff — subpath syntax is compatible |

## Critical Analysis

**Value assessment:** High. Subset selection is the #1 missing feature for consuming large skill packages. The agentskills RFC has this as an explicit open question, and craft can lead by example.

**Build vs. modify:** This is a modify — extending existing systems rather than replacing them. The DepURL struct gets one new field; the manifest format gains one new shape; the resolver adds one filter step.

**Complexity budget:** The structured `DependencySpec` type is the biggest new concept. Custom YAML unmarshaling (string → simple, object → structured) is well-trodden Go pattern. Everything else is incremental.

## Resolved Questions

1. **Glob patterns in `select`?** → **No.** Explicit paths only. Predictability over convenience — avoids accidental skill creep from upstream changes.
2. **`exclude` counterpart?** → **No.** `select` only. One mechanism, no ambiguity. Users who want most skills list them explicitly.
3. **Pinfile key format?** → **Merged.** One package = one pinfile entry. Selections from multiple aliases are unioned. The pinfile records what was resolved; the manifest records who asked for what.
4. **`craft update` and new skills?** → **Inform only.** Show newly available skills with a hint (`Use 'craft add acme --select xlsx'`) but don't prompt or auto-install.

## Session Notes

- **Explored PURL adoption**: Decided against full adoption due to high integration cost vs. limited immediate benefit. Skills ecosystem is git-only today.
- **PURL compatibility preserved**: Using `#` fragment syntax ensures future PURL adoption path remains open.
- **Structured deps chosen over multi-entry**: User preferred `select` list in an object over multiple craft.yaml entries pointing to same package. Cleaner UX for multi-selection.
- **Interactive preview chosen**: User wants `craft add` to show available skills and let users select interactively, rather than silently installing everything.
- **No interface abstraction now**: DepURL stays a concrete struct. Refactor to interface only when/if PURL is actually adopted.
- **Originated from**: [agentskills/agentskills#210 comment by travbale](https://github.com/agentskills/agentskills/discussions/210#discussioncomment-16284365)
