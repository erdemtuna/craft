# Docs: Subpath Skill Selection

## Architecture

The feature spans five components:

1. **DependencySpec type** (`internal/manifest/`) — Dependencies are now `DependencySpec` values, not plain strings. A `DependencySpec` transparently represents either a simple URL string or a structured `{url, select}` object via a custom YAML unmarshaler on `*yaml.Node`. Serialization canonicalizes: empty `select` → simple string.

2. **DepURL `#subpath` parsing** (`internal/resolve/`) — The URL parser extracts an optional `#fragment` after the version/ref component, storing it in a `Subpath` field. The `#` separator aligns with PURL (ECMA-427) fragment syntax.

3. **Resolver filtering** (`internal/resolve/`) — After discovering all skills in a package, the resolver filters against the `Select` paths. Only matched skills are included in integrity computation and installation. Unmatched select paths cause resolution failure with a descriptive error. When multiple aliases reference the same package, their select lists are unioned; an empty select (meaning "all") wins over any specific selection.

4. **Interactive preview in `craft add`** (`internal/cli/`) — When adding a multi-skill package in a TTY, `craft add` fetches available skills and presents a numbered list. Users enter comma-separated skill numbers (e.g. 1,3,5), 'a' for all, or Enter for all. `--all` or non-TTY skips the prompt.

5. **New-skill discovery in `craft update`** (`internal/cli/`) — During update, the resolver compares the full set of available skills against the current selection. Newly available skills are reported as informational messages.

## Key Design Decisions

- **PURL-compatible `#fragment` syntax** — Uses `#` as the subpath separator (not `/` or `@`), aligning with ECMA-427 Section 5.6.7. This keeps future full PURL adoption a smooth migration rather than a redesign.

- **Consumer selection overrides package exports (FR-006)** — `select` filters against all discoverable skills in the repo, not just the package's declared `skills` list. This gives consumers full control.

- **Format canonicalization** — On write, a structured dependency with empty/absent `select` is serialized as a plain string. This keeps manifests clean and backward-compatible.

- **Select merge semantics** — When multiple aliases point to the same package, selections are unioned. If any alias has an empty select (all skills), the merged result is "all" — the most permissive intent wins.

- **No schema version bump** — The structured format is a compatible extension of schema version 1. Older craft versions will fail to parse structured deps with a clear YAML error, which is acceptable for a minor version feature.

## Testing

Test coverage added across the feature:

| Area | Coverage |
|------|----------|
| DepURL parsing | `#subpath` extraction, edge cases (empty fragment, multiple `#`, special chars) |
| DependencySpec YAML | Round-trip marshal/unmarshal for string and structured formats, canonicalization |
| Manifest validation | Reject absolute paths, `..` traversal, accept normalized paths |
| Resolver filtering | Select matching, unmatched path errors, multi-alias merging, empty-select semantics |
| Integration (craft add) | Interactive selection mock, `--all` flag, non-TTY fallback |
| Integration (craft get) | `#subpath` single-skill install |
| Integration (craft update) | New-skill discovery messaging |
