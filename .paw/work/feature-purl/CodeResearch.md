# Code Research: Subpath Skill Selection

Deep implementation-focused analysis of the craft codebase for the subpath skill selection feature. This document specifies exact code changes, function signatures, type definitions, and a phasing strategy.

---

## 1. DepURL Parser Changes

### Current ParseDepURL (depurl.go:72–114)

```go
func ParseDepURL(raw string) (*DepURL, error) {
    atIdx := strings.Index(raw, "@")
    // ... splits on @, classifies ref
}
```

Current DepURL struct (depurl.go:40–63) has 7 fields: `Raw`, `Host`, `Org`, `Repo`, `Version`, `Ref`, `RefType`.

### Exact Modified DepURL Struct

```go
type DepURL struct {
    Raw     string
    Host    string
    Org     string
    Repo    string
    Version string
    Ref     string
    RefType RefType
    Subpath string // NEW: optional subpath fragment (e.g., "skills/docx")
}
```

### Exact Modified ParseDepURL

The `#` fragment must be split **before** the `@` split because the URL format is `host/org/repo@ref#subpath`. The `#` appears after the ref, so splitting on `#` first from the raw string cleanly separates the subpath from the rest.

```go
func ParseDepURL(raw string) (*DepURL, error) {
    // NEW: Extract optional #subpath fragment before any other parsing.
    var subpath string
    if hashIdx := strings.Index(raw, "#"); hashIdx >= 0 {
        subpath = raw[hashIdx+1:]
        raw = raw[:hashIdx]
    }

    atIdx := strings.Index(raw, "@")
    if atIdx < 0 {
        return nil, fmt.Errorf("invalid dependency URL %q: missing '@' ...", raw)
    }

    identity := raw[:atIdx]
    ref := raw[atIdx+1:]
    // ... existing ref classification logic unchanged ...

    d := &DepURL{
        Raw:     raw,  // NOTE: Raw stores the fragment-stripped URL
        Host:    matches[1],
        Org:     matches[2],
        Repo:    matches[3],
        Subpath: normalizeSubpath(subpath), // NEW
    }
    // ... existing ref type logic unchanged ...
    return d, nil
}
```

### Subpath Normalization Helper

```go
// normalizeSubpath cleans a subpath fragment: strips leading "./", trailing "/",
// and returns empty string for empty/whitespace-only input.
func normalizeSubpath(s string) string {
    s = strings.TrimSpace(s)
    s = strings.TrimPrefix(s, "./")
    s = strings.TrimSuffix(s, "/")
    return s
}
```

### String() Method Change (depurl.go:173–178)

Current `String()` returns `d.Raw` if set, else reconstructs. With the change, `Raw` stores the fragment-stripped URL. The reconstructed form must append `#subpath` if present:

```go
func (d *DepURL) String() string {
    base := d.Raw
    if base == "" {
        base = d.PackageIdentity() + "@" + d.RefString()
    }
    if d.Subpath != "" {
        return base + "#" + d.Subpath
    }
    return base
}
```

### WithVersion() Behavior (depurl.go:167–170)

`WithVersion()` returns a bare URL for version comparison. Subpath should be **dropped** — this method is used for manifest URL updates (update.go:161) where the subpath is tracked in `DependencySpec.Select`, not in the URL itself.

Current implementation is already correct — no change needed:
```go
func (d *DepURL) WithVersion(version string) string {
    version = strings.TrimPrefix(version, "v")
    return d.PackageIdentity() + "@v" + version
}
```

### Edge Case: `github.com/org/repo@branch:feat/thing#sub/path`

The `#` character is unambiguous here. The parsing order is:
1. Split on first `#` → `github.com/org/repo@branch:feat/thing` + `sub/path`
2. Split on first `@` → `github.com/org/repo` + `branch:feat/thing`
3. `branch:feat/thing` is correctly parsed as a branch ref (slashes are valid in branch names)

The only ambiguity would be a branch name **containing** `#`, which the spec explicitly declares as unsupported (Spec.md edge cases, line 85: "Branch name containing `#` character: unsupported, documented limitation").

### Test Changes (depurl_test.go)

New test cases to add to the table-driven `TestParseDepURL`:
```go
{
    name:  "tag with subpath",
    input: "github.com/acme/skills@v1.0.0#skills/docx",
    want: &DepURL{
        Raw:     "github.com/acme/skills@v1.0.0",
        Host:    "github.com",
        Org:     "acme",
        Repo:    "skills",
        Version: "1.0.0",
        RefType: RefTypeTag,
        Subpath: "skills/docx",
    },
},
{
    name:  "commit with subpath",
    input: "github.com/acme/tools@abc1234#tools/x",
    want: &DepURL{
        Raw:     "github.com/acme/tools@abc1234",
        Host:    "github.com",
        Org:     "acme",
        Repo:    "tools",
        Ref:     "abc1234",
        RefType: RefTypeCommit,
        Subpath: "tools/x",
    },
},
{
    name:  "branch with subpath",
    input: "github.com/acme/tools@branch:main#tools/x",
    want: &DepURL{
        Raw:     "github.com/acme/tools@branch:main",
        Host:    "github.com",
        Org:     "acme",
        Repo:    "tools",
        Ref:     "main",
        RefType: RefTypeBranch,
        Subpath: "tools/x",
    },
},
{
    name:  "nested subpath",
    input: "github.com/acme/skills@v1.0.0#skills/nested/docx",
    want: &DepURL{
        Raw:     "github.com/acme/skills@v1.0.0",
        Host:    "github.com",
        Org:     "acme",
        Repo:    "skills",
        Version: "1.0.0",
        RefType: RefTypeTag,
        Subpath: "skills/nested/docx",
    },
},
{
    name:  "empty fragment treated as no subpath",
    input: "github.com/acme/skills@v1.0.0#",
    want: &DepURL{
        Raw:     "github.com/acme/skills@v1.0.0",
        Host:    "github.com",
        Org:     "acme",
        Repo:    "skills",
        Version: "1.0.0",
        RefType: RefTypeTag,
        Subpath: "",
    },
},
{
    name:  "subpath with leading dot-slash normalized",
    input: "github.com/acme/skills@v1.0.0#./skills/docx",
    want: &DepURL{
        Raw:     "github.com/acme/skills@v1.0.0",
        Host:    "github.com",
        Org:     "acme",
        Repo:    "skills",
        Version: "1.0.0",
        RefType: RefTypeTag,
        Subpath: "skills/docx",
    },
},
{
    name:  "branch with slashes and subpath",
    input: "github.com/org/repo@branch:feat/thing#sub/path",
    want: &DepURL{
        Raw:     "github.com/org/repo@branch:feat/thing",
        Host:    "github.com",
        Org:     "org",
        Repo:    "repo",
        Ref:     "feat/thing",
        RefType: RefTypeBranch,
        Subpath: "sub/path",
    },
},
```

