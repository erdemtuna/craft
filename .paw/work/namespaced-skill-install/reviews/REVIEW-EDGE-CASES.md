# Edge Cases Review — Namespaced Skill Installation

**Reviewer**: Edge Cases Specialist
**Scope**: Boundary analysis of namespaced skill install/remove changes
**Diff**: `internal/cli/install.go`, `internal/cli/remove.go`, `internal/resolve/resolver.go`, `internal/install/installer.go` and associated tests

---

## Boundary Categories Checked

For each code path modified in this diff, the following boundary categories were analyzed:

| Category | install.go collectSkillFiles | install.go verifyIntegrity | remove.go runRemove | remove.go cleanEmptyParents |
|---|---|---|---|---|
| Null/empty input | ✅ checked | ✅ checked | ❌ **finding** | ✅ checked |
| Partial failure | ✅ handled | ⚠️ **finding** | ❌ **finding** | ✅ handled |
| Duplicate/collision | ✅ handled by design | ✅ handled | ❌ **finding** | N/A |
| Interrupted operation | ✅ staging swap | ✅ read-only | ✅ RemoveAll is atomic | ✅ idempotent |
| Concurrent access | ✅ staging swap | ✅ read-only | ⚠️ acceptable | ⚠️ acceptable |
| Maximum values | ✅ no new limits | ✅ no new limits | ✅ no new limits | ✅ bounded by depth |
| Symlinks | N/A | N/A | N/A | ⚠️ **finding** |

---

### Finding: Orphan detection uses bare skill names but cleanup uses namespaced paths — shared-name skills are never cleaned up

**Severity**: must-fix
**Confidence**: HIGH
**Category**: edge-cases

#### Grounds (Evidence)

In `internal/cli/remove.go:80-104`, the orphan detection logic builds `remainingSkills` from pinfile `entry.Skills`, which contains **bare skill names** (e.g., `"skill-creator"`):

```go
// remove.go:81-87
remainingSkills := make(map[string]bool)
for _, remainingURL := range m.Dependencies {
    if entry, ok := pf.Resolved[remainingURL]; ok {
        for _, s := range entry.Skills {
            remainingSkills[s] = true  // bare name: "skill-creator"
        }
    }
}
```

The orphaned check at lines 99-104 compares bare names:

```go
for _, s := range removedSkills {
    if !remainingSkills[s] {  // "skill-creator" IS in remainingSkills from dep-b
        orphaned = append(orphaned, s)
    }
}
```

But cleanup at lines 126-129 uses namespaced paths:

```go
if nsPrefix != "" {
    skillDir = filepath.Join(tp, nsPrefix, skillName)  // target/github.com/org/a/skill-creator
}
```

**Concrete scenario**: dep-a (`github.com/org/a@v1.0.0`) and dep-b (`github.com/org/b@v1.0.0`) both export `"skill-creator"`. Skills are installed at:
- `target/github.com/org/a/skill-creator/`
- `target/github.com/org/b/skill-creator/`

When the user runs `craft remove dep-a`, `"skill-creator"` is in `remainingSkills` (from dep-b), so it is NOT added to `orphaned`. dep-a's `github.com/org/a/skill-creator/` directory is **never cleaned up**. The empty parent directories `github.com/org/a/` also remain.

The test at `internal/cli/remove_test.go:180-184` acknowledges this with a comment but does not assert that dep-a's shared-skill IS removed — it only checks that `unique-a` is removed.

#### Warrant (Rule)

With namespaced paths, skills from different deps are **independent on disk** even when they share a name. The entire premise of this PR is that `github.com/org/a/skill-creator` and `github.com/org/b/skill-creator` are distinct paths that don't collide. The orphan detection must reflect this: when dep-a is removed, **all** of dep-a's skills should be cleaned up regardless of whether another dep exports the same bare name, because the disk paths are different. The current bare-name comparison is a holdover from the flat-path layout where same-name skills truly occupied the same path.

#### Rebuttal Conditions

This is NOT a concern if: (1) the pinfile `Skills` field stores namespaced names rather than bare names — but `pinfile/types.go:32` shows it stores bare names; or (2) the design intentionally retains dep-a's files when dep-b shares a name — but this contradicts the spec at Spec.md line 43-44 ("only that dependency's skills to be cleaned up") and leaves unreachable files on disk.

#### Suggested Verification

