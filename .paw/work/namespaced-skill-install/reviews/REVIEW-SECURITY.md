# Security Review: Namespaced Skill Installation

## Examination Summary
I analyzed the changes in `internal/cli/install.go`, `internal/cli/remove.go`, and `internal/install/installer.go` focusing on path traversal, filesystem integrity, and cleanup logic. The introduction of composite keys (`host/owner/repo/skill`) and the removal of collision detection were evaluated for security implications. The core path traversal protections in `Install` appear robust even with deeper nesting, as they rely on `filepath.Abs` and prefix checking against the target root. However, I identified two significant issues regarding data cleanup and potential filesystem collisions.

## Findings

### Finding: Stale code persistence due to incomplete cleanup on removal

**Severity**: should-fix
**Confidence**: HIGH
**Category**: security

#### Grounds (Evidence)
In `internal/cli/remove.go`, the `runRemove` function identifies orphaned skills using logic from the old flat-layout era:
```go
// Find orphaned skills (only in removed dep, not in any remaining dep)
var orphaned []string
for _, s := range removedSkills {
    if !remainingSkills[s] {
        orphaned = append(orphaned, s)
    }
}
```
This logic explicitly skips removal of a skill if *any other* dependency exports a skill with the same name (`if !remainingSkills[s]`).

#### Warrant (Rule)
In the new namespaced architecture, skills are **never** shared on disk. Dependency A installs to `.../DepA/skill` and Dependency B installs to `.../DepB/skill`. If the user removes Dependency A, `.../DepA/skill` must be deleted. The current logic incorrectly preserves `.../DepA/skill` if Dependency B also has a skill with the same name.
From a security perspective, "removing" a dependency must remove its code. If Dependency A is removed because it contains a vulnerability, this bug preserves the vulnerable code on disk (orphaned in `.../DepA/skill`) simply because Dependency B has a similarly-named skill. Tools scanning the install directory may still execute or bundle the vulnerable code.

#### Rebuttal Conditions
This would not be a security concern if the installation layout was still flat (shared directories), where reference counting would be required. However, the spec confirms the layout is now fully namespaced.

#### Suggested Verification
1. Install two dependencies (DepA, DepB) that both export `common-skill`.
2. Verify both exist on disk at their namespaced paths.
3. Run `craft remove DepA`.
4. Verify that `.../DepA/common-skill` still exists on disk (it should have been removed).

---

### Finding: Filesystem corruption via collision of same-repo/mixed-version dependencies

**Severity**: consider
**Confidence**: MEDIUM
**Category**: security

#### Grounds (Evidence)
The namespacing strategy uses `parsed.PackageIdentity()` (Host/Owner/Repo) as the directory prefix in `internal/cli/install.go`:
```go
prefix := parsed.PackageIdentity()
// ...
compositeKey := prefix + "/" + skillName
```
If the resolver allows the same repository to be included twice (e.g., via different aliases, or `https` vs `ssh` URLs) at *different versions*, they will map to the exact same directory structure `target/host/owner/repo/...`.

#### Warrant (Rule)
The removal of `detectCollisions` (FR-002) allows overlapping skills. However, without a collision detector or a version/alias discriminator in the path, two versions of the same repo will overwrite each other's files in a non-deterministic order (Last Write Wins during map iteration). This violates integrity: the on-disk state will be a corrupted mix of Version A and Version B files, or silently Version B when the user expects Version A for that alias.

#### Rebuttal Conditions
This is not a concern if the Resolver explicitly enforces a "Singleton Repo" rule (one version per repo identity globally), preventing the Diamond Dependency problem. If `craft` allows multiple versions of the same lib (like NPM), then this is a bug. Given `detectCollisions` was removed, the system is now more permissive, increasing the likelihood of this clash.

#### Suggested Verification
Add a test case with two dependencies pointing to the same git repo but different commits, using different aliases. Run install. Check if the files in `target/host/owner/repo` correspond to only one commit or a mix, and if the user is warned.

---

### Finding: `cleanEmptyParents` is safe but relies on strict usage

**Severity**: consider
**Confidence**: HIGH
**Category**: security

#### Grounds (Evidence)
In `internal/cli/remove.go`, `cleanEmptyParents` walks up the directory tree:
```go
for {
    absDir, err := filepath.Abs(dir)
    if err != nil || absDir == absRoot || !strings.HasPrefix(absDir, absRoot+string(filepath.Separator)) {
        break
    }
    if err := os.Remove(dir); err != nil { break }
    dir = filepath.Dir(dir)
}
```

#### Warrant (Rule)
This implementation is secure. It explicitly checks `!strings.HasPrefix` to ensure it never deletes directories outside the target root, and `absDir == absRoot` ensures it doesn't delete the root itself. It uses `os.Remove` (not `RemoveAll`), which guarantees only empty directories are removed. This finding confirms the safety of this sensitive operation, as requested in the review instructions.
