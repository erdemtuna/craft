package manifest

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// fieldOrder defines the canonical ordering of top-level manifest fields
// for consistent serialization output.
var fieldOrder = []string{
	"schema_version",
	"name",
	"version",
	"description",
	"license",
	"skills",
	"dependencies",
	"metadata",
}

// Write serializes a Manifest to the given writer in YAML format
// with consistent field ordering.
func Write(m *Manifest, w io.Writer) error {
	// Build an ordered YAML document using yaml.v3 nodes for field ordering.
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	mapping := &yaml.Node{Kind: yaml.MappingNode}

	addField(mapping, "schema_version", m.SchemaVersion)
	addField(mapping, "name", m.Name)
	addField(mapping, "version", m.Version)

	if m.Description != "" {
		addField(mapping, "description", m.Description)
	}
	if m.License != "" {
		addField(mapping, "license", m.License)
	}

	addStringSlice(mapping, "skills", m.Skills)

	if len(m.Dependencies) > 0 {
		addStringMap(mapping, "dependencies", m.Dependencies)
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

// addStringMap adds a map[string]string as a mapping node.
func addStringMap(mapping *yaml.Node, key string, m map[string]string) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	mapNode := &yaml.Node{Kind: yaml.MappingNode}

	for k, v := range m {
		mapNode.Content = append(mapNode.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: k,
		}, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: v,
		})
	}

	mapping.Content = append(mapping.Content, keyNode, mapNode)
}
