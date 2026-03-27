# Spec Research: Subpath Skill Selection

## Summary

This document provides deep code-level analysis of every codebase component affected by the subpath skill selection feature. The feature adds `#subpath` fragment support to DepURL, introduces a `DependencySpec` type that can unmarshal from either a string or an object in craft.yaml, and extends the resolver to filter discovered skills by `select` lists. All existing formats remain backward-compatible.

The codebase is well-structured with clear separation: `DepURL` handles parsing, `manifest` handles serialization, `resolve` handles discovery+MVS, `pinfile` records state, `install` handles file layout, and CLI commands orchestrate flows. Each layer has a clean injection point for the new subpath functionality.

---

## DepURL Analysis

### Current Structure (`internal/resolve/depurl.go`)

The `DepURL` struct (lines 40–63) has 7 fields:

```go
type DepURL struct {
    Raw     string   // Original URL string
    Host    string   // e.g., "github.com"
    Org     string   // e.g., "example"
    Repo    string   // e.g., "skills"
    Version string   // Semver without 'v' prefix (tag refs only)
    Ref     string   // Commit SHA or branch name
    RefType RefType  // tag, commit, or branch
}
```

### Parsing Logic (`ParseDepURL`, lines 72–115)

1. Splits on first `@` → `identity` (before) and `ref` (after)
2. Validates identity via `hostOrgRepoPattern` regex (line 25): `^([a-zA-Z0-9](...)?)/([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)$`
3. Classifies ref:
   - `branch:` prefix → `RefTypeBranch`
   - Matches `semverRefPattern` → `RefTypeTag`
   - Hex string 7–64 chars → `RefTypeCommit`
   - Otherwise → error

### Changes Needed for Subpath

**Add `Subpath` field to struct:**
```go
Subpath string // Optional subpath fragment (e.g., "skills/docx")
```

**Extend `ParseDepURL`:**
Before splitting on `@`, check for `#` fragment:
```go
// Before atIdx logic:
subpath := ""
if hashIdx := strings.Index(raw, "#"); hashIdx >= 0 {
    subpath = raw[hashIdx+1:]
    raw = raw[:hashIdx]  // strip fragment for remaining parsing
}
```

Key considerations:
- Must parse `#` **before** `@` since the subpath is after `@ref`: `host/org/repo@v1.0.0#skills/docx`
- Actually, the order in the URL is: `identity@ref#subpath`. So split on `#` first from the full raw string, then proceed with existing `@` logic on the prefix.
- `Subpath` should be normalized: no leading `/`, no trailing `/`, no `./` prefix.
- Empty fragment (`url#`) should be treated as no subpath.

**Affected methods:**
- `String()` (line 173): Must append `#subpath` if present. Currently returns `Raw` if set, or reconstructs. The `Raw` field will naturally include the fragment. But reconstructed form needs: `d.PackageIdentity() + "@" + d.RefString() + "#" + d.Subpath`.
- `WithVersion()` (line 167): Does NOT include subpath — it returns a bare URL for version comparison. No change needed.
- `PackageIdentity()` (line 120): Unchanged — subpath is not part of identity.
- `GitRef()` (line 126): Unchanged — subpath is not part of the git ref.

**Impact on `depURLPattern` in `validate.go`:**
The manifest validation regex (line 15 of `validate.go`) will need to allow optional `#subpath` fragment OR validation needs to work with the `DependencySpec.URL` field which will NOT contain fragments (fragments only appear in `craft get` CLI args and in the `DependencySpec.Select` list).

**Important architectural note:** Per WorkShaping, `#subpath` is used in two places:
1. `craft get github.com/acme/skills@v1.0.0#skills/docx` — CLI argument, single subpath via DepURL
2. `select: [skills/docx, skills/pdf]` — manifest structured dep, multiple subpaths in DependencySpec

The DepURL `Subpath` field serves case 1. Case 2 uses `DependencySpec.Select`. They should not overlap — a DepURL in a manifest's `DependencySpec.URL` should NOT contain a `#fragment`.

### Test Impact (`internal/resolve/depurl_test.go`)

