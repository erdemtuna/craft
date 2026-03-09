package cli

import (
	"strings"

	"github.com/erdemtuna/craft/internal/resolve"
	"github.com/erdemtuna/craft/internal/ui"
	"github.com/spf13/cobra"
)

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Print dependency tree",
	Long:  "Display the dependency tree showing local skills and all resolved dependencies with their skills.",
	Args:  cobra.NoArgs,
	RunE:  runTree,
}

func runTree(cmd *cobra.Command, args []string) error {
	m, pf, err := loadManifestAndPinfile()
	if err != nil {
		return err
	}

	verboseLog(cmd, "Loaded manifest: %s@%s", m.Name, m.Version)

	packageName := m.Name
	if m.Version != "" {
		packageName += "@" + m.Version
	}

	// Extract local skill names from paths
	var localSkills []string
	for _, s := range m.Skills {
		parts := strings.Split(strings.TrimRight(s, "/"), "/")
		localSkills = append(localSkills, parts[len(parts)-1])
	}

	// Build alias lookup from manifest
	urlToAlias := make(map[string]string)
	for alias, depURL := range m.Dependencies {
		parsed, err := resolve.ParseDepURL(depURL)
		if err != nil {
			continue
		}
		urlToAlias[parsed.PackageIdentity()] = alias
	}

	// Build dep nodes from pinfile
	var deps []ui.DepNode
	for key, entry := range pf.Resolved {
		parsed, err := resolve.ParseDepURL(key)
		if err != nil {
			continue
		}

		alias := urlToAlias[parsed.PackageIdentity()]
		if alias == "" {
			alias = parsed.Repo
		}

		deps = append(deps, ui.DepNode{
			Alias:  alias,
			URL:    key,
			Skills: entry.Skills,
		})
	}

	ui.RenderTree(cmd.OutOrStdout(), packageName, localSkills, deps)
	return nil
}
