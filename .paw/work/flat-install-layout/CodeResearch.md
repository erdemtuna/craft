# Code Research: Flat Install Layout

## 1. installer.go — `Install()` Function

**File**: `internal/install/installer.go`

### Signature
[installer.go:17](internal/install/installer.go#L17)
```go
func Install(target string, skills map[string]map[string][]byte) error
```

- `target`: root directory to install into (e.g., `~/.claude/skills/` or `forge/`)
- `skills`: map of composite key → (relative file path → content)

### Full Function Body (lines 17–88)

**Target directory creation and resolution** [installer.go:18–25](internal/install/installer.go#L18):
```go
if err := os.MkdirAll(target, 0o700); err != nil {
    return fmt.Errorf("creating target directory: %w", err)
}
absTarget, err := filepath.Abs(target)
if err != nil {
    return fmt.Errorf("resolving target path: %w", err)
}
```

**Skill iteration loop** [installer.go:27](internal/install/installer.go#L27):
```go
for skillName, files := range skills {
```
- `skillName` is the composite key (e.g., `github.com/org/repo/my-skill`)
- `files` is `map[string][]byte` of relative paths to content

**Directory creation via `filepath.Join`** [installer.go:28](internal/install/installer.go#L28):
```go
skillDir := filepath.Join(target, skillName)
```
- When `skillName` is `github.com/org/repo/my-skill`, this creates nested dirs automatically via OS path semantics.

**Path traversal validation** [installer.go:29–35](internal/install/installer.go#L29):
```go
absSkillDir, err := filepath.Abs(skillDir)
if err != nil {
    return fmt.Errorf("resolving skill path: %w", err)
}
if !strings.HasPrefix(absSkillDir, absTarget+string(filepath.Separator)) {
    return fmt.Errorf("skill name %q escapes target directory", skillName)
}
```

**Staging directory** [installer.go:38–44](internal/install/installer.go#L38):
```go
stagingDir := skillDir + ".staging"
_ = os.RemoveAll(stagingDir)
if err := os.MkdirAll(stagingDir, 0o700); err != nil {
    return fmt.Errorf("creating staging directory for %q: %w", skillName, err)
}
```

**File writing with per-file path traversal check** [installer.go:47–68](internal/install/installer.go#L47):
```go
writeErr := func() error {
    for relPath, content := range files {
        fullPath := filepath.Join(stagingDir, relPath)
        absFullPath, err := filepath.Abs(fullPath)
        // ...validate against staging dir...
        if !strings.HasPrefix(absFullPath, filepath.Clean(stagingDir)+string(filepath.Separator)) {
            return fmt.Errorf("file path %q escapes skill directory", relPath)
        }
        // ...MkdirAll + WriteFile...
    }
    return nil
}()
```

**Atomic swap** [installer.go:76–84](internal/install/installer.go#L76):
```go
if err := os.RemoveAll(skillDir); err != nil { ... }
if err := os.Rename(stagingDir, skillDir); err != nil { ... }
```

### Key Observations for `InstallFlat()`
- `Install()` already handles composite keys as `skillName` — it just passes them through `filepath.Join(target, skillName)` which creates nested directories.
- `InstallFlat()` can transform keys via `FlatKey()` then call `Install()` with modified map, OR duplicate the loop with flat path construction.
- The spec says `InstallFlat()` should delegate to `Install()` — transforming keys then passing through. This reuses all staging, validation, and atomicity logic.

---

## 2. install.go — Global vs Project Install Calls

**File**: `internal/cli/install.go`

### Global Install Path

**`runInstall` dispatches on `globalFlag`** [install.go:43–48](internal/cli/install.go#L43):
```go
func runInstall(cmd *cobra.Command, args []string) error {
    if globalFlag {
        return runInstallGlobal(cmd)
    }
    return runInstallProject(cmd)
}
```

**Target resolution for global** [install.go:106–109](internal/cli/install.go#L106):
```go
targetPaths, err := resolveInstallTargets(installTarget)
if err != nil {
    return err
}
```
- `targetPaths` is `[]string` — one or more agent install directories (e.g., `~/.claude/skills/`).
- Resolved by `resolveInstallTargets()` at [install.go:256](internal/cli/install.go#L256).

**Skill files collection** [install.go:111–115](internal/cli/install.go#L111):
```go
skillFiles, err := collectSkillFiles(fetcher, result)
```
- Returns `map[string]map[string][]byte` — composite key → files.

**`Install()` call for global** [install.go:123–128](internal/cli/install.go#L123):
```go
for _, targetPath := range targetPaths {
    if err := installlib.Install(targetPath, skillFiles); err != nil {
        progress.Fail("Installation failed")
        return fmt.Errorf("installation failed: %w", err)
    }
}
```
→ **This loop needs to change to call `installlib.InstallFlat(targetPath, skillFiles)` for global installs.**

### Project Install Path

**forge path** [install.go:206](internal/cli/install.go#L206):
```go
forgePath := filepath.Join(root, "forge")
```

**`Install()` call for project** [install.go:222–226](internal/cli/install.go#L222):
```go
if err := installlib.Install(forgePath, skillFiles); err != nil {
    progress.Fail("Vendoring failed")
    return fmt.Errorf("vendoring failed: %w", err)
}
```
→ **This stays as `Install()` — project layout unchanged.**

### `collectSkillFiles` — Composite Key Construction

[install.go:351–397](internal/cli/install.go#L351)

Composite key is built at [install.go:391](internal/cli/install.go#L391):
```go
compositeKey := prefix + "/" + skillName
skills[compositeKey] = files
```
Where `prefix` = `parsed.PackageIdentity()` (e.g., `github.com/org/repo`) and `skillName` is the skill's leaf name (e.g., `my-skill`). Result: `github.com/org/repo/my-skill`.

### Import Alias

[install.go:14](internal/cli/install.go#L14):
```go
installlib "github.com/erdemtuna/craft/internal/install"
```

---

## 3. get.go — `Install()` Call

**File**: `internal/cli/get.go`

**Import** [get.go:10](internal/cli/get.go#L10):
```go
installlib "github.com/erdemtuna/craft/internal/install"
```

**Target resolution** [get.go:220](internal/cli/get.go#L220):
```go
targetPaths, err := resolveInstallTargets(getTarget)
```

**Skill files collection** [get.go:226](internal/cli/get.go#L226):
```go
skillFiles, err := collectSkillFiles(fetcher, result)
```

**`Install()` call** [get.go:240–245](internal/cli/get.go#L240):
```go
for _, targetPath := range targetPaths {
    if err := installlib.Install(targetPath, skillFiles); err != nil {
        progress.Fail("Installation failed")
        return fmt.Errorf("installation failed: %w\n  note: ...", err)
    }
}
```
→ **Needs to change to `installlib.InstallFlat()`** — `get` is always global scope.

---

## 4. update.go — `Install()` Call

**File**: `internal/cli/update.go`

**Import** [update.go:12](internal/cli/update.go#L12):
```go
installlib "github.com/erdemtuna/craft/internal/install"
```

**Global branch** [update.go:214–231](internal/cli/update.go#L214):
```go
if globalFlag {
    targetPaths, err := resolveInstallTargets(updateTarget)
    // ...
    for _, targetPath := range targetPaths {
        if err := installlib.Install(targetPath, skillFiles); err != nil {
            progress.Fail("Installation failed")
            return fmt.Errorf("installation failed: %w", err)
        }
    }
```
→ **`Install()` at [update.go:227](internal/cli/update.go#L227) needs to change to `InstallFlat()` for global scope.**

**Project branch** [update.go:244–254](internal/cli/update.go#L244):
```go
forgePath := filepath.Join(root, "forge")
// ...
if err := installlib.Install(forgePath, skillFiles); err != nil {
```
At [update.go:251](internal/cli/update.go#L251) — **stays as `Install()`**.

---

## 5. remove.go — Deletion Logic

**File**: `internal/cli/remove.go`

### Scope Branching

**Global vs project** [remove.go:37–53](internal/cli/remove.go#L37):
```go
if globalFlag {
    manifestPath, err = GlobalManifestPath()
    // ...
    pfPath, err = GlobalPinfilePath()
} else {
    root, err = os.Getwd()
    // ...
    manifestPath = filepath.Join(root, "craft.yaml")
    pfPath = filepath.Join(root, "craft.pin.yaml")
}
```

### Skill Name Source

**Removed skill names from pinfile** [remove.go:78–82](internal/cli/remove.go#L78):
```go
if pfErr == nil {
    if entry, ok := pf.Resolved[depURL]; ok {
        removedSkills = entry.Skills
    }
}
```
- `removedSkills` contains leaf skill names (e.g., `["my-skill", "other-skill"]`), NOT composite keys.

### Target Resolution for Cleanup

[remove.go:111–121](internal/cli/remove.go#L111):
```go
if globalFlag {
    targetPath, err = resolveInstallTargets(removeTarget)
} else if removeTarget != "" {
    targetPath = []string{removeTarget}
} else {
    targetPath = []string{filepath.Join(root, "forge")}
}
```

### Path Construction for Deletion

**Namespace prefix** [remove.go:129–135](internal/cli/remove.go#L129):
```go
parsed, parseErr := resolve.ParseDepURL(depURL)
// ...
nsPrefix := parsed.PackageIdentity()
```
- `nsPrefix` = `github.com/org/repo`

**Skill directory path** [remove.go:141](internal/cli/remove.go#L141):
```go
skillDir := filepath.Join(tp, nsPrefix, skillName)
```
- Produces nested path: `~/.claude/skills/github.com/org/repo/my-skill`
→ **For global scope, this needs to use `FlatKey(nsPrefix + "/" + skillName)` instead, producing `~/.claude/skills/github-com--org--repo--my-skill`.**

### Path Traversal Protection

[remove.go:143–154](internal/cli/remove.go#L143):
```go
absSkillDir, err := filepath.Abs(skillDir)
// ...
absTarget, err := filepath.Abs(tp)
// ...
if !strings.HasPrefix(absSkillDir, absTarget+string(filepath.Separator)) {
    cmd.PrintErrf("  warning: skill name %q escapes target directory, skipping\n", skillName)
    continue
}
```

### Deletion + Parent Cleanup

[remove.go:156–162](internal/cli/remove.go#L156):
```go
if _, err := os.Stat(skillDir); err == nil {
    if err := os.RemoveAll(skillDir); err != nil {
        cmd.PrintErrf("  warning: could not remove %s: %v\n", skillDir, err)
    } else {
        removedFromAny = true
        cleanEmptyParents(tp, filepath.Dir(skillDir))
    }
}
```
→ **For global flat layout, `cleanEmptyParents` should be skipped (no nested parents exist).**

### `cleanEmptyParents` Function

[remove.go:181–196](internal/cli/remove.go#L181):
```go
func cleanEmptyParents(root, dir string) {
    absRoot, err := filepath.Abs(root)
    if err != nil {
        return
    }
    for {
        absDir, err := filepath.Abs(dir)
        if err != nil || absDir == absRoot || !strings.HasPrefix(absDir, absRoot+string(filepath.Separator)) {
            break
        }
        if err := os.Remove(dir); err != nil {
            break // not empty or permission error
        }
        dir = filepath.Dir(dir)
    }
}
```
- Walks up from `dir` to `root`, removing empty dirs. Safe — `os.Remove` fails on non-empty.
- For flat layout, `filepath.Dir(flatSkillDir)` == `target`, so `cleanEmptyParents` would be a no-op anyway. But spec says to skip the call entirely for clarity.

---

## 6. list.go / tree.go — Skill Name Display

### list.go

**File**: `internal/cli/list.go`

**Data source**: pinfile entries [list.go:59–77](internal/cli/list.go#L59):
```go
for key, entry := range pf.Resolved {
    parsed, err := resolve.ParseDepURL(key)
    // ...
    alias := urlToAlias[parsed.PackageIdentity()]
    if alias == "" {
        alias = parsed.Repo
    }
    deps = append(deps, depInfo{
        alias:   alias,
        version: parsed.RefString(),
        url:     parsed.PackageIdentity(),
        skills:  entry.Skills,
    })
}
```

**Display — default (tabular)** [list.go:94–101](internal/cli/list.go#L94):
```go
_, _ = fmt.Fprintf(w, "%s\t%s\t(%d %s)\n", sanitize(d.alias), d.version, len(d.skills), skillWord)
```
- Shows alias, version, skill count. Does NOT show individual skill names in default mode.

**Display — detailed** [list.go:84–92](internal/cli/list.go#L84):
```go
cmd.Printf("%s  %s  %s\n", sanitize(d.alias), d.version, sanitize(d.url))
if len(d.skills) > 0 {
    cmd.Printf("  skills: %s\n", sanitize(strings.Join(d.skills, ", ")))
}
```
- `d.url` = `parsed.PackageIdentity()` = `github.com/org/repo`
- `d.skills` = `entry.Skills` = leaf skill names from pinfile (e.g., `["my-skill"]`)
- **Skill names displayed are leaf names from pinfile**, not composite keys and not flat keys.

### tree.go

**File**: `internal/cli/tree.go`

**Data source**: pinfile entries [tree.go:47–65](internal/cli/tree.go#L47):
```go
for key, entry := range pf.Resolved {
    parsed, err := resolve.ParseDepURL(key)
    // ...
    alias := urlToAlias[parsed.PackageIdentity()]
    if alias == "" {
        alias = parsed.Repo
    }
    deps = append(deps, ui.DepNode{
        Alias:  alias,
        URL:    key,
        Skills: entry.Skills,
    })
}
```

**Rendering** via `ui.RenderTree` at [tree.go:67](internal/cli/tree.go#L67):
```go
ui.RenderTree(cmd.OutOrStdout(), packageName, localSkills, deps)
```

### ui/tree.go — RenderTree

**File**: `internal/ui/tree.go`

**Dep node display** [tree.go:58](internal/ui/tree.go#L58):
```go
_, _ = fmt.Fprintf(w, "%s%s (%s)\n", connector, dep.Alias, dep.URL)
```
- Shows alias and full dep URL (e.g., `github.com/org/repo@v1.0.0`)

**Skill display** [tree.go:60–65](internal/ui/tree.go#L60):
```go
for j, skill := range dep.Skills {
    // ...
    _, _ = fmt.Fprintf(w, "%s%s%s\n", childPrefix, skillConn, skill)
}
```
- `skill` = leaf name from pinfile (e.g., `my-skill`).

### Key Observation for Spec FR-007

The spec says `list -g` and `tree -g` should display skills using **composite key format** (`github.com/org/repo/skill`). Currently:

- **list.go default**: shows alias + version + skill count — no change needed (no skill names displayed).
- **list.go --detailed**: shows `d.url` (= `PackageIdentity()` = `github.com/org/repo`) on the header line, then leaf skill names. The spec wants composite key format for skills → change `d.skills` display to prefix each with `d.url + "/"`.
- **tree.go**: shows leaf skill names under each dep node. The spec wants composite key format → prefix each skill with the package identity.

Both commands get skill names from **pinfile** (`entry.Skills`), not from filesystem scanning. No filesystem scan is involved — this is purely a display concern.

---

## 7. installer_test.go — Test Structure

**File**: `internal/install/installer_test.go`

### Test Inventory

| Test | Lines | What it tests |
|------|-------|---------------|
| `TestInstallCreatesStructure` | [10–30](internal/install/installer_test.go#L10) | Basic install creates dir + files |
| `TestInstallOverwrites` | [32–53](internal/install/installer_test.go#L32) | Second install replaces content |
| `TestInstallEmpty` | [55–60](internal/install/installer_test.go#L55) | Empty skills map is no-op |
| `TestInstallRejectsTraversalSkillName` | [62–76](internal/install/installer_test.go#L62) | `../../etc/malicious` → error |
| `TestInstallRejectsTraversalFilePath` | [78–92](internal/install/installer_test.go#L78) | File path traversal → error |
| `TestInstallAllowsNormalSkillNames` | [94–108](internal/install/installer_test.go#L94) | Normal names + nested files OK |
| `TestInstallRejectsDotSkillName` | [110–121](internal/install/installer_test.go#L110) | `.` skill name → error |
| `TestInstallRejectsEmptySkillName` | [123–134](internal/install/installer_test.go#L123) | Empty skill name → error |
| `TestInstallCleansUpStagingOnError` | [136–151](internal/install/installer_test.go#L136) | Staging dir cleaned on failure |
| `TestInstallAtomicOverwrite` | [153–177](internal/install/installer_test.go#L153) | Overwrite is atomic (old → new) |
| `TestInstallCompositeKeys` | [179–210](internal/install/installer_test.go#L179) | Composite keys create nested dirs |

### Test Patterns

**Temp directories**: All tests use `t.TempDir()` [installer_test.go:11](internal/install/installer_test.go#L11):
```go
target := filepath.Join(t.TempDir(), "skills")
```

**Skills map construction**: Inline `map[string]map[string][]byte{ ... }` literals.

**Assertions**: Standard `t.Fatalf`/`t.Errorf` with `os.ReadFile`, `os.Stat`, `os.ReadDir`, `strings.Contains`.

**No test helpers**: Each test is self-contained with inline setup. No shared helper functions.

**No subtests**: All top-level test functions, no `t.Run()` subtests.

### Key Test for Reference: `TestInstallCompositeKeys`

[installer_test.go:179–210](internal/install/installer_test.go#L179) — Tests that composite keys like `github.com/org/repo/my-skill` create nested directory structures. This test should remain passing unchanged after adding `InstallFlat()`.

---

## Summary: Required Changes by File

| File | Change | Lines Affected |
|------|--------|----------------|
| `internal/install/installer.go` | Add `FlatKey()` + `InstallFlat()` | New code after line 88 |
| `internal/cli/install.go` | Global: `Install()` → `InstallFlat()` | [124](internal/cli/install.go#L124) |
| `internal/cli/get.go` | `Install()` → `InstallFlat()` | [241](internal/cli/get.go#L241) |
| `internal/cli/update.go` | Global: `Install()` → `InstallFlat()` | [227](internal/cli/update.go#L227) |
| `internal/cli/remove.go` | Global: flat path + skip `cleanEmptyParents` | [141](internal/cli/remove.go#L141), [161](internal/cli/remove.go#L161) |
| `internal/cli/list.go` | Detailed: prefix skills with composite key | [87](internal/cli/list.go#L87) |
| `internal/cli/tree.go` | Prefix skills with package identity | [60–65](internal/ui/tree.go#L60) |
| `internal/install/installer_test.go` | Add `TestFlatKey*`, `TestInstallFlat*` tests | New code after line 210 |
