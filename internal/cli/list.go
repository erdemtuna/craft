package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	installlib "github.com/erdemtuna/craft/internal/install"
	"github.com/erdemtuna/craft/internal/resolve"
	"github.com/spf13/cobra"
)

var listDetailed bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List resolved dependencies",
	Long:  "Show all dependencies resolved in craft.pin.yaml with their versions and skill counts.",
	Args:  cobra.NoArgs,
	RunE:  runList,
}

func init() {
	listCmd.Flags().BoolVar(&listDetailed, "detailed", false, "Show extended dependency information including URLs and skill names")
}

func runList(cmd *cobra.Command, args []string) error {
	m, pf, err := requireManifestAndPinfileForScope()
	if err != nil {
		return err
	}

	verboseLog(cmd, "Loaded manifest: %s", m.Name)
	verboseLog(cmd, "Loaded pinfile with %d resolved entries", len(pf.Resolved))

	if len(pf.Resolved) == 0 {
		cmd.Println("No dependencies resolved.")
		return nil
	}

	// Build alias-to-URL lookup from manifest
	urlToAlias := make(map[string]string)
	for alias, depURL := range m.Dependencies {
		parsed, err := resolve.ParseDepURL(depURL)
		if err != nil {
			continue
		}
		urlToAlias[parsed.PackageIdentity()] = alias
	}

	type depInfo struct {
		alias   string
		version string
		url     string
		skills  []string
	}

	var deps []depInfo
	for key, entry := range pf.Resolved {
		parsed, err := resolve.ParseDepURL(key)
		if err != nil {
			verboseLog(cmd, "Skipping unparseable pinfile key: %s", key)
			continue
		}

		alias := urlToAlias[parsed.PackageIdentity()]
		if alias == "" {
			alias = parsed.Repo
		}

		deps = append(deps, depInfo{
			alias:   alias,
			version: parsed.RefString(),
			url:     parsed.PackageIdentity(),
			skills:  entry.Skills,
		})
	}

	sort.Slice(deps, func(i, j int) bool {
		return deps[i].alias < deps[j].alias
	})

	if listDetailed {
		for _, d := range deps {
			cmd.Printf("%s  %s  %s\n", sanitize(d.alias), d.version, sanitize(d.url))
			if len(d.skills) > 0 {
				displaySkills := d.skills
				if globalFlag {
					displaySkills = installlib.QualifySkillNames(d.url, d.skills)
				}
				cmd.Printf("  skills: %s\n", sanitize(strings.Join(displaySkills, ", ")))
			} else {
				cmd.Printf("  skills: (none)\n")
			}
			cmd.Println()
		}
	} else {
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
		for _, d := range deps {
			skillWord := "skills"
			if len(d.skills) == 1 {
				skillWord = "skill"
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t(%d %s)\n", sanitize(d.alias), d.version, len(d.skills), skillWord)
		}
		_ = w.Flush()
	}

	return nil
}
