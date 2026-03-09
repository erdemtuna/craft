# Testing Specialist Review: Namespaced Skill Installation

**Specialist**: Testing  
**Review Date**: 2026-03-09  
**Confidence**: HIGH

## Executive Summary

This review analyzes the test coverage for the namespaced skill installation feature. The change namespaces installed skills by host/owner/repo to allow same-name skills from different dependencies. From a testing perspective, the implementation introduces several high-risk scenarios that lack adequate test coverage, particularly around error handling paths and the new `cleanEmptyParents()` function. The existing test suite has been updated for the new namespaced paths, but several gaps remain that could allow production regressions.

**Highest Priority Concerns**:
1. No tests for `cleanEmptyParents()` function — this recursive directory cleanup logic is untested
2. `verifyIntegrity()` error path (ParseDepURL failure) is untested
3. Non-GitHub hosts (GitLab, Bitbucket, self-hosted) are untested despite being in scope
4. `TestRunRemove_SharedSkillRetained` was simplified and may no longer verify the right behavior with namespacing

---

## Findings

### Finding: cleanEmptyParents function has no tests despite handling recursive directory deletion

**Severity**: must-fix  
**Confidence**: HIGH  
**Category**: testing

#### Grounds (Evidence)

In `internal/cli/remove.go:265-282`, the new `cleanEmptyParents()` function was introduced:

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

This function performs recursive directory cleanup, walking up the directory tree and removing empty parent directories. It has complex boundary conditions:
- Stops at the root boundary
- Handles filepath.Abs errors
- Handles os.Remove errors (non-empty dirs, permissions)
- Uses string prefix matching for security

Despite this complexity, there are **zero tests** in `internal/cli/remove_test.go` that directly exercise this function. The closest test is `TestRunRemove_ExistingDep` (line 82-89 of remove_test.go), which verifies that `github.com/org/repo` is cleaned up, but this is an indirect check buried in an integration test — not a focused unit test of the cleanup logic.

#### Warrant (Rule)