New `TestDepURLMethods` assertions:
```go
// String() with subpath
d, _ := ParseDepURL("github.com/acme/skills@v1.0.0#skills/docx")
if got := d.String(); got != "github.com/acme/skills@v1.0.0#skills/docx" {
    t.Errorf("String() with subpath = %q", got)
}

// WithVersion drops subpath
if got := d.WithVersion("v2.0.0"); got != "github.com/acme/skills@v2.0.0" {
    t.Errorf("WithVersion should not include subpath, got %q", got)
}

// PackageIdentity excludes subpath
if got := d.PackageIdentity(); got != "github.com/acme/skills" {
    t.Errorf("PackageIdentity should exclude subpath, got %q", got)
}
```

---

## 2. DependencySpec Implementation

### Exact DependencySpec Struct (types.go)

Add to `internal/manifest/types.go`:

```go
// DependencySpec represents a dependency in the manifest.
// It can be serialized as either a simple URL string or a structured
// object with url and select fields.
type DependencySpec struct {
    // URL is the dependency URL (always present).
    URL string `yaml:"url"`

    // Select lists subpath filters (optional). Empty means all skills.
    Select []string `yaml:"select,omitempty"`
}
```

### Changed Manifest Struct (types.go:6–27)

```go
type Manifest struct {
    SchemaVersion int                       `yaml:"schema_version"`
    Name          string                    `yaml:"name"`
    Description   string                    `yaml:"description,omitempty"`
    License       string                    `yaml:"license,omitempty"`
    Skills        []string                  `yaml:"skills"`
    Dependencies  map[string]DependencySpec `yaml:"dependencies,omitempty"`  // CHANGED
    Metadata      map[string]string         `yaml:"metadata,omitempty"`
}
```

### Exact UnmarshalYAML Method

```go
func (d *DependencySpec) UnmarshalYAML(value *yaml.Node) error {
    switch value.Kind {
    case yaml.ScalarNode:
        // Simple string form: "github.com/org/repo@v1.0.0"
        d.URL = value.Value
        return nil
    case yaml.MappingNode:
        // Structured form: {url: "...", select: [...]}
        type raw struct {
            URL    string   `yaml:"url"`
            Select []string `yaml:"select"`
        }
        var r raw
        if err := value.Decode(&r); err != nil {
            return err
        }
        if r.URL == "" {
            return fmt.Errorf("structured dependency must have a 'url' field")
        }
        d.URL = r.URL
        d.Select = r.Select
        return nil
    default:
        return fmt.Errorf("dependency must be a string or mapping, got %v", value.Tag)
    }
}
```

This requires adding `"fmt"` and `"gopkg.in/yaml.v3"` imports to `types.go`.

### Exact MarshalYAML Method

Not needed as a method on DependencySpec. Instead, the `Write()` function in `write.go` needs a new `addDependencies` helper (replacing the `addStringMap` call on line 32).

### write.go Changes

**Line 31–33:** Replace `addStringMap` call with new `addDependencies` call:
```go
// BEFORE:
if len(m.Dependencies) > 0 {
    addStringMap(mapping, "dependencies", m.Dependencies)
}

// AFTER:
if len(m.Dependencies) > 0 {
    addDependencies(mapping, "dependencies", m.Dependencies)
}
```

**New function `addDependencies`:**

```go
// addDependencies serializes the dependency map with format fidelity:
// simple deps (no Select) are written as scalar strings; structured deps
// are written as mapping nodes with url + select fields.
func addDependencies(mapping *yaml.Node, key string, deps map[string]DependencySpec) {
    keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
    mapNode := &yaml.Node{Kind: yaml.MappingNode}

    keys := make([]string, 0, len(deps))
    for k := range deps {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    for _, alias := range keys {
        dep := deps[alias]
        aliasNode := &yaml.Node{Kind: yaml.ScalarNode, Value: alias}

        if len(dep.Select) == 0 {
            // Simple string form
            valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: dep.URL}
            mapNode.Content = append(mapNode.Content, aliasNode, valNode)
        } else {
            // Structured form: {url: "...", select: [...]}
            objNode := &yaml.Node{Kind: yaml.MappingNode}

            // url field
            objNode.Content = append(objNode.Content,
                &yaml.Node{Kind: yaml.ScalarNode, Value: "url"},
                &yaml.Node{Kind: yaml.ScalarNode, Value: dep.URL},
            )

            // select field
            selKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "select"}
            selSeq := &yaml.Node{Kind: yaml.SequenceNode}
            for _, s := range dep.Select {
                selSeq.Content = append(selSeq.Content,
                    &yaml.Node{Kind: yaml.ScalarNode, Value: s},
                )
            }
            objNode.Content = append(objNode.Content, selKey, selSeq)

            mapNode.Content = append(mapNode.Content, aliasNode, objNode)
        }
    }

    mapping.Content = append(mapping.Content, keyNode, mapNode)
}
```

### validate.go Changes (lines 54–59)

```go
// BEFORE:
for alias, url := range m.Dependencies {
    if !depURLPattern.MatchString(url) {
        errs = append(errs, fmt.Errorf("dependencies[%q]: %q does not match ...", alias, url))
    }
}

// AFTER:
for alias, dep := range m.Dependencies {
    if !depURLPattern.MatchString(dep.URL) {
        errs = append(errs, fmt.Errorf("dependencies[%q]: %q does not match ...", alias, dep.URL))
    }
    for _, sel := range dep.Select {
        if strings.HasPrefix(sel, "/") {
            errs = append(errs, fmt.Errorf("dependencies[%q].select: %q must be a relative path (no leading '/')", alias, sel))
        }
        if strings.Contains(sel, "..") {
            errs = append(errs, fmt.Errorf("dependencies[%q].select: %q must not contain '..' (path traversal)", alias, sel))
        }
    }
}
```

