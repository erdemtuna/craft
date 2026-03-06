// Package resolve implements dependency resolution for craft packages.
package resolve

import (
	"fmt"
	"regexp"
	"strings"
)

// depURLPattern matches dependency URL format: host/org/repo@vMAJOR.MINOR.PATCH
// Reused from internal/manifest/validate.go for consistency.
var depURLPattern = regexp.MustCompile(`^([a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?)/([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)@v((0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*))$`)

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
	Version string
}

// ParseDepURL parses a dependency URL string into its components.
// Returns an error if the URL does not match the expected format.
func ParseDepURL(raw string) (*DepURL, error) {
	matches := depURLPattern.FindStringSubmatch(raw)
	if matches == nil {
		return nil, fmt.Errorf("invalid dependency URL %q: expected host/org/repo@vMAJOR.MINOR.PATCH (pre-release versions like -beta.1 are not supported)", raw)
	}

	return &DepURL{
		Raw:     raw,
		Host:    matches[1],
		Org:     matches[2],
		Repo:    matches[3],
		Version: matches[4],
	}, nil
}

// PackageIdentity returns the version-independent package identifier
// (host/org/repo). Used by MVS to identify the same package at different
// versions.
func (d *DepURL) PackageIdentity() string {
	return d.Host + "/" + d.Org + "/" + d.Repo
}

// GitTag returns the git tag reference for this version (e.g., "v1.0.0").
func (d *DepURL) GitTag() string {
	return "v" + d.Version
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
func (d *DepURL) WithVersion(version string) string {
	version = strings.TrimPrefix(version, "v")
	return d.PackageIdentity() + "@v" + version
}

// String returns the raw dependency URL.
func (d *DepURL) String() string {
	return d.Raw
}