Current tests (lines 5–251) use table-driven pattern with `tests := []struct{ name, input string; want *DepURL; wantErr bool }`. New test cases needed:
- `"url with subpath"` → `github.com/acme/skills@v1.0.0#skills/docx`
- `"url with nested subpath"` → `github.com/acme/skills@v1.0.0#skills/nested/docx`
- `"url with empty fragment"` → `github.com/acme/skills@v1.0.0#` → subpath should be ""
- `"subpath with commit ref"` → `github.com/acme/tools@abc1234#tools/x`
- `"subpath with branch ref"` → `github.com/acme/tools@branch:main#tools/x`
- Error: `"subpath with leading slash"` → should normalize or reject

---

## Manifest Analysis

### Current Dependencies Type (`internal/manifest/types.go`)

```go
Dependencies map[string]string `yaml:"dependencies,omitempty"`
```

This is a simple `map[string]string` where keys are aliases and values are DepURL strings. YAML unmarshal handles this natively.

### Custom YAML Unmarshal Design

The `Dependencies` field type must change to `map[string]DependencySpec`. The `DependencySpec` type needs a custom `UnmarshalYAML` to handle both forms:

```yaml
# String form (backward compatible):
dependencies:
  tools: github.com/example/tools@v1.0.0

# Object form (new):
dependencies:
  acme:
    url: github.com/acme/skills@v1.0.0
    select:
      - skills/docx
      - skills/pdf
```

**Proposed `DependencySpec` type:**
```go
type DependencySpec struct {
    URL    string   // DepURL string (always present)
    Select []string // Optional subpath filter list
}

func (d *DependencySpec) UnmarshalYAML(value *yaml.Node) error {
    if value.Kind == yaml.ScalarNode {
        d.URL = value.Value
        return nil
    }
    if value.Kind == yaml.MappingNode {
        // Decode as struct
        type raw struct {
            URL    string   `yaml:"url"`
            Select []string `yaml:"select"`
        }
        var r raw
        if err := value.Decode(&r); err != nil {
            return err
        }
        d.URL = r.URL
        d.Select = r.Select
        return nil
    }
    return fmt.Errorf("dependency must be a string or object, got %v", value.Kind)
}
```

The `gopkg.in/yaml.v3` library supports custom unmarshalers via `*yaml.Node` parameter. The codebase already uses `yaml.v3` (see `parse.go` line 8, `write.go` line 8). The `yaml.Node` approach gives access to `Kind` to distinguish scalar vs mapping nodes.

### Parse Impact (`internal/manifest/parse.go`)

`Parse()` (lines 13–25) uses `yaml.Unmarshal(data, &m)`. The custom `UnmarshalYAML` on `DependencySpec` will be called automatically — no changes needed in `parse.go` itself. The YAML library handles dispatching to the custom unmarshaler.

### Validate Impact (`internal/manifest/validate.go`)

The validation loop (lines 55–59) currently iterates `m.Dependencies` as `map[string]string`:
```go
for alias, url := range m.Dependencies {
    if !depURLPattern.MatchString(url) { ... }
}
```

This changes to:
```go
for alias, dep := range m.Dependencies {
    if !depURLPattern.MatchString(dep.URL) { ... }
    // Validate select paths: no leading /, no .., no absolute paths
    for _, sel := range dep.Select {
        if strings.HasPrefix(sel, "/") || strings.Contains(sel, "..") {
            errs = append(errs, ...)
        }
    }
}
```

The `depURLPattern` regex (line 15) should NOT be changed to allow `#` fragments — manifest URLs should never contain fragments. Subpaths go in the `select` list.

### Write Impact (`internal/manifest/write.go`)

`Write()` uses `addStringMap()` (line 31) for dependencies. This writes a flat `map[string]string`. For `DependencySpec`, we need a new serialization function:

```go
func addDependencies(mapping *yaml.Node, key string, deps map[string]DependencySpec) {
    // For each dep:
    //   - If Select is empty, write as scalar value (string)
    //   - If Select is non-empty, write as mapping with url + select
}
```

This preserves clean output: simple deps stay as `key: value`, structured deps get the object form. The `Write()` function already uses `yaml.Node` for full control over output structure (lines 16–46), so this fits naturally.

### Backward Compatibility

- Existing `craft.yaml` files with `dependencies: map[string]string` continue to parse correctly (the `UnmarshalYAML` scalar path handles them).
- `Write()` outputs simple deps as strings, so round-tripping preserves the format.
- The `Manifest` struct's YAML tag doesn't change: `yaml:"dependencies,omitempty"`.