This requires adding `"strings"` to validate.go imports (currently only `"fmt"` and `"regexp"`).

---

## 3. Dependencies Consumer Audit

Every file:line that references `m.Dependencies` or uses the dependency value as a string. Each must be updated for the `map[string]string` → `map[string]DependencySpec` change.

### internal/manifest/types.go:23
```go
// BEFORE: Dependencies map[string]string `yaml:"dependencies,omitempty"`
// AFTER:  Dependencies map[string]DependencySpec `yaml:"dependencies,omitempty"`
```

### internal/manifest/validate.go:55–58
```go
// BEFORE: for alias, url := range m.Dependencies { ... url ... }
// AFTER:  for alias, dep := range m.Dependencies { ... dep.URL ... }
```
Plus new select path validation loop (see §2 above).

### internal/manifest/write.go:31–33
```go
// BEFORE: addStringMap(mapping, "dependencies", m.Dependencies)
// AFTER:  addDependencies(mapping, "dependencies", m.Dependencies)
```

### internal/manifest/write_test.go:17–19
```go
// BEFORE: Dependencies: map[string]string{"dep-a": "github.com/org/repo-a@v1.0.0"},
// AFTER:  Dependencies: map[string]DependencySpec{"dep-a": {URL: "github.com/org/repo-a@v1.0.0"}},
```
Also line 55–56: `parsed.Dependencies["dep-a"]` → `parsed.Dependencies["dep-a"].URL`

### internal/manifest/write_test.go:127–131 (TestWriteMapKeyOrder)
```go
// BEFORE:
Dependencies: map[string]string{
    "charlie": "github.com/org/charlie@v1.0.0",
    "alpha":   "github.com/org/alpha@v1.0.0",
    "bravo":   "github.com/org/bravo@v1.0.0",
},
// AFTER:
Dependencies: map[string]DependencySpec{
    "charlie": {URL: "github.com/org/charlie@v1.0.0"},
    "alpha":   {URL: "github.com/org/alpha@v1.0.0"},
    "bravo":   {URL: "github.com/org/bravo@v1.0.0"},
},
```

### internal/manifest/parse_test.go:44–48
```go
// BEFORE:
if m.Dependencies["git-ops"] != "github.com/example/git@v1.0.0" {
    t.Errorf("Dependencies[git-ops] = %q", m.Dependencies["git-ops"])
}
// AFTER:
if m.Dependencies["git-ops"].URL != "github.com/example/git@v1.0.0" {
    t.Errorf("Dependencies[git-ops].URL = %q", m.Dependencies["git-ops"].URL)
}
```

### internal/manifest/parse_test.go:72–73
```go
// len(m.Dependencies) — no change needed (len works on any map)
```

### internal/manifest/validate_test.go:25–27
```go
// BEFORE: Dependencies: map[string]string{"git-ops": "github.com/example/git@v1.0.0"},
// AFTER:  Dependencies: map[string]DependencySpec{"git-ops": {URL: "github.com/example/git@v1.0.0"}},
```
Same pattern for lines 133, 207, 221, 234.

### internal/resolve/resolver.go:56
```go
// len(m.Dependencies) == 0 — no change needed (len works on any map)
```

### internal/resolve/resolver.go:205
```go
// len(depManifest.Dependencies) > 0 — no change needed
```

### internal/resolve/resolver.go:282–296
```go
// BEFORE:
for alias, depURL := range m.Dependencies {
    parsed, err := ParseDepURL(depURL)
    ...
    dep := ResolvedDep{
        URL:     depURL,
        Alias:   alias,
        Source:  source,
        RefType: parsed.RefType,
    }
// AFTER:
for alias, depSpec := range m.Dependencies {
    parsed, err := ParseDepURL(depSpec.URL)
    ...
    dep := ResolvedDep{
        URL:     depSpec.URL,
        Alias:   alias,
        Source:  source,
        RefType: parsed.RefType,
        Select:  depSpec.Select,  // NEW field on ResolvedDep
    }
```

### internal/resolve/resolver.go:327
```go
// len(depManifest.Dependencies) > 0 — no change needed
```

### internal/cli/add.go:85–88
```go
// BEFORE:
if existing, ok := m.Dependencies[alias]; ok {
    isUpdate = true
    if existing == depURL { ... }
    cmd.Printf("Updating %q: %s → %s\n", alias, existing, depURL)
}
// AFTER:
if existing, ok := m.Dependencies[alias]; ok {
    isUpdate = true
    if existing.URL == depURL { ... }
    cmd.Printf("Updating %q: %s → %s\n", alias, existing.URL, depURL)
}
```

### internal/cli/add.go:95–98
```go
// BEFORE:
if m.Dependencies == nil {
    m.Dependencies = make(map[string]string)
}
m.Dependencies[alias] = depURL
// AFTER:
if m.Dependencies == nil {
    m.Dependencies = make(map[string]manifest.DependencySpec)
}
m.Dependencies[alias] = manifest.DependencySpec{URL: depURL}
```

### internal/cli/get.go:109
```go
// BEFORE: Dependencies: make(map[string]string),
// AFTER:  Dependencies: make(map[string]manifest.DependencySpec),
```

### internal/cli/get.go:113–114
```go
// BEFORE: m.Dependencies = make(map[string]string)
// AFTER:  m.Dependencies = make(map[string]manifest.DependencySpec)
```

### internal/cli/get.go:120–124
```go
// BEFORE:
existing, ok := m.Dependencies[dep.alias]
if !ok { continue }
if existing == dep.url { ... }
// AFTER:
existingSpec, ok := m.Dependencies[dep.alias]
if !ok { continue }
if existingSpec.URL == dep.url { ... }
```