Recursive directory deletion is one of the highest-risk operations in a package manager. Off-by-one errors in boundary conditions could cause:
1. Deleting too much (removing root or parent directories outside the target)
2. Deleting too little (leaving empty directories that accumulate over time)
3. Infinite loops (if `filepath.Dir` doesn't eventually reach root)
4. Path traversal vulnerabilities (if prefix check is incorrect)

The test at line 87-89 of `remove_test.go` checks that `github.com/org/repo` is gone, but it doesn't verify:
- What happens when `root` and `dir` are equal (should stop immediately)
- What happens when `dir` is already outside `root` (should not delete anything)
- What happens when a parent dir is non-empty (should stop at that level)
- What happens when `filepath.Abs` fails (should return gracefully)
- Whether it correctly stops before deleting `root` itself

These are classic boundary conditions for recursive algorithms, and they're untested. If a bug is introduced (e.g., changing `absDir == absRoot` to `absDir != absRoot`), the existing tests might still pass while the function deletes the wrong directories.

#### Rebuttal Conditions

This is NOT a concern if: (1) the integration test at `TestRunRemove_ExistingDep` has sufficient assertions that would catch all realistic boundary bugs (verify: does it test non-empty parent dirs, root boundary, and prefix violations?); or (2) `cleanEmptyParents` is a trivial wrapper around a well-tested stdlib function (check: `os.Remove` is tested by stdlib, but the loop logic and boundary conditions are not).

#### Suggested Verification

Add unit tests for `cleanEmptyParents()` covering:
1. **Normal case**: `root=/target`, `dir=/target/github.com/org/repo/skill` → removes `repo`, `org`, `github.com` if all empty
2. **Stops at non-empty**: `root=/target`, `dir=/target/github.com/org/repo/skill` with sibling file in `/target/github.com/org/repo/other-skill` → removes only `skill`, stops at `repo`
3. **Root boundary**: `root=/target`, `dir=/target` → removes nothing (stops immediately)
4. **Already outside root**: `root=/target`, `dir=/outside` → removes nothing
5. **Abs error handling**: Mock `filepath.Abs` to return error → function returns without panic
6. **Remove error handling**: Create a non-empty dir in the chain → stops at that dir without error

Mark these as unit tests (not integration tests) so they run fast and provide clear failure messages.

---

### Finding: verifyIntegrity() ParseDepURL error path is untested, allowing silent failures

**Severity**: must-fix  
**Confidence**: HIGH  
**Category**: testing

#### Grounds (Evidence)

In `internal/cli/install.go:338-343` (diff lines 38-43), `verifyIntegrity()` now calls `ParseDepURL()` and silently continues on error:

```go
parsed, err := resolve.ParseDepURL(dep.URL)
if err != nil {
    continue
}
```

This error path is **never exercised** in the test suite. In `internal/cli/install_test.go`, all three `verifyIntegrity` tests (`TestVerifyIntegrity_Pass`, `TestVerifyIntegrity_Mismatch`, `TestVerifyIntegrity_SkipsMissingPinEntry`) use well-formed URLs like `"github.com/org/repo@v1.0.0"`.

There is no test with a malformed URL that would cause `ParseDepURL` to fail (e.g., `"not-a-valid-url"`, `"github.com/invalid"`, `""`). The existing test `TestCollectSkillFiles_SkipsBadDepURL` (line 153-173) tests the error path in `collectSkillFiles()`, but not in `verifyIntegrity()`.

#### Warrant (Rule)

The `continue` on line 340 means that if `ParseDepURL` fails for a resolved dependency, that dependency's integrity is **never checked**. This is a security-critical code path — integrity verification is the mechanism that prevents cache poisoning attacks. If a malformed URL causes the verification to be skipped, an attacker could potentially inject tampered skill files without detection.

The behavioral contract for `verifyIntegrity()` should be: "for every resolved dependency, verify that the fetched files match the pinfile digest." The current implementation violates this contract silently when `ParseDepURL` fails. The tests don't verify this contract — they only verify that well-formed URLs are checked correctly.

A realistic regression scenario: a future refactor changes the URL format in `ResolvedDep`, causing `ParseDepURL` to fail for all dependencies. The `verifyIntegrity()` function would skip all checks and return success, and all existing tests would pass because they don't assert what happens when parsing fails.

#### Rebuttal Conditions

This is NOT a concern if: (1) `ParseDepURL` is guaranteed to succeed for all URLs that make it into a `ResolvedDep` (check: are there validation steps earlier in the pipeline that reject malformed URLs?); or (2) the `continue` behavior is intentional and safe (e.g., if a URL can't be parsed, it's acceptable to skip verification for that dep). Verify the design intent.

#### Suggested Verification

Add test: `TestVerifyIntegrity_SkipsMalformedURL`. Set up:
- Create a `ResolvedDep` with a malformed URL (e.g., `"not-a-valid-url"` from the existing test)
- Provide skill files for that dep with tampered content
- Provide a pinfile entry with a known-good digest

Assert: `verifyIntegrity()` returns success (not error) because it skips the malformed dep. Then add a comment explaining whether this is the intended behavior or a silent failure. If it's a silent failure, change the code to return an error or log a warning.

Alternative: If `ParseDepURL` failure is considered a bug (dependencies should never have malformed URLs), change the `continue` to `return fmt.Errorf("invalid dep URL: %w", err)` and update the test to expect an error.

---

### Finding: No tests for non-GitHub hosts despite being explicitly in scope

**Severity**: must-fix  
**Confidence**: HIGH  
**Category**: testing

#### Grounds (Evidence)

The spec (`Spec.md:69`) explicitly lists non-GitHub hosts as an edge case:
> "Non-GitHub hosts (GitLab, Bitbucket, self-hosted) produce valid namespace paths (e.g., `gitlab.com/org/repo/skill-name/`)."

Success criterion SC-005 states:
> "Skills from non-GitHub hosts install under the correct host-prefixed path."

However, **zero tests** in the test suite use non-GitHub hosts. All test URLs use `github.com`:
- `install_test.go`: `"github.com/org/repo"`, `"github.com/lossyrob/paw"`, `"github.com/anthropics/skills"`
- `remove_test.go`: `"github.com/org/a"`, `"github.com/org/b"`, `"github.com/org/repo"`
- `installer_test.go`: `"github.com/org/repo"`, `"github.com/other/tools"`

The namespacing logic in `collectSkillFiles()` uses `parsed.PackageIdentity()` (line 9, diff), which is supposed to work for any git host. The spec says it's in scope and has a success criterion, but there's no test verifying that `gitlab.com/org/repo@v1.0.0` produces `<target>/gitlab.com/org/repo/skill/` instead of some mangled or incorrect path.

#### Warrant (Rule)

The highest-risk scenario for multi-host support is that the code works for GitHub but silently fails for other hosts. Common failure modes:
1. **Hardcoded assumptions**: Code assumes "github.com" and breaks on other hosts
2. **URL parsing differences**: GitLab/Bitbucket URLs might parse differently (e.g., `gitlab.com` vs `www.gitlab.com`, `git@gitlab.com:org/repo.git` vs `https://gitlab.com/org/repo.git`)
3. **Path separator issues**: Some hosts might include extra segments (e.g., `gitlab.com/namespace/subgroup/project`)

Without tests for non-GitHub hosts, the first time a user tries to install a skill from GitLab, it could fail in production. The spec explicitly scopes this as a requirement, but the tests don't verify it — meaning the spec and tests are out of sync.

#### Rebuttal Conditions

This is NOT a concern if: (1) the underlying `ParseDepURL()` function has its own comprehensive tests for multi-host parsing (check `internal/resolve/depurl_test.go`); AND (2) the integration between `ParseDepURL` and `collectSkillFiles`/`Install` is guaranteed to work for any valid host (verify: is there any GitHub-specific logic in the integration path?).

#### Suggested Verification

Add tests with non-GitHub hosts:
1. `TestCollectSkillFiles_GitLabHost`: Use `"gitlab.com/org/repo@v1.0.0"` and verify the composite key is `"gitlab.com/org/repo/skill-name"` (not `"github.com/..."` or mangled)
2. `TestInstallCompositeKeys_BitbucketHost`: Use `"bitbucket.org/user/project@v1.0.0"` and verify files install under `<target>/bitbucket.org/user/project/skill/`
3. `TestRunRemove_SelfHostedGit`: Use `"code.internal.company.com/team/repo@v1.0.0"` and verify cleanup path is `<target>/code.internal.company.com/team/repo/skill/`

These tests should be parallel to the existing GitHub tests, not in a separate "edge case" section — multi-host support is a first-class requirement, not an edge case.

---

### Finding: TestRunRemove_SharedSkillRetained was simplified and may no longer verify the intended behavior

**Severity**: should-fix  
**Confidence**: MEDIUM  
**Category**: testing

#### Grounds (Evidence)

In `internal/cli/remove_test.go:128-188`, the test `TestRunRemove_SharedSkillRetained` was updated for namespaced paths. The test comment at line 180-184 now says:

```go
// shared-skill should be retained (dep-b still provides it)
// Note: with namespacing, shared-skill from dep-a lives under github.com/org/a/
// and dep-b's shared-skill would live under github.com/org/b/ — they're separate paths.
// The orphan check still uses skill NAMES, but the disk paths are namespaced.
// unique-a should be cleaned up since only dep-a provided it
```

The test setup (lines 163-164) creates only **one** skill directory on disk:
```go
_ = os.MkdirAll(filepath.Join(targetDir, "github.com", "org", "a", "shared-skill"), 0755)
_ = os.MkdirAll(filepath.Join(targetDir, "github.com", "org", "a", "unique-a"), 0755)
```

Notice that `dep-b`'s `shared-skill` directory (`github.com/org/b/shared-skill`) is **not created**. The test comment says "shared-skill should be retained (dep-b still provides it)", but there's nothing to retain — `dep-b`'s shared-skill was never installed on disk.

The original test (before the diff) likely had a non-namespaced `shared-skill/` directory that was shared by both deps. After removing `dep-a`, the test verified that the shared directory remained because `dep-b` still needed it. But with namespacing, the two `shared-skill` directories are separate (`a/shared-skill` and `b/shared-skill`), so there's nothing shared to retain.

The test now only verifies that `unique-a` is removed (line 185-187), which is a trivial case — any skill from a removed dep should be deleted. It doesn't test the "shared skill retained" scenario anymore.

#### Warrant (Rule)

The test name is `TestRunRemove_SharedSkillRetained`, but it no longer tests that scenario. With namespacing, same-name skills from different deps occupy different disk paths, so the orphan check logic changed. The test should verify the **new** behavior:

1. **Orphan check uses skill names** (not paths): When `dep-a` is removed, the orphan check looks at all remaining deps and sees if any still export `"shared-skill"`. If `dep-b` exports `"shared-skill"`, then `"shared-skill"` is not orphaned — but since the disk paths are namespaced, `dep-a`'s `shared-skill` directory (`a/shared-skill`) should still be deleted, while `dep-b`'s `shared-skill` directory (`b/shared-skill`) should remain untouched.

2. **The test doesn't verify this**: It creates `dep-a`'s shared-skill but not `dep-b`'s. After removing `dep-a`, there's no `dep-b` directory to check if it was retained or incorrectly deleted.

The test needs to be updated to reflect the new namespaced behavior, or renamed to reflect what it actually tests (e.g., `TestRunRemove_UniqueSkillDeleted`).

#### Rebuttal Conditions

This is NOT a concern if: (1) the test's original intent was to verify that the removal logic doesn't crash when multiple deps share a skill name, and the current test still verifies that (no crash, correct cleanup of removed dep); or (2) the orphan check is name-based but the cleanup is path-based, and the test correctly verifies that path-based cleanup works even when names overlap. Check the actual orphan logic in `remove.go` to confirm.

#### Suggested Verification

Option 1: Update the test to verify the full scenario:
```go
// Create both deps' shared-skill directories
_ = os.MkdirAll(filepath.Join(targetDir, "github.com", "org", "a", "shared-skill"), 0755)
_ = os.MkdirAll(filepath.Join(targetDir, "github.com", "org", "b", "shared-skill"), 0755)
_ = os.MkdirAll(filepath.Join(targetDir, "github.com", "org", "a", "unique-a"), 0755)

// After removing dep-a, verify:
// - dep-a's shared-skill is GONE
if _, err := os.Stat(filepath.Join(targetDir, "github.com", "org", "a", "shared-skill")); err == nil {
    t.Error("dep-a's shared-skill should be removed")
}
// - dep-b's shared-skill is RETAINED
if _, err := os.Stat(filepath.Join(targetDir, "github.com", "org", "b", "shared-skill")); err != nil {
    t.Error("dep-b's shared-skill should be retained")
}
// - unique-a is GONE
if _, err := os.Stat(filepath.Join(targetDir, "github.com", "org", "a", "unique-a")); err == nil {
    t.Error("unique-a should be removed")
}
```

Option 2: Rename the test to `TestRunRemove_NamespacedSkillsIndependent` and add a comment explaining that same-name skills from different deps are treated as independent (no shared directory).

---

### Finding: TestCollectSkillFiles_SameNameDifferentDeps only exercises the happy path

**Severity**: consider  
**Confidence**: MEDIUM  
**Category**: testing

#### Grounds (Evidence)

In `internal/cli/install_test.go:249-303`, the new test `TestCollectSkillFiles_SameNameDifferentDeps` verifies the core use case: two dependencies (`github.com/lossyrob/paw` and `github.com/anthropics/skills`) both export a skill named `"skill-creator"`, and both are collected under separate composite keys.

The test verifies:
1. Both skills exist in the result (line 284-286)
2. The composite keys are correct (lines 288-290, 296-298)
3. The file contents are distinct (lines 292-294, 300-302)

This is a good happy-path test, but it doesn't cover realistic variations:
- What if the two deps export the same skill name **and** have identical file contents? (This is the realistic collision case — two repos independently create a `skill-creator` skill with the same structure)
- What if one of the deps' skills has an empty SkillPaths entry? (Root-level skill vs subdirectory skill with the same name)
- What if the two deps are from the same owner but different repos? (`github.com/org/repo-a/skill` vs `github.com/org/repo-b/skill`)

#### Warrant (Rule)

The test verifies that the namespacing mechanism works in principle, but it doesn't stress-test the boundary conditions. The most realistic collision scenario is when two skills have the same name **and** similar or identical content (e.g., both are template generators with similar structure). The test uses different content (`"paw version"` vs `"anthropic version"`), which makes it easy to distinguish them, but that's not the hard case.

The hard case is: same name, same content, different sources. The namespacing should distinguish them by path, but if the content is identical, it's harder to verify that the correct version is installed for each dep. The test doesn't exercise this.

This is a "nice to have" rather than "must fix" because the current test does verify the core mechanism. But if a future bug causes the two skills to overwrite each other (e.g., because the composite key logic is bypassed somewhere), the test might not catch it if the content is identical.

#### Rebuttal Conditions

This is NOT a concern if: (1) the existing test is sufficient to catch all realistic regressions (the distinct content is just for human readability, and the composite key assertions would fail regardless of content); or (2) there are other tests that exercise identical-content scenarios (check: do any tests use the same skill files across multiple deps?).

#### Suggested Verification

Add a test variant: `TestCollectSkillFiles_SameNameIdenticalContent`. Set up:
- Two deps both export `"formatter"` with identical `SKILL.md` content: `[]byte("---\nname: formatter\n---\n")`
- Both use the same skill path: `"skills/formatter"`

Assert:
- Both composite keys exist: `"github.com/org/a/formatter"` and `"github.com/org/b/formatter"`
- Both have the same content (verifying that identical content doesn't cause a merge or deduplication)
- The skill map has exactly 2 entries (not 1, which would indicate incorrect deduplication)

This test verifies that the namespacing is based on **source identity**, not content hash.

---

### Finding: Path traversal security tests only cover installer_test.go, not the new namespaced paths in remove.go

**Severity**: consider  
**Confidence**: MEDIUM  
**Category**: testing

#### Grounds (Evidence)

In `internal/install/installer_test.go:62-122`, there are comprehensive path traversal security tests:
- `TestInstallRejectsTraversalSkillName` (line 62): Rejects `"../../etc/malicious"` as a skill name
- `TestInstallRejectsTraversalFilePath` (line 78): Rejects `"../../etc/passwd"` as a file path
- `TestInstallRejectsDotSkillName` (line 110): Rejects `"."` as a skill name
- `TestInstallRejectsEmptySkillName` (line 123): Rejects `""` as a skill name

These tests verify that the `Install()` function has path traversal protections. However, the new `remove.go` logic (lines 238-243, diff) constructs paths with the namespace prefix:

```go
if nsPrefix != "" {
    skillDir = filepath.Join(tp, nsPrefix, skillName)
} else {
    skillDir = filepath.Join(tp, skillName)
}
```

The comment at line 212 (diff line 216) says "Path traversal protection", but there's no **test** verifying that this protection works with the namespaced paths. For example:
- What if `nsPrefix` contains `../../` (from a malformed URL)?
- What if `skillName` contains `../../` and is joined with `nsPrefix`?
- What if the combined `nsPrefix + skillName` escapes the target directory?

The existing installer tests don't cover the removal path, and the remove tests don't have any path traversal cases.

#### Warrant (Rule)

Path traversal vulnerabilities are a high-severity security risk. The removal logic constructs paths dynamically from `nsPrefix` (derived from `ParseDepURL`) and `skillName` (from the pinfile). If either of these inputs can be controlled by an attacker (e.g., via a crafted pinfile or malicious dependency), and the path traversal checks fail, an attacker could delete arbitrary directories on the user's system.

The existing path traversal protections in `installer.go` (lines 246-261 of the file, not in the diff) use `filepath.Abs` and `strings.HasPrefix` to verify that the final path is within the target directory. The removal logic at line 246-261 of `remove.go` (diff lines 246-261) has **identical checks**:

```go
absSkillDir, err := filepath.Abs(skillDir)
if err != nil {
    continue
}
absTP, err := filepath.Abs(tp)
if err != nil {
    continue
}
if !strings.HasPrefix(absSkillDir, absTP+string(filepath.Separator)) {
    fmt.Fprintf(cmd.ErrOrStderr(), "Skipping %s (path escapes target)\n", skillName)
    continue
}
```

This is good defense-in-depth, but there's no **test** verifying it works. If a future refactor removes or weakens these checks (e.g., changes `HasPrefix` to `Contains`), there's no test that would fail.

#### Rebuttal Conditions

This is NOT a concern if: (1) `nsPrefix` is guaranteed to never contain `..` or other traversal sequences because `ParseDepURL` validates and normalizes it (check the `ParseDepURL` implementation and tests); AND (2) `skillName` is guaranteed to never contain traversal sequences because the manifest/pinfile validation rejects them (check the validation logic). If both inputs are validated upstream, the remove.go checks are defense-in-depth and don't need dedicated tests.

#### Suggested Verification

Add test: `TestRunRemove_RejectsTraversalInNamespacedPath`. Set up:
- Create a pinfile with a crafted entry: `"github.com/../../etc/passwd@v1.0.0"` (malicious nsPrefix)
- Or create a pinfile with a skill name containing traversal: `"../../../etc/shadow"`

Attempt to run `remove` for that dependency. Assert:
- The command does not crash or panic
- No files outside the target directory are deleted
- A warning message is printed: `"Skipping ... (path escapes target)"`

This test verifies that the defense-in-depth checks in `remove.go` actually work. If they do, the test passes. If they're bypassed or broken, the test fails (or worse, deletes the wrong files).

---

### Finding: No test verifies that cleanEmptyParents stops before deleting the target root

**Severity**: must-fix  
**Confidence**: HIGH  
**Category**: testing

#### Grounds (Evidence)

This is a specific boundary case for `cleanEmptyParents()` that deserves its own finding because it's the **most dangerous** failure mode. In `remove.go:265-282`, the loop condition is:

```go
if err != nil || absDir == absRoot || !strings.HasPrefix(absDir, absRoot+string(filepath.Separator)) {
    break
}
```

The check `absDir == absRoot` is supposed to stop the loop before deleting the root itself. However, this check is **implicit** — it's not obvious from the code whether it actually works. The test at `TestRunRemove_ExistingDep` (line 87-89) verifies that `github.com/org/repo` is cleaned up, but it doesn't explicitly verify that the target directory (`installed/`) itself is **not** deleted.

Consider the scenario:
- Target root: `/home/user/.copilot/skills`
- Skill path: `/home/user/.copilot/skills/github.com/org/repo/skill`
- After removing the skill, `cleanEmptyParents` walks up: `repo/` → `org/` → `github.com/` → `skills/`

The question: does it stop at `skills/` (correct), or does it try to delete `skills/` (incorrect)? The code says it should stop because of the `absDir == absRoot` check, but there's no test proving this works.

If the check is wrong (e.g., off-by-one), the function could delete the user's entire skill directory, causing data loss for all other skills.

#### Warrant (Rule)

Boundary condition: the loop termination condition for recursive algorithms is the most common source of off-by-one errors. The classic mistake is `<` vs `<=`, or `==` vs `!=`. In this case, the check is `absDir == absRoot`, which means "stop when we reach root." But what if:

1. The check is **before** the delete, so it stops at root (correct) ✓
2. The check is **after** the delete, so it deletes root and then stops (incorrect) ✗

Looking at the code (lines 276-279):
```go
if err := os.Remove(dir); err != nil {
    break // not empty or permission error
}
dir = filepath.Dir(dir)
```

The delete happens first, **then** `dir` is updated to the parent. On the next iteration, the check `absDir == absRoot` is evaluated. So the sequence is:
1. Delete `github.com/` (suppose this was the last dir to delete)
2. Update `dir` to `skills/` (the parent, which is root)
3. Next iteration: check `absDir == absRoot` → true → break

So it **should not** delete root. But this is subtle and not obvious from a code review. A test would make this explicit.

#### Rebuttal Conditions

This is NOT a concern if: (1) the integration test at `TestRunRemove_ExistingDep` explicitly asserts that the target directory still exists after cleanup (check line 82-89: does it verify that `targetDir` exists?); or (2) there's a separate unit test for `cleanEmptyParents` that covers this case (there isn't, per Finding 1).

#### Suggested Verification

Add test: `TestCleanEmptyParents_StopsAtRoot`. Set up:
```go
root := t.TempDir()
nestedDir := filepath.Join(root, "a", "b", "c")
os.MkdirAll(nestedDir, 0755)

cleanEmptyParents(root, nestedDir)

// Verify nested dirs are gone
if _, err := os.Stat(nestedDir); err == nil {
    t.Error("nested dir should be removed")
}
// Verify root still exists
if _, err := os.Stat(root); err != nil {
    t.Error("root should NOT be removed")
}
```

This test explicitly verifies the boundary condition: `cleanEmptyParents` removes all empty parents up to (but not including) root.

---

## Summary of High-Priority Actions

1. **Add unit tests for `cleanEmptyParents()`** covering all boundary conditions (root boundary, non-empty parents, error handling)
2. **Add test for `verifyIntegrity()` ParseDepURL error path** to verify behavior when URL parsing fails
3. **Add tests for non-GitHub hosts** (GitLab, Bitbucket, self-hosted) per spec requirement SC-005
4. **Update or clarify `TestRunRemove_SharedSkillRetained`** to verify namespaced behavior correctly
5. **Add explicit test for cleanEmptyParents root boundary** to prevent deletion of target root

## Test Quality Assessment

**Strengths**:
- Existing tests were correctly updated for namespaced paths (all composite keys use `"github.com/org/repo/skill"` format)
- Good coverage of atomic install and path traversal security in `installer_test.go`
- New test `TestCollectSkillFiles_SameNameDifferentDeps` directly exercises the core use case

**Weaknesses**:
- **No unit tests** for new `cleanEmptyParents()` function (recursive deletion is high-risk)
- **Error paths are under-tested**: ParseDepURL failure in `verifyIntegrity()`, non-GitHub hosts
- **Boundary conditions are implicit**: Root boundary for cleanup, empty parent handling
- **One test was simplified** (`TestRunRemove_SharedSkillRetained`) and may no longer verify the intended behavior

**Risk Assessment**:
The highest risk is in the `cleanEmptyParents()` function — it's new, untested, and performs recursive directory deletion. A bug here could cause data loss (deleting too much) or directory pollution (not cleaning up empty dirs). The second-highest risk is the silent `continue` in `verifyIntegrity()` when ParseDepURL fails — this could skip integrity checks for malformed deps, opening a security hole.