Fix: With namespacing, the `remainingSkills` check for same-name retention is no longer necessary for disk cleanup. Every skill from the removed dep should be treated as orphaned because its disk path (`nsPrefix/skillName`) is unique to that dep. Simplify: replace the orphaned detection with `orphaned = removedSkills` (all skills from the removed dep are orphaned). Alternatively, make `remainingSkills` use composite keys `(depURL, skillName)` instead of bare names. Add a test that removes dep-a when dep-b exports the same skill name, and assert dep-a's namespaced directory IS deleted.

---

### Finding: ParseDepURL failure in remove.go silently skips cleanup — files installed at namespaced paths become unreachable

**Severity**: should-fix
**Confidence**: MEDIUM
**Category**: edge-cases

#### Grounds (Evidence)

In `internal/cli/remove.go:115-120`:

```go
parsed, parseErr := resolve.ParseDepURL(depURL)
var nsPrefix string
if parseErr == nil {
    nsPrefix = parsed.PackageIdentity()
}
```

If `ParseDepURL` fails, `nsPrefix` remains `""`, and cleanup falls through to the flat path at line 131: `skillDir = filepath.Join(tp, skillName)`. But skills were **installed** under namespaced paths (e.g., `target/github.com/org/repo/skill-creator/`). The flat path `target/skill-creator/` doesn't exist, `os.Stat` at line 146 returns an error, and the skill is silently not removed.

Meanwhile, in `collectSkillFiles` (install.go:252-254), `ParseDepURL` failure is a hard error that aborts installation. This asymmetry means: if a dep URL somehow fails to parse during remove but succeeded during install, the namespaced files are orphaned with no warning to the user.

#### Warrant (Rule)

Every input has an implicit boundary. The `depURL` value comes from `m.Dependencies[alias]` which was written by the user in `craft.yaml`. While `ParseDepURL` should succeed for any URL that was successfully installed, defensive code should not silently degrade to a wrong path. The `continue` on parse failure would produce a confusing user experience: "cleaned up 0 orphaned skills" while files remain on disk.

#### Rebuttal Conditions

This is NOT a concern if: (1) `ParseDepURL` is guaranteed to succeed for any URL in a valid manifest — but the manifest schema only validates URL format loosely; or (2) the code logs a warning on parse failure — but it currently does not.

#### Suggested Verification

Add a warning when `ParseDepURL` fails in remove.go: `cmd.PrintErrf("warning: could not parse dep URL %q for cleanup: %v\n", depURL, parseErr)`. Alternatively, return an error since a URL that was valid during install should always be valid during remove.

---

### Finding: verifyIntegrity silently skips integrity checking when ParseDepURL fails — cache corruption bypasses detection

**Severity**: should-fix
**Confidence**: MEDIUM
**Category**: edge-cases

#### Grounds (Evidence)

In `internal/cli/install.go:306-309`:

```go
parsed, err := resolve.ParseDepURL(dep.URL)
if err != nil {
    continue  // silently skip integrity check for this dep
}
```

If `ParseDepURL` fails, the entire integrity verification for that dependency is skipped without error or warning. A tampered or corrupted dependency would bypass the integrity check.

In contrast, `collectSkillFiles` at line 252-254 treats `ParseDepURL` failure as a hard error. The inconsistency means: if `collectSkillFiles` succeeds, `verifyIntegrity` should also be able to parse the same URLs. But the silent `continue` masks any logic errors that might cause the two functions to diverge.

#### Warrant (Rule)

Integrity verification is a security boundary. Silent skipping on any condition that isn't a clear "this dep doesn't need checking" weakens the guarantee. The existing `continue` for missing pinfile entries (line 302) is appropriate — those deps genuinely don't have digests. But `ParseDepURL` failure is an unexpected condition that should surface.

#### Rebuttal Conditions

This is NOT a concern if: `collectSkillFiles` always runs before `verifyIntegrity` (it does — line 107 vs 113 in `runInstall`), and both use the same `dep.URL` values from the same `result.Resolved` slice. In practice, a parse failure in verify implies one already occurred in collect, which would have aborted. The risk is theoretical but the silent skip is fragile.

#### Suggested Verification

Change the `continue` to return an error: `return fmt.Errorf("verifying integrity for %s: %w", dep.URL, err)`. This maintains the principle that unexpected conditions in integrity checking should surface, not hide.

---

### Finding: cleanEmptyParents uses filepath.Abs (not EvalSymlinks) — symlinked intermediate directories could be removed

**Severity**: consider
**Confidence**: LOW
**Category**: edge-cases