And line 134: `existingParsed, existingErr := resolve.ParseDepURL(existing)` → `resolve.ParseDepURL(existingSpec.URL)`
And line 148: `m.Dependencies[newAlias]` — just checking existence, no change needed.

### internal/cli/get.go:196
```go
// BEFORE: m.Dependencies[dep.alias] = dep.url
// AFTER:  m.Dependencies[dep.alias] = manifest.DependencySpec{URL: dep.url}
```

### internal/cli/update.go:76
```go
// len(m.Dependencies) == 0 — no change needed
```

### internal/cli/update.go:91–93
```go
// BEFORE:
if _, ok := m.Dependencies[targetAlias]; !ok {
    return fmt.Errorf("... %s", targetAlias, availableAliases(m.Dependencies))
// AFTER: Need to change availableAliases signature (see below)
```

### internal/cli/update.go:106
```go
// BEFORE: for alias, depURL := range m.Dependencies {
// AFTER:  for alias, depSpec := range m.Dependencies {
//         depURL := depSpec.URL
```
Lines 111, 143, 144, 155, 161 use `depURL` — all will work via the local `depURL := depSpec.URL`.

### internal/cli/update.go:161
```go
// BEFORE: m.Dependencies[alias] = newURL
// AFTER:  m.Dependencies[alias] = manifest.DependencySpec{URL: newURL, Select: depSpec.Select}
```
**Critical**: Must preserve the existing `Select` when updating the URL.

### internal/cli/update.go:180
```go
// BEFORE: for alias, depURL := range m.Dependencies {
// AFTER:  for alias, depSpec := range m.Dependencies {
//         depURL := depSpec.URL
```

### internal/cli/remove.go:69–71
```go
// BEFORE:
depURL, ok := m.Dependencies[alias]
if !ok {
    available := availableAliases(m.Dependencies)
// AFTER:
depSpec, ok := m.Dependencies[alias]
if !ok {
    available := availableAliases(m.Dependencies)
// ... use depSpec.URL where depURL was used (lines 81, 94, 130)
```

### internal/cli/remove.go:87
```go
// delete(m.Dependencies, alias) — no change needed (works on any map)
```

### internal/cli/remove.go:207
```go
// BEFORE: func availableAliases(deps map[string]string) string {
// AFTER:  func availableAliases(deps map[string]manifest.DependencySpec) string {
```

### internal/cli/list.go:44
```go
// BEFORE: for alias, depURL := range m.Dependencies {
// AFTER:  for alias, depSpec := range m.Dependencies {
//         depURL := depSpec.URL (used on line 45)
```
Actually line 45 does `parsed, err := resolve.ParseDepURL(depURL)` which becomes `resolve.ParseDepURL(depSpec.URL)`.

### internal/cli/tree.go:39
```go
// BEFORE: for alias, depURL := range m.Dependencies {
// AFTER:  for alias, depSpec := range m.Dependencies {
//         depURL := depSpec.URL
```

### internal/cli/outdated.go:29, 34, 64–65, 71
```go
// len(m.Dependencies) — no change
// for alias := range m.Dependencies — no change (iterating keys only)
// Line 71: depURL := m.Dependencies[alias] → depSpec := m.Dependencies[alias]; depURL := depSpec.URL
```

### internal/cli/install.go:70, 171
```go
// len(m.Dependencies) == 0 — no change needed
```

### internal/validate/runner.go:258
```go
// len(m.Dependencies) == 0 — no change needed
```

### internal/validate/runner.go:267–268
```go
// BEFORE:
for _, url := range m.Dependencies {
    depURLs[url] = true
    if _, ok := p.Resolved[url]; !ok {
// AFTER:
for _, dep := range m.Dependencies {
    depURLs[dep.URL] = true
    if _, ok := p.Resolved[dep.URL]; !ok {
```

### internal/validate/runner.go:318
```go
// BEFORE: for alias, url := range m.Dependencies {
// AFTER:  for alias, dep := range m.Dependencies {
//         url := dep.URL
```

### Test files with map[string]string literal for Dependencies

Files that construct `Manifest` literals with `Dependencies: map[string]string{...}`:
- `internal/manifest/write_test.go:17` → `map[string]DependencySpec`
- `internal/manifest/write_test.go:127` → `map[string]DependencySpec`
- `internal/manifest/validate_test.go:25,133,207,221,234` → `map[string]DependencySpec`
- `internal/cli/remove_test.go:246` → `map[string]DependencySpec` (via grep)
- `internal/cli/update_test.go:106` → `map[string]DependencySpec` (via grep)
- Any other test constructing `manifest.Manifest` with Dependencies

**Total: ~45 individual line-level changes across 14 files.**

---

## 4. Resolver Filter Implementation

### ResolvedDep Struct Change (types.go)

```go
type ResolvedDep struct {
    URL        string
    Alias      string
    Commit     string
    Integrity  string
    Skills     []string
    SkillPaths []string
    Source     string
    RefType    RefType
    Select     []string // NEW: subpath filter from manifest (empty = all)
}
```

### Exact `filterBySelect` Function

New function in `internal/resolve/resolver.go`:

```go
// filterBySelect filters discovered skills to only those matching the select
// paths. Returns an error if any select path has no matching skill.
func filterBySelect(names, dirs []string, files map[string][]byte, selectPaths []string) ([]string, []string, map[string][]byte, error) {
    if len(selectPaths) == 0 {
        return names, dirs, files, nil
    }

    // Normalize select paths for comparison
    normalized := make([]string, len(selectPaths))
    for i, s := range selectPaths {
        s = strings.TrimPrefix(s, "./")
        s = strings.TrimSuffix(s, "/")
        normalized[i] = s
    }

    // Build lookup from dir → index for matching
    dirIndex := make(map[string]int, len(dirs))
    for i, d := range dirs {
        dirIndex[d] = i
    }

    var filteredNames []string
    var filteredDirs []string
    filteredFiles := make(map[string][]byte)

    for _, sel := range normalized {
        idx, ok := dirIndex[sel]
        if !ok {
            return nil, nil, nil, fmt.Errorf("selected path %q does not match any skill in the package (available: %s)",
                sel, strings.Join(dirs, ", "))
        }
        filteredNames = append(filteredNames, names[idx])
        filteredDirs = append(filteredDirs, dirs[idx])

        // Include files under this skill directory
        prefix := sel + "/"
        if sel == "" {
            prefix = ""
        }
        for path, content := range files {
            if prefix == "" || strings.HasPrefix(path, prefix) {
                filteredFiles[path] = content
            }
        }
    }

    return filteredNames, filteredDirs, filteredFiles, nil
}
```

