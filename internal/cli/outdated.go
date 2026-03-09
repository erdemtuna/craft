package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/erdemtuna/craft/internal/fetch"
	"github.com/erdemtuna/craft/internal/resolve"
	"github.com/erdemtuna/craft/internal/semver"
	"github.com/spf13/cobra"
)

var outdatedCmd = &cobra.Command{
	Use:   "outdated",
	Short: "Show available dependency updates",
	Long:  "Check each direct dependency for newer versions and classify updates as major, minor, or patch.",
	Args:  cobra.NoArgs,
	RunE:  runOutdated,
}

func runOutdated(cmd *cobra.Command, args []string) error {
	m, pf, err := requireManifestAndPinfileForScope()
	if err != nil {
		return err
	}

	if len(m.Dependencies) == 0 {
		cmd.Println("No dependencies to check.")
		return nil
	}

	verboseLog(cmd, "Checking %d direct dependencies for updates...", len(m.Dependencies))

	fetcher, err := newFetcher()
	if err != nil {
		return err
	}

	// Build pinfile identity-to-key lookup for finding current version
	pinKeyByIdentity := make(map[string]string)
	for key := range pf.Resolved {
		parsed, err := resolve.ParseDepURL(key)
		if err != nil {
			continue
		}
		pinKeyByIdentity[parsed.PackageIdentity()] = key
	}

	type depStatus struct {
		alias      string
		current    string
		latest     string
		updateType string
		err        error
		warning    string
		skipped    bool
	}

	var results []depStatus

	// Sort dependency aliases for deterministic iteration order and verbose output
	aliases := make([]string, 0, len(m.Dependencies))
	for alias := range m.Dependencies {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	for _, alias := range aliases {
		depURL := m.Dependencies[alias]
		parsed, err := resolve.ParseDepURL(depURL)
		if err != nil {
			results = append(results, depStatus{alias: alias, err: fmt.Errorf("invalid URL: %w", err)})
			continue
		}

		// Non-tag deps can't be checked for semver updates
		switch parsed.RefType {
		case resolve.RefTypeCommit:
			results = append(results, depStatus{
				alias:   alias,
				current: parsed.Ref[:min(12, len(parsed.Ref))],
				skipped: true,
			})
			verboseLog(cmd, "Skipping %s: commit-pinned dependencies are frozen", alias)
			continue
		case resolve.RefTypeBranch:
			results = append(results, depStatus{
				alias:   alias,
				current: "branch:" + parsed.Ref,
				skipped: true,
			})
			verboseLog(cmd, "Skipping %s: branch-tracked dependencies use 'craft update' instead", alias)
			continue
		}
		// Only tag deps reach here

		// Find current version from pinfile
		currentVersion := parsed.Version
		if pinKey, ok := pinKeyByIdentity[parsed.PackageIdentity()]; ok {
			if pinParsed, err := resolve.ParseDepURL(pinKey); err == nil {
				currentVersion = pinParsed.Version
			}
		} else {
			verboseLog(cmd, "No pinfile entry for %s, using declared version v%s", alias, currentVersion)
		}

		cloneURL := fetch.NormalizeCloneURL(parsed.PackageIdentity())
		verboseLog(cmd, "Fetching tags from %s...", cloneURL)

		tags, err := fetcher.ListTags(cloneURL)
		if err != nil {
			results = append(results, depStatus{
				alias:   alias,
				current: currentVersion,
				err:     fmt.Errorf("failed to list tags: %w", err),
			})
			continue
		}

		verboseLog(cmd, "Found %d tags for %s", len(tags), alias)

		latest := semver.FindLatest(tags)
		if latest == "" {
			results = append(results, depStatus{
				alias:   alias,
				current: currentVersion,
				warning: "no semver tags found",
				skipped: true,
			})
			continue
		}

		latestVersion := strings.TrimPrefix(latest, "v")

		// Check if pinned version tag exists in remote tags
		pinnedTag := "v" + currentVersion
		tagExists := false
		for _, t := range tags {
			if t == pinnedTag {
				tagExists = true
				break
			}
		}

		var warning string
		if !tagExists {
			warning = fmt.Sprintf("pinned version %s not found in remote tags", pinnedTag)
		}

		if semver.Compare(currentVersion, latestVersion) < 0 {
			updateType := classifyUpdate(currentVersion, latestVersion)
			results = append(results, depStatus{
				alias:      alias,
				current:    currentVersion,
				latest:     latestVersion,
				updateType: updateType,
				warning:    warning,
			})
		} else {
			results = append(results, depStatus{
				alias:   alias,
				current: currentVersion,
				warning: warning,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].alias < results[j].alias
	})

	// Print results
	hasUpdates := false
	hasErrors := false
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)

	for _, r := range results {
		if r.err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %s: %v\n", sanitize(r.alias), r.err)
			hasErrors = true
			continue
		}
		if r.warning != "" {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %s\n", sanitize(r.alias), r.warning)
		}
		if r.skipped {
			continue
		}
		if r.latest != "" {
			_, _ = fmt.Fprintf(w, "%s\tv%s → v%s\t(%s)\n", sanitize(r.alias), r.current, r.latest, r.updateType)
			hasUpdates = true
		} else {
			_, _ = fmt.Fprintf(w, "%s\tv%s\t(up to date)\n", sanitize(r.alias), r.current)
		}
	}
	_ = w.Flush()

	if hasUpdates || hasErrors {
		return &silentExitError{code: 1}
	}
	return nil
}

// classifyUpdate determines if an update is major, minor, or patch.
func classifyUpdate(current, latest string) string {
	c := semver.ParseParts(current)
	l := semver.ParseParts(latest)
	if l[0] != c[0] {
		return "major"
	}
	if l[1] != c[1] {
		return "minor"
	}
	return "patch"
}
