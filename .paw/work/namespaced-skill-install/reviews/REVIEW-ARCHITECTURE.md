# Architecture Review: Namespaced Skill Installation

**Reviewer**: Architecture Specialist  
**Date**: 2026-03-09  
**Target**: Namespaced skill install feature diff  

## Summary

This review examines the architectural implications of introducing composite keys for skill namespacing. The changes eliminate cross-dependency skill name collisions by embedding namespace prefixes (`host/owner/repo`) directly in map keys, leveraging `filepath.Join` for directory creation. While functionally sound, the design introduces several architectural concerns around abstraction leakage and responsibility boundaries.

## Findings

### Finding: Composite key approach leaks path structure into business logic domains

**Severity**: should-fix  
**Confidence**: HIGH  
**Category**: architecture  

#### Grounds (Evidence)

In `/tmp/review-diff.txt:29`, `collectSkillFiles()` creates composite keys using `compositeKey := prefix + "/" + skillName` where `prefix` is `parsed.PackageIdentity()`. These keys like `"github.com/org/repo/skillname"` are then used throughout the system:
- In test expectations at line 74: `skills["github.com/org/repo/lint"]`
- In `verifyIntegrity()` at line 320: `compositeKey := prefix + "/" + skillName`
- In `Install()` function signature documentation at line 372: "composite key (host/owner/repo/skill-name)"

The original codebase uses simple skill names as keys (`skills["lint"]`) in business logic, while filesystem paths are handled by the `Install()` function. The new design embeds filesystem structure (`/` separators, path ordering) directly in the business logic map keys.

#### Warrant (Rule)

This violates the abstraction principle that business logic should be independent of storage implementation details. The existing pattern keeps skill identification (business concern) separate from skill installation paths (storage concern). The composite key approach couples these domains by encoding filesystem path structure in business identifiers.

When a map key contains path separators and relies on `filepath.Join` behavior for its meaning, the map has become a filesystem abstraction rather than a skill registry. Business logic that processes skills now must understand that keys are actually path fragments. This makes the code harder to reason about and creates implicit dependencies on filesystem path conventions.

The existing codebase demonstrates a cleaner separation: `collectSkillFiles()` builds a skill registry (skill name → files), and `Install()` handles path construction. The new design conflates these concerns.

#### Rebuttal Conditions

This is NOT a concern if: (1) the composite key serves purposes beyond filesystem organization (e.g., skill lookup by package identity); or (2) the `Install()` function signature was already designed to accept path-like keys (examine the original design intent); or (3) there's a documented architectural decision to merge skill identity with installation paths for simplicity.

#### Suggested Verification

Consider introducing a dedicated `SkillRef` or `NamespacedSkillName` type that encapsulates the composite key logic and provides methods like `String()`, `InstallPath()`, and `SkillName()`. This would preserve the namespace functionality while maintaining abstraction boundaries.

---

### Finding: Removal of detectCollisions leaves architectural gap in dependency validation

**Severity**: consider  
**Confidence**: MEDIUM  
**Category**: architecture  

#### Grounds (Evidence)

In `/tmp/review-diff.txt:443-468`, the `detectCollisions()` function is completely removed from `resolver.go`. The original function checked for duplicate skill names across resolved dependencies and returned clear error messages like `"skill name collision: %q is exported by both %s (commit %s) and %s (commit %s)"`. 

The test at line 481 shows the original expectation: `TestResolveCollision` expected collision errors, but is renamed to `TestResolveSameNameSkillsAllowed` and now expects success. No replacement validation logic appears in the diff.

#### Warrant (Rule)

The resolver's responsibility includes dependency graph validation and conflict detection. While skill name conflicts are now resolved through namespacing, the complete removal of collision detection eliminates an entire class of validation that was providing valuable feedback to users.

The original collision detection served a user experience purpose: it informed users when their dependency choices created naming conflicts and required resolution. With namespacing, users may not realize they have two skills with identical names until they attempt to use them, creating a delayed discovery problem.

However, this is a borderline concern because the collision detection may have been purely prohibitive (blocking valid use cases) rather than informational. The architectural question is whether some form of conflict awareness should be preserved, even if it doesn't block resolution.

#### Rebuttal Conditions

This is NOT a concern if: (1) user tooling will provide skill discovery that makes name conflicts obvious; or (2) the collision detection was intended to be temporary until namespacing was implemented; or (3) the pinfile or installation logs provide sufficient visibility into installed skills that naming conflicts are readily apparent to users.

#### Suggested Verification

Check if there's a deliberate architectural decision to remove all collision detection, or if warnings (rather than errors) should be preserved to inform users about naming conflicts without blocking installation.

---

### Finding: Install function contract ambiguity around composite key directory creation

**Severity**: consider  
**Confidence**: MEDIUM  
**Category**: architecture  