### Where in `resolveOne()` the Filter is Called (resolver.go:367–377)

```go
// BEFORE (lines 367–377):
skillNames, skillPaths, skillFiles, err := r.discoverSkillsForDep(cloneURL, commitSHA)
if err != nil {
    return dep, err
}
dep.Skills = skillNames
dep.SkillPaths = skillPaths
dep.Integrity = integrity.Digest(skillFiles)

// AFTER:
skillNames, skillPaths, skillFiles, err := r.discoverSkillsForDep(cloneURL, commitSHA)
if err != nil {
    return dep, err
}

// Apply select filter if specified
if len(dep.Select) > 0 {
    skillNames, skillPaths, skillFiles, err = filterBySelect(skillNames, skillPaths, skillFiles, dep.Select)
    if err != nil {
        return dep, fmt.Errorf("filtering skills for %s: %w", dep.URL, err)
    }
}

dep.Skills = skillNames
dep.SkillPaths = skillPaths
dep.Integrity = integrity.Digest(skillFiles)
```

### How `collectDeps()` Passes Select Through (resolver.go:282–297)

Already covered in §3 above. The `depSpec.Select` is assigned to `dep.Select` when constructing the `ResolvedDep` in `collectDeps()`:

```go
dep := ResolvedDep{
    URL:     depSpec.URL,
    Alias:   alias,
    Source:  source,
    RefType: parsed.RefType,
    Select:  depSpec.Select,  // threads select through to resolveOne
}
```

For **transitive** dependencies (lines 321–333), the `collectDeps` recurse uses the _dependency's own manifest_ which won't have select — transitive deps are resolved in full per spec assumption.

### MVS Select-Merge Logic (resolver.go:80–166)

When MVS groups by `PackageIdentity()` and multiple manifest entries point to the same package with different selects, the selects must be **unioned**. If any entry has an empty select (= all skills), the merged result is "all skills".

Insert after MVS version selection (after line 123 for tags, line 135 for commits, line 147 for branches), before `selected[identity] = best`:

```go
// Merge Select lists from all entries referencing this package.
// Empty select in any entry → all skills (nil).
var mergedSelect []string
allSkills := false
for _, dep := range deps {
    if len(dep.Select) == 0 {
        allSkills = true
        break
    }
    mergedSelect = append(mergedSelect, dep.Select...)
}
if allSkills {
    best.Select = nil
} else {
    best.Select = deduplicateStrings(mergedSelect)
}
```

New helper:
```go
func deduplicateStrings(ss []string) []string {
    seen := make(map[string]bool, len(ss))
    var result []string
    for _, s := range ss {
        if !seen[s] {
            seen[s] = true
            result = append(result, s)
        }
    }
    sort.Strings(result)
    return result
}
```

This merge logic should be factored out and applied **once** after the switch statement (all three ref-type branches), not duplicated in each case.

---

## 5. CLI Changes Audit

### internal/cli/add.go — Per-Line Changes

| Line | Current | Change |
|------|---------|--------|
| 39 | `var alias, depURL string` | No change (local var name) |
| 48 | `parsed, err := resolve.ParseDepURL(depURL)` | No change |
| 85 | `if existing, ok := m.Dependencies[alias]; ok {` | Value is now `DependencySpec` |
| 87 | `if existing == depURL {` | `if existing.URL == depURL {` |
| 88 | `cmd.Printf("... %s — nothing ...", alias, depURL)` | No change |
| 91 | `cmd.Printf("... %s → %s\n", alias, existing, depURL)` | `existing.URL` |
| 95–96 | `m.Dependencies = make(map[string]string)` | `make(map[string]manifest.DependencySpec)` |
| 98 | `m.Dependencies[alias] = depURL` | `manifest.DependencySpec{URL: depURL}` |

**Interactive preview flow for `craft add` (FR-010):**

Insert between lines 98 and 100 (after adding dep to manifest, before resolving). The flow:

1. After `resolve.ParseDepURL(depURL)` succeeds
2. Create a temporary resolver to discover skills in the package
3. If TTY and skill count > 1 and no `--all` flag:
   - Present interactive multi-select
   - User toggles skills on/off
   - If user selects a subset → `m.Dependencies[alias] = manifest.DependencySpec{URL: depURL, Select: selectedPaths}`
   - If user selects all → `m.Dependencies[alias] = manifest.DependencySpec{URL: depURL}` (simple form)
4. Non-TTY or `--all` → install all (existing behavior)

New flag:
```go
var addAll bool
// In init():
addCmd.Flags().BoolVar(&addAll, "all", false, "Install all skills without interactive selection")
```

Injection point (after line 98, before line 100):
```go
// Interactive skill preview (only for TTY + multi-skill packages)
if !addAll && term.IsTerminal(int(os.Stdin.Fd())) {
    skills, err := previewSkills(depURL)
    if err == nil && len(skills) > 1 {
        selected, err := promptSkillSelection(cmd, skills)
        if err != nil {
            return err
        }
        if len(selected) < len(skills) {
            m.Dependencies[alias] = manifest.DependencySpec{URL: depURL, Select: selected}
        }
    }
}
```

### internal/cli/get.go — Per-Line Changes

| Line | Current | Change |
|------|---------|--------|
| 71 | `parsed, err := resolve.ParseDepURL(args[1])` | Parse subpath from `parsed.Subpath` |
| 75 | `deps = append(deps, depEntry{alias: args[0], url: args[1], parsed: parsed, ...})` | Add `select` field |
| 79–83 | `parsed, err := resolve.ParseDepURL(arg)` | Extract `parsed.Subpath` into select |
| 109 | `Dependencies: make(map[string]string)` | `make(map[string]manifest.DependencySpec)` |
| 113–114 | `m.Dependencies = make(map[string]string)` | `make(map[string]manifest.DependencySpec)` |
| 120–124 | `existing, ok := m.Dependencies[dep.alias]`; `existing == dep.url` | `existingSpec.URL == dep.url` |
| 134 | `resolve.ParseDepURL(existing)` | `resolve.ParseDepURL(existingSpec.URL)` |
| 196 | `m.Dependencies[dep.alias] = dep.url` | See below |

