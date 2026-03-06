package resolve

// ResolvedDep holds the fully resolved state of a single dependency.
type ResolvedDep struct {
	// URL is the full dependency URL (e.g., "github.com/example/skills@v1.0.0").
	URL string

	// Alias is the manifest key for this dependency.
	Alias string

	// Commit is the resolved git commit SHA.
	Commit string

	// Integrity is the SHA-256 integrity digest (sha256-<base64>).
	Integrity string

	// Skills lists the discovered skill names in this dependency.
	Skills []string

	// SkillPaths lists the skill directory paths relative to the repo root.
	SkillPaths []string

	// Source is the dependency URL of the package that declared this
	// dependency (empty for direct dependencies, set for transitive).
	Source string
}
