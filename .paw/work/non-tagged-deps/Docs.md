# Non-Tagged Repository Dependencies — Technical Reference

## Summary

This feature extends craft's dependency system with two new reference types beyond semver tags:

- **Commit SHA** (`@<hex7+>`): Pin a dependency to an exact commit
- **Branch tracking** (`@branch:<name>`): Track a named branch, resolved to HEAD at install time

Tagged dependencies (`@vX.Y.Z`) retain their full rigor unchanged.

## Dependency URL Formats

| Format | Example | RefType |
|--------|---------|---------|
| `host/org/repo@vX.Y.Z` | `github.com/acme/tools@v1.0.0` | tag |
| `host/org/repo@<sha>` | `github.com/acme/tools@abc1234def` | commit |
| `host/org/repo@branch:<name>` | `github.com/acme/tools@branch:main` | branch |

- Commit SHAs must be ≥7 hex characters
- Branch names require the `branch:` prefix to disambiguate from SHAs
- A URL with no ref (no `@`) produces an error

## Resolution Behavior

| RefType | MVS | `craft update` | Pinfile reuse |
|---------|-----|-----------------|---------------|
| tag | semver comparison → highest wins | ListTags → FindLatest | yes |
| commit | same-SHA assertion | skipped (deliberate freeze) | yes |
| branch | same-branch assertion | re-resolve HEAD | no (always fresh) |

### Conflict Detection

When the same package (`host/org/repo`) appears with different ref types (e.g., `@v1.0.0` from one dep and `@branch:main` from another), the resolver raises an error: "conflicting ref types for package X — resolve manually."

## Pinfile Format

Non-tagged entries include a `ref_type` field:

```yaml
github.com/acme/tools@branch:main:
  commit: abc1234def567890abc1234def567890abc1234d
  ref_type: branch
  integrity: sha256-...
  skills:
    - tool-a
```

Legacy pinfiles without `ref_type` are treated as `tag` (backward compatible).

## Warning System

Non-tagged dependencies produce yellow warnings at:

- `craft add`: when adding a non-tagged dependency
- `craft validate`: for each non-tagged dep (direct and transitive)

Warnings are non-blocking and informational.

## Files Modified

| File | Change |
|------|--------|
| `internal/resolve/depurl.go` | RefType enum, extended parsing, GitRef/RefString methods |
| `internal/resolve/types.go` | RefType field on ResolvedDep |
| `internal/resolve/resolver.go` | Ref-type-aware MVS, conflict detection, branch cache bypass |
| `internal/pinfile/types.go` | RefType field on ResolvedEntry |
| `internal/pinfile/parse.go` | Backward-compat defaulting (empty → tag) |
| `internal/pinfile/write.go` | ref_type field in YAML output |
| `internal/manifest/validate.go` | Extended regex for non-tagged URLs |
| `internal/cli/add.go` | Non-tagged warnings, ref-type display |
| `internal/cli/update.go` | Ref-type-specific update behavior |
| `internal/validate/runner.go` | checkNonTaggedDeps warning method |

## Verification

```bash
task test     # go test -race ./...
task lint     # golangci-lint run ./...
task build    # go build
```