**`#subpath` parsing and DependencySpec construction for `craft get`:**

The `depEntry` struct (line 45) should gain a `selectPaths` field:
```go
type depEntry struct {
    alias         string
    url           string       // URL without fragment
    parsed        *resolve.DepURL
    explicitAlias bool
    selectPaths   []string     // NEW: from #subpath parsing
}
```

After `ParseDepURL` succeeds (line 71 for explicit alias, line 79 for URL-only), extract the subpath:
```go
parsed, err := resolve.ParseDepURL(arg)
if err != nil { ... }

// Extract subpath from parsed URL
var selectPaths []string
if parsed.Subpath != "" {
    selectPaths = []string{parsed.Subpath}
}
// Use fragment-stripped URL for manifest storage
cleanURL := parsed.PackageIdentity() + "@" + parsed.RefString()

deps = append(deps, depEntry{
    alias:       parsed.Repo,
    url:         cleanURL,     // without #fragment
    parsed:      parsed,
    selectPaths: selectPaths,
})
```

Line 196 changes:
```go
// BEFORE: m.Dependencies[dep.alias] = dep.url
// AFTER:
m.Dependencies[dep.alias] = manifest.DependencySpec{
    URL:    dep.url,
    Select: dep.selectPaths,
}
```

### internal/cli/update.go — Per-Line Changes

| Line | Current | Change |
|------|---------|--------|
| 91 | `if _, ok := m.Dependencies[targetAlias]; !ok {` | No change (existence check) |
| 93 | `availableAliases(m.Dependencies)` | Signature change (see §3) |
| 106 | `for alias, depURL := range m.Dependencies {` | `for alias, depSpec := range m.Dependencies { depURL := depSpec.URL` |
| 161 | `m.Dependencies[alias] = newURL` | `manifest.DependencySpec{URL: newURL, Select: depSpec.Select}` |
| 180 | `for alias, depURL := range m.Dependencies {` | `for alias, depSpec := range m.Dependencies { depURL := depSpec.URL` |

**New-skill-discovery logic (FR-014):**

After resolution succeeds (around line 195), for each selectively-installed dep, compare available skills vs selected skills:

```go
// After resolution, check for new skills in selectively installed deps
for alias, depSpec := range m.Dependencies {
    if len(depSpec.Select) == 0 {
        continue // all skills installed, no discovery needed
    }
    for _, dep := range result.Resolved {
        if dep.Alias != alias {
            continue
        }
        // dep.Skills contains only selected skills.
        // Need to discover ALL skills to compare.
        // This requires a second discovery pass or caching all skills.
        // See implementation notes below.
    }
}
```

**Implementation approach for new-skill discovery:** The resolver's `resolveOne()` can store the full skill list before filtering in a new `AllSkillPaths` field on `ResolvedDep`, or we can re-discover after resolution. The cleaner approach is to add `AllSkillPaths []string` and `AllSkills []string` to `ResolvedDep`, populated before filtering, and use the diff in the update command.

However, this adds complexity. **Recommended: defer to Phase 2** and implement as a separate pass in `runUpdate` that re-discovers skills for selectively installed deps.

### internal/cli/remove.go — Per-Line Changes

| Line | Current | Change |
|------|---------|--------|
| 69 | `depURL, ok := m.Dependencies[alias]` | `depSpec, ok := m.Dependencies[alias]` |
| 70–71 | Uses `depURL` | Use `depSpec.URL` as `depURL` |
| 81 | `pf.Resolved[depURL]` | `pf.Resolved[depSpec.URL]` |
| 94 | `cmd.Printf("... %q (%s)\n", alias, depURL)` | `depSpec.URL` |
| 130 | `resolve.ParseDepURL(depURL)` | `resolve.ParseDepURL(depSpec.URL)` |
| 207 | `func availableAliases(deps map[string]string)` | `func availableAliases(deps map[string]manifest.DependencySpec)` |

### internal/cli/list.go — Per-Line Changes

| Line | Current | Change |
|------|---------|--------|
| 44 | `for alias, depURL := range m.Dependencies {` | `for alias, depSpec := range m.Dependencies { depURL := depSpec.URL` |

### internal/cli/tree.go — Per-Line Changes

| Line | Current | Change |
|------|---------|--------|
| 39 | `for alias, depURL := range m.Dependencies {` | `for alias, depSpec := range m.Dependencies { depURL := depSpec.URL` |

### internal/cli/outdated.go — Per-Line Changes

| Line | Current | Change |
|------|---------|--------|
| 71 | `depURL := m.Dependencies[alias]` | `depSpec := m.Dependencies[alias]; depURL := depSpec.URL` |

### internal/cli/install.go — No Dep-Value Changes

Lines 70 and 171 only check `len(m.Dependencies)` — no value access. No changes needed.

### internal/cli/validate.go — No Direct Changes

Calls `manifest.ValidateGlobal(m)` which internally uses the updated validate loop. No changes needed.

---

## 6. Interactive UI Analysis

### Existing UI Primitives (internal/ui/)

**`progress.go`:** TTY-aware progress indicator with `Start`, `Update`, `Done`, `Fail`. Uses `\r\033[K` for line overwriting on TTY. Has `IsTTY()` method. Uses `golang.org/x/term` for detection.

**`tree.go`:** Box-drawing tree renderer. `DepNode` struct with `Alias`, `URL`, `Skills`. `RenderTree()` writes to `io.Writer`.

### Multi-Select Prompt: Does Not Exist — Must Be Built

No existing multi-select prompt exists in `internal/ui/`. The codebase has two interactive prompts:
1. **Agent choice prompt** (`install.go:299–336`): Numbered list with single selection via `bufio.Scanner`
2. **Update confirmation** (`get.go:162–168`): Simple y/N prompt via `bufio.Scanner`