---

## Resolver Analysis

### Skill Discovery Flow (`internal/resolve/resolver.go`)

The key flow is in `resolveOne()` (lines 341–378):

1. Parse DepURL
2. Check pinfile reuse (lines 348–357)
3. Resolve commit SHA via `fetcher.ResolveRef()` (line 361)
4. **Discover skills** via `discoverSkillsForDep()` (line 368) → returns `names, paths, files`
5. Set `dep.Skills`, `dep.SkillPaths`, compute `dep.Integrity`

`discoverSkillsForDep()` (lines 382–399):
1. Try reading `craft.yaml` from the dep → if it has `Skills` list, use `discoverFromManifestSkills()`
2. Fallback: `autoDiscoverSkills()` scans for `SKILL.md` files

### Where to Inject Subpath Filtering

**Option A (recommended): Filter after discovery in `resolveOne()`.**

After line 373 (`dep.Skills = skillNames`), add filtering:

```go
if len(selectPaths) > 0 {
    dep.Skills, dep.SkillPaths, skillFiles = filterBySelect(
        dep.Skills, dep.SkillPaths, skillFiles, selectPaths,
    )
}
```

This keeps the discovery layer unchanged and adds a clean filter step. The `selectPaths` must be threaded through from the manifest's `DependencySpec.Select`.

**Threading `select` to `resolveOne()`:**

The `collectDeps()` function (lines 274–338) creates `ResolvedDep` structs from `m.Dependencies`. Currently line 282 iterates `alias, depURL` as strings. With `DependencySpec`, this becomes `alias, depSpec`, and the `ResolvedDep` would need a new field:

```go
type ResolvedDep struct {
    // ... existing fields ...
    Select []string // Subpath filter from manifest (empty = all)
}
```

Or better: keep `ResolvedDep` focused on resolved state and pass `Select` via a separate map through `ResolveOptions`.

**Practical approach:** Add `Select []string` to `ResolvedDep`. It's transient (not persisted in pinfile) but flows through the resolution pipeline. The resolver uses it in `resolveOne()` to filter. The pinfile records the **result** (filtered skills) not the filter itself.

### MVS Implications

MVS groups by `PackageIdentity()` (lines 82–89). When multiple manifest entries point to the same package:
- MVS selects one version (highest for tags)
- The `Select` lists from different entries should be **unioned** (per WorkShaping)

In the MVS loop (lines 92–166), after selecting the best version, merge `Select` lists:
```go
// After selecting best:
var merged []string
for _, dep := range deps {
    merged = append(merged, dep.Select...)
}
selected[identity].Select = dedupe(merged)
```

If any entry has empty `Select` (meaning "all"), the merged result should also be "all" (empty).

### `collectDeps()` Changes

Line 282: `for alias, depURL := range m.Dependencies` must change to handle `DependencySpec`:
```go
for alias, depSpec := range m.Dependencies {
    parsed, err := ParseDepURL(depSpec.URL)
    // ...
    dep := ResolvedDep{
        URL:    depSpec.URL,
        Alias:  alias,
        Source: source,
        RefType: parsed.RefType,
        Select: depSpec.Select,
    }
}
```