#### Grounds (Evidence)

In `internal/cli/remove.go:175-190`, `cleanEmptyParents` uses `filepath.Abs` for the boundary check:

```go
absDir, err := filepath.Abs(dir)
if err != nil || absDir == absRoot || !strings.HasPrefix(absDir, absRoot+string(filepath.Separator)) {
    break
}
if err := os.Remove(dir); err != nil {
    break
}
```

`filepath.Abs` does **not** resolve symlinks (only `filepath.EvalSymlinks` does). If an intermediate directory in the namespace path (e.g., `github.com/org/`) is a symlink, `os.Remove` removes the symlink itself rather than the target directory. The `HasPrefix` check compares lexical paths, so a symlink whose target is outside the root would still pass if the symlink's name is under root.

#### Warrant (Rule)

In practice, symlinks in the install target directory tree are unlikely — craft creates these directories itself via `os.MkdirAll`. However, `os.Remove` on a symlink to a non-empty directory would succeed (removing the symlink), potentially orphaning the target's contents. The blast radius is low: only craft-managed directories under the install target are affected.

#### Rebuttal Conditions

This is NOT a concern if: (1) no symlinks exist in the install target tree — craft creates real directories, and nothing in the workflow creates symlinks there; or (2) the `os.Remove` failure on non-empty directories already provides safety — `os.Remove` fails on non-empty real directories but succeeds on symlinks regardless of target contents.

#### Suggested Verification

No code change needed for the common case. If symlink safety is desired in the future, use `filepath.EvalSymlinks` and check that the resolved path is still under the resolved root before removing. Low priority given current usage.

---

### Finding: Dep exporting zero skills — no namespace directory created (handled correctly)

**Severity**: N/A (no issue found)
**Confidence**: HIGH
**Category**: edge-cases

#### Examination Summary

In `internal/cli/install.go:266`, the `for i, skillName := range dep.Skills` loop body is never entered when `dep.Skills` is empty. No composite key is inserted into the `skills` map, no namespace directory is created by `installer.go`, and no empty directory cleanup is needed. The same applies in `verifyIntegrity` — the `combined` map stays empty and produces a valid (deterministic) digest. The zero-skills case is handled correctly by the absence of iteration.

---

### Finding: Two dep URLs with the same PackageIdentity at different versions — MVS prevents this, but verify the assumption

**Severity**: consider
**Confidence**: MEDIUM
**Category**: edge-cases

#### Grounds (Evidence)

In `internal/cli/install.go:288`:

```go
compositeKey := prefix + "/" + skillName
skills[compositeKey] = files
```

If two entries in `result.Resolved` have the same `PackageIdentity()` (e.g., `github.com/org/repo` at v1.0.0 and v2.0.0) and export the same skill name, the second iteration **silently overwrites** the first in the `skills` map. There is no error or warning.

The resolver at `internal/resolve/resolver.go` uses MVS (Minimum Version Selection) which should reduce same-package deps to a single version. But transitive dependency resolution could theoretically produce two entries for the same package identity if MVS has a bug or if the `ResolvedDep` list contains duplicates.

#### Warrant (Rule)

Map key collision with silent overwrite is a latent data-loss pattern. The current code assumes `result.Resolved` never contains two entries with the same `PackageIdentity()` + skill name. This assumption is valid given MVS but is not enforced at this layer.

#### Rebuttal Conditions

This is NOT a concern if: (1) MVS is correctly implemented and guarantees at most one version per `PackageIdentity()` in the resolved set — which appears to be the case; or (2) a pre-existing validation in the resolver prevents duplicates.

#### Suggested Verification

Add a defensive check: if `compositeKey` already exists in the `skills` map when inserting, log a warning or return an error. This costs one map lookup and catches resolver bugs early. Low priority given MVS guarantees.

---

## Summary

| # | Finding | Severity | Confidence |
|---|---------|----------|------------|
| 1 | Orphan detection uses bare names — shared-name skills never cleaned up | must-fix | HIGH |
| 2 | ParseDepURL failure in remove.go silently skips cleanup | should-fix | MEDIUM |
| 3 | verifyIntegrity silently skips on ParseDepURL failure | should-fix | MEDIUM |
| 4 | cleanEmptyParents doesn't resolve symlinks | consider | LOW |
| 5 | Zero-skill deps handled correctly | N/A | HIGH |
| 6 | Same PackageIdentity collision — silent map overwrite | consider | MEDIUM |