Neither supports multi-select (toggle individual items on/off).

### TTY Detection Pattern

Already in use in two places:
- `get.go:118`: `isTTY := term.IsTerminal(int(os.Stdin.Fd()))`
- `ui/progress.go:25`: `term.IsTerminal(int(os.Stderr.Fd()))`

The `golang.org/x/term` package is already a dependency (verified in `go.mod` via imports).

### Recommended Multi-Select Implementation

**Simple numbered-list approach** (no arrow-key navigation needed for MVP):

```go
// internal/ui/select.go (new file)

// MultiSelect presents a numbered list of items and lets the user
// toggle selections with space-separated numbers, then confirm.
// Returns indices of selected items.
func MultiSelect(w io.Writer, r io.Reader, prompt string, items []string) ([]int, error) {
    // Print numbered list with [x] checkboxes
    selected := make([]bool, len(items))
    for i := range selected {
        selected[i] = true // all selected by default
    }

    fmt.Fprintf(w, "\n%s\n\n", prompt)
    for i, item := range items {
        fmt.Fprintf(w, "  %d) [x] %s\n", i+1, item)
    }
    fmt.Fprintf(w, "\nEnter numbers to toggle (space-separated), 'a' for all, 'n' for none, or Enter to confirm: ")

    scanner := bufio.NewScanner(r)
    // ... toggle loop ...
    // Returns selected indices
}
```

### How to Handle `--all` Flag

Pattern from `install.go`:
```go
var addAll bool
addCmd.Flags().BoolVar(&addAll, "all", false, "Install all skills without interactive selection")
```

In `runAdd`:
```go
if addAll || !term.IsTerminal(int(os.Stdin.Fd())) {
    // Skip interactive selection, install all
} else {
    // Present multi-select
}
```

---

## 7. Testing Strategy

### DepURL Parser Tests (depurl_test.go)

- Add 7 new table-driven test cases (see §1 above)
- Add `TestDepURLSubpathMethods` for `String()`, `WithVersion()`, `PackageIdentity()` with subpath
- Verify `Raw` field stores fragment-stripped URL

### Manifest Round-Trip Tests (write_test.go, parse_test.go)

**New test: `TestWriteRoundTripStructuredDep`**
```go
func TestWriteRoundTripStructuredDep(t *testing.T) {
    original := &Manifest{
        SchemaVersion: 1,
        Name:          "structured",
        Skills:        []string{"./skill"},
        Dependencies: map[string]DependencySpec{
            "simple": {URL: "github.com/org/a@v1.0.0"},
            "selective": {URL: "github.com/org/b@v2.0.0", Select: []string{"skills/docx", "skills/pdf"}},
        },
    }
    var buf bytes.Buffer
    if err := Write(original, &buf); err != nil { t.Fatalf(...) }

    // Verify YAML output format
    output := buf.String()
    // "simple" should be a scalar
    if !strings.Contains(output, "simple: github.com/org/a@v1.0.0") {
        t.Error("Simple dep should be written as scalar")
    }
    // "selective" should be a mapping
    if !strings.Contains(output, "url: github.com/org/b@v2.0.0") {
        t.Error("Structured dep should have url field")
    }

    // Round-trip
    parsed, err := Parse(&buf)
    if err != nil { t.Fatalf(...) }
    if parsed.Dependencies["simple"].URL != "github.com/org/a@v1.0.0" {
        t.Error("Simple dep URL mismatch")
    }
    if len(parsed.Dependencies["simple"].Select) != 0 {
        t.Error("Simple dep should have no Select")
    }
    if parsed.Dependencies["selective"].URL != "github.com/org/b@v2.0.0" {
        t.Error("Structured dep URL mismatch")
    }
    if len(parsed.Dependencies["selective"].Select) != 2 {
        t.Error("Structured dep should have 2 Select paths")
    }
}
```

**New test: `TestParseStringAndObjectDeps`**
```go
func TestParseStringAndObjectDeps(t *testing.T) {
    input := `schema_version: 1
name: mixed-deps
skills:
  - ./skill
dependencies:
  simple: github.com/org/a@v1.0.0
  selective:
    url: github.com/org/b@v2.0.0
    select:
      - skills/docx
      - skills/pdf
`
    m, err := Parse(strings.NewReader(input))
    if err != nil { t.Fatalf("Parse failed: %v", err) }

    if m.Dependencies["simple"].URL != "github.com/org/a@v1.0.0" { ... }
    if len(m.Dependencies["simple"].Select) != 0 { ... }
    if m.Dependencies["selective"].URL != "github.com/org/b@v2.0.0" { ... }
    if len(m.Dependencies["selective"].Select) != 2 { ... }
}
```

### Validation Tests (validate_test.go)

**New test: `TestValidateSelectPaths`**
```go
func TestValidateSelectPaths(t *testing.T) {
    tests := []struct {
        name    string
        sel     []string
        wantErr bool
    }{
        {"valid relative paths", []string{"skills/docx", "skills/pdf"}, false},
        {"absolute path", []string{"/skills/docx"}, true},
        {"path traversal", []string{"skills/../secrets"}, true},
        {"empty select", []string{}, false},
        {"leading dot-slash (normalized)", []string{"./skills/docx"}, false}, // depends on normalization in validate
    }
    for _, tc := range tests {
        m := &Manifest{
            SchemaVersion: 1,
            Name:          "test",
            Skills:        []string{"./skill"},
            Dependencies:  map[string]DependencySpec{"dep": {URL: "github.com/org/repo@v1.0.0", Select: tc.sel}},
        }
        errs := Validate(m)
        hasSelectErr := false
        for _, e := range errs {
            if strings.Contains(e.Error(), "select") {
                hasSelectErr = true
            }
        }
        if tc.wantErr && !hasSelectErr { t.Errorf(...) }
        if !tc.wantErr && hasSelectErr { t.Errorf(...) }
    }
}
```

### Resolver Filter Tests