For transitive dependencies (parsing a dep's own `craft.yaml`), the `Select` from the *consumer's* manifest does NOT propagate — the transitive dep's own skills list is used unchanged. Only direct dependency `Select` applies.

### `DiscoverSkills` in `discover.go`

The standalone `DiscoverSkills()` function (lines 17–69) is used independently of the resolver. It returns `[]DiscoveredSkill`. This function doesn't need modification — filtering happens at a higher level.

---

## Pinfile Analysis

### Current Structure (`internal/pinfile/types.go`)

```go
type Pinfile struct {
    PinVersion int                        `yaml:"pin_version"`
    Resolved   map[string]ResolvedEntry   `yaml:"resolved"`
}

type ResolvedEntry struct {
    Commit     string   `yaml:"commit"`
    RefType    string   `yaml:"ref_type,omitempty"`
    Integrity  string   `yaml:"integrity"`
    Source     string   `yaml:"source,omitempty"`
    Skills     []string `yaml:"skills"`
    SkillPaths []string `yaml:"skill_paths,omitempty"`
}
```

The pinfile key is the full DepURL string (e.g., `"github.com/acme/skills@v1.0.0"`).

### Recording Selected Paths

**Option A: Add `SelectedPaths` field to `ResolvedEntry`:**
```go
SelectedPaths []string `yaml:"selected_paths,omitempty"`
```

This records what the user selected, separate from `SkillPaths` (which records what was discovered and installed). Purpose: allows `craft update` to know which skills to re-check.

**Option B (simpler): The `Skills` and `SkillPaths` already reflect the filtered set.**

After filtering, `dep.Skills` and `dep.SkillPaths` contain only the selected skills. The pinfile already records these. So the pinfile naturally captures the filter result without needing the filter input.

However, per WorkShaping, `craft update` should "inform about new skills." To do this, `craft update` needs to know the full available set vs. the selected set. This could be done by:
1. Re-discovering all skills at update time (fetch+scan)
2. Comparing against the pinfile's `Skills` list
3. Reporting any new skills not in the list

This works without storing `SelectedPaths` in the pinfile — the manifest's `select` list is the source of truth for what was selected, and the pinfile records the result.

### Merge Semantics

Per WorkShaping: "One package = one pinfile entry. Selections from multiple aliases are unioned."

The pinfile is keyed by DepURL (e.g., `github.com/acme/skills@v1.0.0`). If two manifest aliases both point to the same package (after MVS), they produce one pinfile entry with the union of selected skills.

The current pinfile write logic (resolver.go lines 256–271) creates one entry per `ResolvedDep`. Since MVS already deduplicates to one entry per package identity, this naturally produces merged entries. The `Skills`/`SkillPaths` on the `ResolvedDep` would already be the union after MVS merging.

### Integrity with Subpath Selection

Currently, `integrity.Digest(skillFiles)` computes over ALL skill files. With subpath filtering, only selected skills' files are included. This means:
- The integrity digest changes when the selection changes (correct behavior — different content).
- If a user adds a new `select` entry, the pinfile integrity will change, triggering a re-resolve.

---

## Installer Analysis

### Composite Keys (`internal/install/installer.go`)

**`Install()`** (lines 17–88): Takes `skills map[string]map[string][]byte` where keys are composite keys like `github.com/org/repo/skill-name`. Files are written to `target/compositeKey/`.

**`CompositeKey()`** (line 119): `packageIdentity + "/" + skillName` — e.g., `"github.com/org/repo/my-skill"`.

**`FlatKey()`** (line 96): Converts `/` to `--` for global installs: `"github.com--org--repo--my-skill"`.

### Impact of Subpath on Naming

**No impact.** The installer uses `skillName` (from `dep.Skills[i]`), not the subpath. The subpath is the *directory path within the repo* (e.g., `skills/docx`), while the skill name comes from `SKILL.md` frontmatter (e.g., `docx`).

Example: For a skill at `skills/docx/SKILL.md` with `name: docx`:
- `CompositeKey("github.com/acme/big", "docx")` → `"github.com/acme/big/docx"`
- `FlatKey(...)` → `"github.com--acme--big--docx"`

This is the same regardless of whether the skill was selected via subpath or installed as part of the full package. The installer doesn't know or care about selection.

### `collectSkillFiles()` in `install.go` (lines 365–411)

This function iterates `result.Resolved`, fetches skill files, and maps them by composite key. No changes needed — it already uses `dep.Skills[i]` and `dep.SkillPaths[i]` which will be the filtered set after subpath selection.

---

## CLI Analysis

### `craft add` Flow (`internal/cli/add.go`)

Current flow (lines 38–187):
1. Parse args (alias + URL)
2. Validate URL via `ParseDepURL()`
3. Derive alias from repo name if not provided
4. Parse existing `craft.yaml`
5. Check for existing dependency (update vs. add)
6. Add to `m.Dependencies[alias] = depURL`
7. Resolve to verify
8. Write manifest atomically
9. Optionally install (`--install` flag)

**Interactive preview injection point: Between steps 6 and 7.**

After adding the dependency but before resolving, fetch the package and discover available skills:

```go
// After line 98: m.Dependencies[alias] = depURL
// 1. Fetch package
// 2. Discover all skills (using existing discoverSkillsForDep or similar)
// 3. If multiple skills found AND stdin is TTY:
//    - Present interactive selection UI
//    - If user selects subset: write structured DependencySpec with select
//    - If user selects all: write simple string format
// 4. If --all flag: skip interactivity, install all
```

**Flag design:**
- `--all` flag: skip interactive preview, install all skills (useful for CI/CD)
- Detect non-TTY (line 118 of `get.go` shows the pattern: `term.IsTerminal(int(os.Stdin.Fd()))`)

**Writing structured deps:**
Currently line 98 does `m.Dependencies[alias] = depURL` (string). With `DependencySpec`:
```go
m.Dependencies[alias] = manifest.DependencySpec{
    URL:    depURL,
    Select: selectedPaths, // nil if all selected
}
```

### `craft get` Flow (`internal/cli/get.go`)

Current flow (lines 43–307):
1. Parse args: detect alias vs URL pattern
2. Parse each URL via `ParseDepURL()` (lines 71, 79)
3. Load global manifest
4. Check for conflicts/existing deps
5. Add all deps to manifest
6. Resolve and install globally (flat keys)

**Subpath support injection (lines 71–82):**

Currently `ParseDepURL(args[1])` and `ParseDepURL(arg)`. With fragment support, `ParseDepURL` will extract `Subpath`. The flow needs:

```go
parsed, err := resolve.ParseDepURL(arg)
if err != nil { ... }
// If subpath present, create structured dep
if parsed.Subpath != "" {
    // Add as DependencySpec with Select: [parsed.Subpath]
    m.Dependencies[alias] = manifest.DependencySpec{
        URL:    parsed.PackageIdentity() + "@" + parsed.RefString(), // URL without fragment
        Select: []string{parsed.Subpath},
    }
} else {
    m.Dependencies[alias] = manifest.DependencySpec{URL: arg}
}
```

### `craft install` Flow (`internal/cli/install.go`)

`runInstallProject()` (lines 150–253) and `runInstallGlobal()` (lines 50–148):
1. Parse manifest
2. Load existing pinfile
3. Resolve dependencies
4. Write pinfile
5. Collect skill files
6. Install

**Minimal changes needed.** The install command reads `m.Dependencies` and passes the manifest to the resolver. If `Dependencies` is now `map[string]DependencySpec`, the resolver handles filtering internally. The install command doesn't need to know about `select` — it just installs whatever the resolver returns.

The `collectSkillFiles()` helper (lines 365–411) uses `dep.Skills` and `dep.SkillPaths` from `ResolvedDep`, which are already filtered.

---

## Fetcher Analysis

### `ListTree()` (`internal/fetch/gogit.go`, lines 116–159)

Returns all file paths in the repo tree at a given commit. Used for:
- Auto-discovery of skills (scanning for `SKILL.md`)
- Collecting skill directory files for integrity
- **New:** Previewing available skills in `craft add`

Has an in-memory cache (`treeCache`, line 32) keyed by `url\x00commitSHA`. This means repeated `ListTree` calls for the same commit are free after the first call.

### `ReadFiles()` (`internal/fetch/gogit.go`, lines 162–204)

Reads specific files by path at a commit. Missing files are silently skipped. Size limit: 10MB per file.

### For Interactive Preview

To show available skills during `craft add`, the flow would be:
1. `fetcher.ResolveRef(cloneURL, parsed.GitRef())` → commitSHA
2. `fetcher.ListTree(cloneURL, commitSHA)` → all paths
3. Scan for `SKILL.md` files in paths
4. `fetcher.ReadFiles(cloneURL, commitSHA, mdPaths)` → read SKILL.md frontmatter
5. Parse frontmatter to get skill names
6. Present to user

This is essentially what `discoverSkillsForDep()` does (resolver.go lines 382–399), but we need it before resolution (for the preview). Options:
- Extract discovery logic into a reusable function callable from CLI code
- Call `resolver.discoverSkillsForDep()` directly (it's currently unexported)
- Use the standalone `DiscoverSkills()` from `discover.go` (already exported)

**Recommended:** Use `DiscoverSkills()` from `discover.go`. It takes `allPaths` and a `readFile` function — the CLI can construct these from the fetcher:

```go
allPaths, _ := fetcher.ListTree(cloneURL, commitSHA)
readFile := func(path string) ([]byte, error) {
    files, err := fetcher.ReadFiles(cloneURL, commitSHA, []string{path})
    if err != nil { return nil, err }
    if content, ok := files[path]; ok { return content, nil }
    return nil, fmt.Errorf("not found: %s", path)
}
skills, _ := resolve.DiscoverSkills(allPaths, readFile)
```

### Caching Implications

The `ListTree` cache is per-fetcher-instance. In `craft add`, a new fetcher is created (line 102). The preview call and the subsequent resolve call will share the same fetcher instance, so the tree cache will be warm for the resolve step. No additional caching needed.

---

## Testing Patterns

### Conventions Observed

1. **Table-driven tests** are the dominant pattern:
   - `depurl_test.go`: `tests := []struct{ name, input string; want *DepURL; wantErr bool }` (lines 7–232)
   - `validate_test.go`: Both inline and table-driven for name validation (lines 48–80)
   - `manifest/validate_test.go`: Mix of individual test functions and table-driven for URL validation (lines 112–150)

2. **MockFetcher** (`internal/fetch/mock.go`): Map-based mock with `Refs`, `Trees`, `Files`, `Errors` maps. Key format: `"url:ref"`, `"url:commit:path"`.

3. **Helper functions**: `setupDep()` in `resolver_test.go` (lines 18–24) creates a standard dep setup. `assertContains()` in `validate_test.go` (lines 242–247).

4. **No external test frameworks** — pure `testing` package with `t.Run()` subtests.

5. **Error testing**: Check `wantErr bool`, then verify error message contents with `strings.Contains()`.

6. **Round-trip testing**: `write_test.go` has `TestWriteRoundTrip` (lines 10–61) — serialize then parse, compare fields.

7. **Determinism testing**: `TestWriteMapKeyOrder` (lines 122–167) writes 5 times, verifies identical output.

### Test Strategy Recommendations

**DepURL tests:**
- Add table entries for `#subpath` variants (with/without, various ref types)
- Test `String()` reconstruction includes subpath
- Test `PackageIdentity()` excludes subpath

**Manifest tests:**
- Round-trip tests for both string and structured dep formats
- Custom YAML unmarshal tests: scalar → `DependencySpec{URL: "..."}`, mapping → full struct
- Write tests: structured deps serialize correctly, simple deps stay as strings
- Validation tests: invalid `select` paths (leading `/`, `..`, absolute)

**Resolver tests:**
- New `TestResolveWithSelect`: mock dep with 3 skills, select 1, verify only 1 in result
- `TestResolveSelectMerge`: two deps same package, different selects, verify union
- `TestResolveSelectNotFound`: select path doesn't match any skill → error
- `TestResolveEmptySelect`: empty select list → all skills (same as omitting)

**Pinfile tests:**
- Round-trip with selected subset of skills
- Verify integrity changes when selection changes

**CLI tests (if integration-style):**
- `craft add` with `--all` flag
- `craft get` with `#subpath` URL

---

## PURL Compatibility

### PURL Subpath Syntax (ECMA-427, Section 5.6.7)

The PURL specification defines subpath as:
```
pkg:type/namespace/name@version?qualifiers#subpath
```

Key rules:
1. **Fragment separator**: `#` — same as URL fragment identifier per RFC 3986
2. **Relative path**: Subpath is relative to the package root, no leading `/`
3. **Segments separated by `/`**: e.g., `#dir/subdir/file.txt`
4. **Percent-encoding**: Special characters must be percent-encoded per RFC 3986
5. **No trailing `/`** in practice

### Our Syntax Alignment

Our proposed syntax: `github.com/acme/skills@v1.0.0#skills/docx`

| PURL Rule | Our Implementation | Status |
|-----------|-------------------|--------|
| `#` fragment separator | ✅ Using `#` | Aligned |
| Relative path (no leading `/`) | ✅ `skills/docx` not `/skills/docx` | Aligned |
| `/`-separated segments | ✅ Standard path segments | Aligned |
| Percent-encoding | ⚠️ Not implemented yet | Low risk — skill paths are simple alphanumeric |

**Difference from PURL:** Our URL format is not a full PURL (`pkg:type/...`). The `#subpath` syntax is borrowed from PURL but applied to our DepURL format (`host/org/repo@ref#subpath`). This is PURL-*compatible* syntax, not PURL itself.

**Future PURL migration path:** If craft adopts full PURL (`pkg:agentskill/...`), the `#subpath` fragment semantics remain identical. The parser would change the prefix format but the subpath handling would transfer directly.

---

## Risks and Concerns

### 1. **`map[string]string` → `map[string]DependencySpec` is a Breaking Type Change** (Severity: High)

Every file that reads `m.Dependencies` must be updated. A search for `m.Dependencies` usage across the codebase is critical:
- `resolver.go` lines 56, 205, 282 — iterate deps
- `add.go` lines 85, 95, 98 — check/set deps
- `get.go` lines 109, 113, 195 — check/set deps
- `install.go` lines 70, 171 — check emptiness
- `validate.go` lines 55–59 — validate URLs
- `write.go` line 31 — serialize deps
- `update.go`, `remove.go`, `list.go`, `tree.go` — various dep operations

This is a wide-impact change, but it's straightforward since `DependencySpec.URL` replaces the string value in all read paths.

### 2. **Custom YAML Unmarshaler Complexity** (Severity: Medium)

The `UnmarshalYAML(*yaml.Node)` pattern is well-supported by `gopkg.in/yaml.v3` but adds complexity. Risks:
- Edge case: what if a YAML value is neither scalar nor mapping? → Return clear error
- Edge case: `select: []` (explicit empty list) vs. omitted `select` → Both should mean "all skills"
- YAML anchors/aliases interacting with custom unmarshaler → Should work but needs testing

### 3. **Select Path Matching Ambiguity** (Severity: Medium)

How to match `select: ["skills/docx"]` against discovered skills:
- Discovered skill at path `skills/docx` → exact match ✅
- Discovered skill at path `./skills/docx` → need normalization (strip `./` prefix)
- Discovered skill at path `skills/docx/` → need normalization (strip trailing `/`)

The resolver already normalizes with `strings.TrimPrefix(sp, "./")` (resolver.go line 405). Apply the same normalization to `select` paths.

### 4. **Integrity Recalculation on Selection Change** (Severity: Low)

If a user changes `select` (adds/removes a skill), the integrity digest changes. This means `craft install` will see a mismatch with the pinfile and require re-resolution. This is correct behavior but should be documented.

### 5. **Transitive Dependency Select Propagation** (Severity: Low)

Per WorkShaping, `select` only applies to direct dependencies. Transitive deps (from a dep's own `craft.yaml`) are always resolved in full. This is the correct design — a consumer shouldn't restrict what a package's own transitive dependencies expose.

However, if Package A declares `select: [skills/x]` for Package B, and Package B's `craft.yaml` has transitive deps, those transitive deps still resolve fully. The `select` only filters B's skills, not B's dependencies.

### 6. **`craft get` URL Parsing Order** (Severity: Low)

When parsing `github.com/acme/skills@v1.0.0#skills/docx`:
- The `#` must be split before `@` processing
- But what about branch refs like `branch:feature/path#subpath`? The `/` in the branch name and the `/` in the subpath are unambiguous because `#` always separates them

Potential issue: `@branch:name#with-hash` — the `#` is the fragment separator. Branch names with `#` would break. However, git branch names rarely contain `#`, and we can document this limitation.

### 7. **Interactive UI in Non-TTY Environments** (Severity: Low)

`craft add` in CI/CD pipelines (non-TTY) should either:
- Default to installing all skills (current behavior)
- Require `--all` flag explicitly
- Auto-detect non-TTY and skip interactive prompt

The `get.go` already has TTY detection (`term.IsTerminal()`, line 118) that can be reused.

### 8. **Schema Version Bump** (Severity: Decision needed)

The structured dependency format is a schema extension. Should `schema_version` bump from 1 to 2? Arguments:
- **No bump:** The string format is still valid; older clients reading a manifest with only string deps work fine. Structured deps in a v1 manifest would fail gracefully on old clients (YAML parse would put an empty string or error).
- **Bump to 2:** Makes it explicit that the manifest uses new features. Old clients can give a clear "unsupported schema version" error.

Recommendation: **Do NOT bump schema version.** The string format is backward-compatible, and older clients encountering structured deps will get a YAML parse error (which is informative enough). A bump would break all existing tooling unnecessarily.
