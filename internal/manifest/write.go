package manifest

import (
	"fmt"
	"io"
	"sort"

	"gopkg.in/yaml.v3"
)

// Write serializes a Manifest to the given writer in YAML format
// with consistent field ordering: schema_version, name, description,
// license, skills, dependencies, metadata.
func Write(m *Manifest, w io.Writer) error {
	// Build an ordered YAML document using yaml.v3 nodes for field ordering.
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	mapping := &yaml.Node{Kind: yaml.MappingNode}

	addField(mapping, "schema_version", m.SchemaVersion)
	addField(mapping, "name", m.Name)

	if m.Description != "" {
		addField(mapping, "description", m.Description)
	}
	if m.License != "" {
		addField(mapping, "license", m.License)
	}

	addStringSlice(mapping, "skills", m.Skills)

	if len(m.Dependencies) > 0 {
		addDependencies(mapping, "dependencies", m.Dependencies)
	}
	if len(m.Metadata) > 0 {
		addStringMap(mapping, "metadata", m.Metadata)
	}

	doc.Content = append(doc.Content, mapping)

	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("writing manifest YAML: %w", err)
	}
	return enc.Close()
}

// addField adds a scalar key-value pair to a mapping node.
func addField(mapping *yaml.Node, key string, value interface{}) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	valNode := &yaml.Node{Kind: yaml.ScalarNode}

	switch v := value.(type) {
	case int:
		valNode.Value = fmt.Sprintf("%d", v)
		valNode.Tag = "!!int"
	case string:
		valNode.Value = v
	default:
		valNode.Value = fmt.Sprintf("%v", v)
	}

	mapping.Content = append(mapping.Content, keyNode, valNode)
}

// addStringSlice adds a string slice as a sequence node.
func addStringSlice(mapping *yaml.Node, key string, values []string) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	seqNode := &yaml.Node{Kind: yaml.SequenceNode}

	for _, v := range values {
		seqNode.Content = append(seqNode.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: v,
		})
	}

	mapping.Content = append(mapping.Content, keyNode, seqNode)
}

// addStringMap adds a map[string]string as a mapping node with sorted keys.
func addStringMap(mapping *yaml.Node, key string, m map[string]string) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	mapNode := &yaml.Node{Kind: yaml.MappingNode}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		mapNode.Content = append(mapNode.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: k,
		}, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: m[k],
		})
	}

	mapping.Content = append(mapping.Content, keyNode, mapNode)
}

// addDependencies adds a map[string]DependencySpec as a mapping node with sorted keys.
// Simple dependencies (no Select) are written as scalar values; structured
// dependencies (with Select) are written as mapping nodes with url and select fields.
func addDependencies(mapping *yaml.Node, key string, deps map[string]DependencySpec) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	mapNode := &yaml.Node{Kind: yaml.MappingNode}

	keys := make([]string, 0, len(deps))
	for k := range deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, alias := range keys {
		aliasNode := &yaml.Node{Kind: yaml.ScalarNode, Value: alias}
		dep := deps[alias]

		if len(dep.Select) == 0 {
			// Write as simple scalar value.
			mapNode.Content = append(mapNode.Content, aliasNode, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: dep.URL,
			})
		} else {
			// Write as structured object with url and select fields.
			objNode := &yaml.Node{Kind: yaml.MappingNode}

			objNode.Content = append(objNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "url"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: dep.URL},
			)

			selKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "select"}
			selSeq := &yaml.Node{Kind: yaml.SequenceNode}
			for _, s := range dep.Select {
				selSeq.Content = append(selSeq.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: s},
				)
			}
			objNode.Content = append(objNode.Content, selKey, selSeq)

			mapNode.Content = append(mapNode.Content, aliasNode, objNode)
		}
	}

	mapping.Content = append(mapping.Content, keyNode, mapNode)
}
