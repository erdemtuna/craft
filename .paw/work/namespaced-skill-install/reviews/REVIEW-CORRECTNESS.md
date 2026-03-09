# Correctness Review — Namespaced Skill Installation

Scope: Reviewed diff in `/tmp/review-diff.txt` against spec at `.paw/work/namespaced-skill-install/Spec.md` (esp. P2/FR-003). Focus is logic/spec fidelity for namespacing, integrity verification, and removal cleanup.

---

### Finding: `craft remove` orphan detection is still keyed only by skill *name*, so removing one of two deps with the same skill name will incorrectly skip deleting the removed dep’s namespaced skill directory

**Severity**: must-fix  
**Confidence**: HIGH  
**Category**: correctness

#### Grounds (Evidence)
- Spec requires same-name skills from different deps to coexist and removal to delete only the removed dependency’s skill (Spec.md:36-44).
- `internal/cli/remove.go:80-105` builds `remainingSkills` as `map[string]bool` keyed by **skill name** only, and computes `orphaned` by checking `if !remainingSkills[s]`.
- With namespacing, two deps can both provide `skill-creator`, but they are installed in different paths. However, the current logic treats `skill-creator` as “still needed” if *any* remaining dep provides the same name, so the removed dep’s copy won’t be considered orphaned and won’t be removed.
- The new comment in `internal/cli/remove_test.go:338-347` explicitly notes this mismatch (“The orphan check still uses skill NAMES…”), which indicates the implementation does not meet the P2 acceptance scenario.

#### Warrant (Rule)
When installation paths are namespaced by `host/owner/repo`, the “is this skill still needed?” question must be answered at least at the granularity of dependency identity (or namespaced path), not only by skill name. Otherwise, the remove operation can produce a semantically wrong result: leaving behind the removed dep’s installed artifacts.

#### Rebuttal Conditions
This is not a concern only if `craft remove` is intentionally defined to *never* delete a skill directory when another dependency has a same-named skill (even if it lives at a different namespaced path). That would contradict Spec.md P2/FR-003, so it would require a spec change.

#### Suggested Verification
Add an integration test: install two deps that both export `skill-creator`, remove one alias, and assert that `<target>/<removed-host>/<removed-owner>/<removed-repo>/skill-creator/` is gone while the other dep’s namespaced path remains. Also ensure removing an alias that still points to the same depURL doesn’t uninstall the dep.

---

### Finding: `craft remove` may remove empty namespace directories under *other* install targets even when no skill was removed from that target

**Severity**: should-fix  
**Confidence**: MEDIUM  
**Category**: correctness

#### Grounds (Evidence)
- In `internal/cli/remove.go:123-158`, the `removed` flag is declared per-skill, then iterates `for _, tp := range targetPath`.
- If the skill is removed from the first target, `removed` stays `true` for subsequent targets. The cleanup block `if removed && nsPrefix != "" { cleanEmptyParents(tp, filepath.Dir(skillDir)) }` (`internal/cli/remove.go:154-157`) can then run for a `tp` where the skill directory did not exist and nothing was removed.

#### Warrant (Rule)
Cleanup actions should be causally tied to the mutation they are meant to clean up. Running “remove empty parents” on a target where no directory was removed can delete empty directories that are unrelated to this operation (even if the deletion is constrained to “empty only”).

#### Rebuttal Conditions
Not a concern if (a) `targetPath` is always length 1 in practice, or (b) it is acceptable to opportunistically prune empty namespace directories under all targets whenever a dep is removed from any target.

#### Suggested Verification
If multiple targets are supported (multi-agent “Both”), add a test where only one target contains the removed dep’s namespaced dir and the other target contains an intentionally-empty namespace dir; verify it is not removed.

---

### Finding: `verifyIntegrity` now silently skips integrity verification for a dependency if `ParseDepURL` fails, weakening the “integrity must be checked” contract

**Severity**: should-fix  
**Confidence**: HIGH  
**Category**: correctness

#### Grounds (Evidence)
- `internal/cli/install.go:306-309`:
  ```go
  parsed, err := resolve.ParseDepURL(dep.URL)
  if err != nil {
      continue
  }
  ```
- This causes an entire dependency’s integrity check to be skipped if parsing fails, even when a pinfile integrity digest exists (`internal/cli/install.go:301-304`).

#### Warrant (Rule)
`verifyIntegrity` is explicitly described as preventing cache corruption/poisoning (internal/cli/install.go:296-299). Skipping verification on an unexpected-but-possible error path means the function can return success even when it failed to verify the intended property.

#### Rebuttal Conditions
Not a concern if it is provably impossible for any `dep.URL` in `result.Resolved` to fail `ParseDepURL` (i.e., a hard invariant enforced by the resolver and pinfile parser). In that case, this `continue` is dead code; otherwise it is a silent correctness gap.

#### Suggested Verification
Make `ParseDepURL` failures in this loop a hard error (or at least surface a warning + fail). Add a unit test that constructs a `ResolveResult` with an unparseable `dep.URL` and asserts that `verifyIntegrity` fails rather than silently succeeding.

---

### Finding: Composite key naming (`host/owner/repo/skillName`) is used as an OS path component; correctness depends on `PackageIdentity()` normalization being identical across install/remove/integrity

**Severity**: consider  
**Confidence**: MEDIUM  
**Category**: correctness

#### Grounds (Evidence)
- `internal/cli/install.go:257-290` uses `prefix := parsed.PackageIdentity()` and `compositeKey := prefix + "/" + skillName`.
- `internal/cli/remove.go:115-131` also uses `parsed.PackageIdentity()` for `nsPrefix` to construct the cleanup directory.

#### Warrant (Rule)
If `PackageIdentity()` normalization differs depending on how the URL is spelled (e.g., case differences, `.git` suffix, scheme vs no scheme), then the computed namespace may differ between installation and removal/integrity reconstruction, leading to orphaned directories or mismatched integrity computation.

#### Rebuttal Conditions
Not a concern if `ParseDepURL` canonicalizes identity strictly and all stored `depURL` strings (manifest + pinfile) are already normalized/canonical.

#### Suggested Verification
Add tests that install from different equivalent URL spellings (if supported) and confirm the namespace path is identical; also ensure remove uses the exact same canonical identity.
