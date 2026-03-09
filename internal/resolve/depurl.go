// Package resolve implements dependency resolution for craft packages.
package resolve

import (
	"fmt"
	"regexp"
	"strings"
)

// RefType identifies the kind of dependency reference.
type RefType string

const (
	// RefTypeTag is a semver tag reference (e.g., v1.0.0).
	RefTypeTag RefType = "tag"

	// RefTypeCommit is a commit SHA reference (e.g., abc1234def).
	RefTypeCommit RefType = "commit"

	// RefTypeBranch is a branch name reference (e.g., main).
	RefTypeBranch RefType = "branch"
)

// hostOrgRepoPattern matches the host/org/repo portion of a dependency URL.
var hostOrgRepoPattern = regexp.MustCompile(`^([a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?)/([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)$`)

// semverPattern matches strict MAJOR.MINOR.PATCH after a 'v' prefix.
var semverRefPattern = regexp.MustCompile(`^v((?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*))$`)

// hexPattern matches hexadecimal strings (for commit SHA detection).
var hexPattern = regexp.MustCompile(`^[0-9a-fA-F]+$`)

// minCommitSHALength is the minimum length for a commit SHA ref.
const minCommitSHALength = 7

// maxCommitSHALength is the maximum length for a commit SHA ref (SHA-256).
const maxCommitSHALength = 64

// DepURL represents a parsed dependency URL from craft.yaml.
type DepURL struct {
	// Raw is the original URL string (e.g., "github.com/example/skills@v1.0.0").
	Raw string

	// Host is the registry host (e.g., "github.com").
	Host string

	// Org is the organization or user name (e.g., "example").
	Org string

	// Repo is the repository name (e.g., "skills").
	Repo string

	// Version is the semver version without the 'v' prefix (e.g., "1.0.0").
	// Only populated when RefType is RefTypeTag.
	Version string

	// Ref is the raw reference value: commit SHA or branch name.
	// For tags, this is empty (use Version instead).
	Ref string

	// RefType identifies the kind of reference (tag, commit, branch).
	RefType RefType
}

// ParseDepURL parses a dependency URL string into its components.
// Accepts three ref formats:
//   - host/org/repo@vMAJOR.MINOR.PATCH  (tag)
//   - host/org/repo@<hex7+>             (commit SHA)
//   - host/org/repo@branch:<name>       (branch)
//
// Returns an error if the URL does not match any expected format.
func ParseDepURL(raw string) (*DepURL, error) {
	atIdx := strings.Index(raw, "@")
	if atIdx < 0 {
		return nil, fmt.Errorf("invalid dependency URL %q: missing '@' — expected host/org/repo@<ref> where ref is vX.Y.Z, a commit SHA, or branch:<name>", raw)
	}

	identity := raw[:atIdx]
	ref := raw[atIdx+1:]

	if ref == "" {
		return nil, fmt.Errorf("invalid dependency URL %q: empty ref after '@' — expected vX.Y.Z, a commit SHA (≥7 hex chars), or branch:<name>", raw)
	}

	matches := hostOrgRepoPattern.FindStringSubmatch(identity)
	if matches == nil {
		return nil, fmt.Errorf("invalid dependency URL %q: expected host/org/repo@<ref>", raw)
	}

	d := &DepURL{
		Raw:  raw,
		Host: matches[1],
		Org:  matches[2],
		Repo: matches[3],
	}

	if strings.HasPrefix(ref, "branch:") {
		branchName := ref[len("branch:"):]
		if branchName == "" {
			return nil, fmt.Errorf("invalid dependency URL %q: empty branch name after 'branch:'", raw)
		}
		d.Ref = branchName
		d.RefType = RefTypeBranch
	} else if m := semverRefPattern.FindStringSubmatch(ref); m != nil {
		d.Version = m[1]
		d.RefType = RefTypeTag
	} else if hexPattern.MatchString(ref) && len(ref) >= minCommitSHALength && len(ref) <= maxCommitSHALength {
		d.Ref = strings.ToLower(ref)
		d.RefType = RefTypeCommit
	} else {
		return nil, fmt.Errorf("invalid dependency URL %q: ref %q is not a valid semver tag (vX.Y.Z), commit SHA (≥7 hex chars), or branch (branch:<name>)", raw, ref)
	}

	return d, nil
}

// PackageIdentity returns the version-independent package identifier
// (host/org/repo). Used by MVS to identify the same package at different
// versions.
func (d *DepURL) PackageIdentity() string {
	return d.Host + "/" + d.Org + "/" + d.Repo
}

// GitRef returns the ref string to pass to fetcher.ResolveRef().
// For tags: "v1.0.0", for commits: the SHA, for branches: the branch name.
func (d *DepURL) GitRef() string {
	switch d.RefType {
	case RefTypeTag:
		return "v" + d.Version
	case RefTypeCommit:
		return d.Ref
	case RefTypeBranch:
		return d.Ref
	default:
		return "v" + d.Version
	}
}

// RefString returns the ref portion as it appears in the URL after '@'.
// For tags: "v1.0.0", for commits: the SHA, for branches: "branch:<name>".
func (d *DepURL) RefString() string {
	switch d.RefType {
	case RefTypeTag:
		return "v" + d.Version
	case RefTypeCommit:
		return d.Ref
	case RefTypeBranch:
		return "branch:" + d.Ref
	default:
		return "v" + d.Version
	}
}

// HTTPSURL returns the HTTPS clone URL (e.g., "https://github.com/example/skills.git").
func (d *DepURL) HTTPSURL() string {
	return "https://" + d.Host + "/" + d.Org + "/" + d.Repo + ".git"
}

// SSHURL returns the SSH clone URL (e.g., "git@github.com:example/skills.git").
func (d *DepURL) SSHURL() string {
	return "git@" + d.Host + ":" + d.Org + "/" + d.Repo + ".git"
}

// WithVersion returns a new dep URL string with the given version
// (e.g., "github.com/example/skills@v2.0.0").
// Only valid for tag-type dependencies.
func (d *DepURL) WithVersion(version string) string {
	version = strings.TrimPrefix(version, "v")
	return d.PackageIdentity() + "@v" + version
}

// String returns the raw dependency URL, or reconstructs it from components.
func (d *DepURL) String() string {
	if d.Raw != "" {
		return d.Raw
	}
	return d.PackageIdentity() + "@" + d.RefString()
}