#### Grounds (Evidence)

In `/tmp/review-diff.txt:372-377`, the `Install()` function documentation states: "Each entry in skills maps a composite key (host/owner/repo/skill-name) to a map of relative file paths to contents. The composite key naturally creates nested directories via filepath.Join."

The actual implementation in `/home/erdemtuna/workspace/personal/craft/internal/install/installer.go:28` shows: `skillDir := filepath.Join(target, skillName)` where `skillName` is now the composite key. This "naturally creates nested directories" through string concatenation rather than explicit directory structure management.

The original function operated on simple skill names, with clear expectations about creating `<target>/<skill-name>/` directories. The new contract implies that any slash-containing string passed as a skill name will create nested directories.

#### Warrant (Rule)

Function contracts should be explicit about their behavior, especially when they change from simple to complex directory creation. The "naturally creates" phrasing suggests implicit behavior that callers must understand rather than explicit interface guarantees.

The architectural concern is that `Install()` now implicitly supports arbitrary directory nesting based on input string content, but this capability isn't reflected in its signature or error handling. A caller could pass `"malicious/../../escape"` as a skill name and rely on existing path traversal checks rather than explicit directory depth validation.

This is not necessarily a security issue (path traversal checks exist), but it represents an implicit contract change that could be made more explicit.

#### Rebuttal Conditions

This is NOT a concern if: (1) the `Install()` function was always designed to handle path-like skill names (check git history); or (2) the path traversal validation is sufficient to handle all edge cases of nested directory creation; or (3) the composite key format is guaranteed to be safe by the caller (e.g., always from `ParseDepURL`).

#### Suggested Verification

Consider making the directory nesting behavior explicit in the function signature (e.g., `InstallNested()`) or adding validation that composite keys conform to expected patterns. Review whether the path traversal checks are sufficient for the new nesting depth.

---

### Finding: Responsibility split between collectSkillFiles and Install creates coupling through composite keys

**Severity**: consider  
**Confidence**: HIGH  
**Category**: architecture  

#### Grounds (Evidence)

In `/tmp/review-diff.txt:258` and `/tmp/review-diff.txt:288`, `collectSkillFiles()` constructs composite keys using `prefix := parsed.PackageIdentity()` and `compositeKey := prefix + "/" + skillName`. The `Install()` function at line 28 in `/home/erdemtuna/workspace/personal/craft/internal/install/installer.go` receives these keys and uses `filepath.Join(target, skillName)` to create the directory structure.

This creates an implicit coupling: `collectSkillFiles()` must construct keys in exactly the format that `Install()` expects for directory creation. The two functions are in different packages (`internal/cli` and `internal/install`) but are now coupled through the string format of composite keys.

In the original design, `collectSkillFiles()` returned simple skill names, and `Install()` handled all path construction locally. The new design splits path construction responsibility across package boundaries.

#### Warrant (Rule)

Module boundaries should minimize coupling between packages. When one package must format strings in a specific way for another package to interpret them as filesystem paths, the packages are implicitly coupled through string format conventions.

This coupling is fragile because changes to the composite key format in `collectSkillFiles()` could break directory creation in `Install()` without any compile-time safety. The two packages must maintain agreement on the string format but have no enforced interface for this coordination.

The existing codebase follows a cleaner pattern where `Install()` is responsible for all path construction decisions, receiving only business-level identifiers (skill names) from callers.

#### Rebuttal Conditions

This is NOT a concern if: (1) the composite key format is standardized and unlikely to change (e.g., mirrors Go module paths exactly); or (2) there are tests that enforce the contract between the two functions; or (3) the coupling is accepted as a reasonable trade-off for implementation simplicity.

#### Suggested Verification

Examine test coverage for the interaction between `collectSkillFiles()` and `Install()` with various composite key formats. Consider whether the composite key format should be defined in a shared package or enforced through type safety.

---

## Recommendations

1. **Consider introducing a NamespacedSkillID type** to encapsulate composite key logic and provide explicit methods for different representations (business identifier vs. filesystem path).

2. **Document the architectural decision** to merge skill identification with installation paths, explaining why the abstraction trade-off was chosen.

3. **Add integration tests** that verify the contract between `collectSkillFiles()` and `Install()` across edge cases like non-GitHub hosts and deeply nested paths.

4. **Evaluate whether collision warnings** (not errors) should be preserved to maintain user awareness of naming conflicts.

## Overall Assessment

The namespaced installation feature successfully addresses the core requirement of eliminating skill name collisions. The composite key approach is pragmatic and leverages existing filesystem primitives effectively. However, the design introduces abstraction leakage that couples business logic with storage concerns. While not functionally problematic, this represents a departure from the cleaner separation of concerns in the original architecture.

The changes are implementation-ready but would benefit from more explicit interfaces and stronger abstraction boundaries to maintain long-term maintainability.