**New test: `TestFilterBySelect`** in `resolver_test.go`:
```go
func TestFilterBySelect(t *testing.T) {
    names := []string{"docx", "pdf", "xlsx"}
    dirs := []string{"skills/docx", "skills/pdf", "skills/xlsx"}
    files := map[string][]byte{
        "skills/docx/SKILL.md": []byte("docx"),
        "skills/pdf/SKILL.md":  []byte("pdf"),
        "skills/xlsx/SKILL.md": []byte("xlsx"),
    }

    // Select 2 of 3
    n, d, f, err := filterBySelect(names, dirs, files, []string{"skills/docx", "skills/pdf"})
    if err != nil { t.Fatalf(...) }
    if len(n) != 2 { t.Errorf("expected 2 names, got %d", len(n)) }
    if len(d) != 2 { t.Errorf("expected 2 dirs, got %d", len(d)) }
    // Verify files only include docx and pdf

    // Select non-existent path
    _, _, _, err = filterBySelect(names, dirs, files, []string{"skills/nonexistent"})
    if err == nil { t.Error("expected error for non-existent path") }

    // Empty select = all
    n, d, f, err = filterBySelect(names, dirs, files, nil)
    if err != nil { t.Fatalf(...) }
    if len(n) != 3 { t.Error("empty select should return all") }
}
```

### MVS Select Merge Tests

Test the deduplication and "empty = all" merge logic in a resolver integration test.

---

## 8. Recommended Phases

### Phase 1: Core Type Changes and Parse/Write Round-Trip
**Goal:** Change the fundamental type and ensure all existing code compiles and tests pass.

**Changes:**
1. Add `DependencySpec` struct to `types.go` with `UnmarshalYAML`
2. Change `Manifest.Dependencies` from `map[string]string` to `map[string]DependencySpec`
3. Add `addDependencies` to `write.go`, update `Write()` call
4. Update `validate.go` loop (`dep.URL` instead of `url`)
5. Add select path validation (absolute, traversal checks)
6. Mechanical update of ALL consumers (§3 above — ~45 line changes across 14 files)
7. Update ALL test files with new type literals
8. Add new parse/write round-trip tests for both string and structured deps

**Verification:** `go build ./...` && `go test ./...` — all existing tests pass, plus new round-trip tests.

**Testable independently:** Yes. This phase produces a codebase where structured deps can be declared but `Select` is ignored at resolution time. Existing behavior is identical.

### Phase 2: DepURL Subpath Parsing
**Goal:** `ParseDepURL` accepts `#subpath` fragments.

**Changes:**
1. Add `Subpath` field to `DepURL` struct
2. Add `normalizeSubpath()` helper
3. Modify `ParseDepURL` to split on `#` before `@`
4. Update `String()` to append `#subpath`
5. Add ~7 new test cases to `depurl_test.go`
6. Add `TestDepURLSubpathMethods`

**Verification:** `go test ./internal/resolve/...` — all parser tests pass including new subpath cases.

**Testable independently:** Yes. Subpath is parsed but not yet used by any consumer.

### Phase 3: Resolver Filter and Select Merge
**Goal:** Resolution filters skills by `Select` and merges selects for same-package MVS.

**Changes:**
1. Add `Select []string` to `ResolvedDep` struct
2. Implement `filterBySelect()` in resolver.go
3. Insert filter call in `resolveOne()` after skill discovery
4. Thread `depSpec.Select` through `collectDeps()` into `ResolvedDep.Select`
5. Implement select-merge in MVS loop with `deduplicateStrings()`
6. Add `TestFilterBySelect` unit tests
7. Add integration test for select-merge scenario

**Verification:** `go test ./internal/resolve/...` — filter and merge tests pass. End-to-end: `craft install` with a structured dep selects correct skills.

**Testable independently:** Yes. This is the core functional feature — selective installation works.

### Phase 4: CLI `craft get` Subpath Support
**Goal:** `craft get url#subpath` installs a single skill.

**Changes:**
1. Extract `parsed.Subpath` into `depEntry.selectPaths` in `runGet()`
2. Use fragment-stripped URL for manifest storage
3. Construct `DependencySpec{URL: cleanURL, Select: selectPaths}` on line 196
4. Add CLI test for `craft get` with `#subpath`

**Verification:** `go test ./internal/cli/...` — get command tests pass with subpath argument.

### Phase 5: Interactive `craft add` Preview
**Goal:** `craft add` shows interactive skill selection for multi-skill packages.

**Changes:**
1. Create `internal/ui/select.go` with `MultiSelect` function
2. Add `previewSkills()` helper in `add.go` (discovers skills without full resolution)
3. Add `--all` flag to `addCmd`
4. Insert interactive flow in `runAdd()` between dep addition and resolution
5. Non-TTY detection: `term.IsTerminal(int(os.Stdin.Fd()))`
6. Add tests for `MultiSelect` (mock stdin)
7. Add CLI tests for `--all` flag behavior

**Verification:** Manual TTY testing + unit tests for `MultiSelect`.

### Phase 6: `craft update` New-Skill Discovery
**Goal:** `craft update` informs users about newly available skills.

**Changes:**
1. After resolution, for selectively installed deps, re-discover all available skills
2. Diff against selected skills to find new ones
3. Print informational message: `"ℹ New skills available in %s: %s (use 'craft add' to include)"`
4. Add test for new-skill notification output

**Verification:** CLI test verifying notification message appears when upstream adds skills.

### Phase Summary

| Phase | Files Changed | LOC Est. | Risk | Independently Testable |
|-------|--------------|----------|------|----------------------|
| 1: Core Types | 14 files | ~150 | Medium (wide refactor) | ✅ |
| 2: DepURL Subpath | 2 files | ~60 | Low | ✅ |
| 3: Resolver Filter | 2 files | ~120 | Medium | ✅ |
| 4: CLI get#subpath | 1 file | ~40 | Low | ✅ |
| 5: Interactive add | 3 files | ~150 | Medium (UI) | ✅ |
| 6: Update discovery | 1 file | ~50 | Low | ✅ |

**Total estimated: ~570 lines of code changes across 16 unique files.**

Phase 1 is the riskiest because it touches 14 files mechanically. It should be done first and verified with `go build ./... && go test ./...` before proceeding. Phases 2–4 can be implemented in sequence. Phases 5–6 are additive features that can be done in parallel or deferred.
