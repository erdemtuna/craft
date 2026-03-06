package pinfile

import (
	"fmt"
	"io"
	"sort"

	"gopkg.in/yaml.v3"
)

// Write serializes a Pinfile to the given writer in YAML format
// with deterministic field ordering: pin_version first, then resolved
// entries sorted by URL key.
func Write(p *Pinfile, w io.Writer) error {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	mapping := &yaml.Node{Kind: yaml.MappingNode}

	// pin_version
	addScalar(mapping, "pin_version", fmt.Sprintf("%d", p.PinVersion), "!!int")

	// resolved (sorted by URL key)
	if len(p.Resolved) > 0 {
		resolvedKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "resolved"}
		resolvedMap := &yaml.Node{Kind: yaml.MappingNode}

		keys := make([]string, 0, len(p.Resolved))
		for k := range p.Resolved {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, url := range keys {
			entry := p.Resolved[url]
			urlNode := &yaml.Node{Kind: yaml.ScalarNode, Value: url}
			entryMap := &yaml.Node{Kind: yaml.MappingNode}

			addScalar(entryMap, "commit", entry.Commit, "")
			addScalar(entryMap, "integrity", entry.Integrity, "")

			if entry.Source != "" {
				addScalar(entryMap, "source", entry.Source, "")
			}

			// skills as sequence
			skillsKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "skills"}
			skillsSeq := &yaml.Node{Kind: yaml.SequenceNode}
			for _, s := range entry.Skills {
				skillsSeq.Content = append(skillsSeq.Content, &yaml.Node{
					Kind:  yaml.ScalarNode,
					Value: s,
				})
			}
			entryMap.Content = append(entryMap.Content, skillsKey, skillsSeq)

			resolvedMap.Content = append(resolvedMap.Content, urlNode, entryMap)
		}

		mapping.Content = append(mapping.Content, resolvedKey, resolvedMap)
	}

	doc.Content = append(doc.Content, mapping)

	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("writing pinfile YAML: %w", err)
	}
	return enc.Close()
}

// addScalar adds a scalar key-value pair to a mapping node.
func addScalar(mapping *yaml.Node, key, value, tag string) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: value}
	if tag != "" {
		valNode.Tag = tag
	}
	mapping.Content = append(mapping.Content, keyNode, valNode)
}
