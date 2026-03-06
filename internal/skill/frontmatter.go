package skill

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseFrontmatter extracts and parses YAML frontmatter from a SKILL.md file.
// Frontmatter must be delimited by "---" markers at the very start of the file.
// Extra fields beyond name/description are captured in Frontmatter.Extra.
func ParseFrontmatter(r io.Reader) (*Frontmatter, error) {
	yamlContent, err := extractFrontmatter(r)
	if err != nil {
		return nil, err
	}

	// First pass: parse into a generic map to capture all fields.
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &raw); err != nil {
		return nil, fmt.Errorf("parsing frontmatter YAML: %w", err)
	}

	// Second pass: parse known fields into struct.
	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return nil, fmt.Errorf("parsing frontmatter fields: %w", err)
	}

	// Capture extra fields for forward compatibility.
	extra := make(map[string]interface{})
	for k, v := range raw {
		if k != "name" && k != "description" {
			extra[k] = v
		}
	}
	if len(extra) > 0 {
		fm.Extra = extra
	}

	return &fm, nil
}

// ParseFrontmatterFile extracts frontmatter from a SKILL.md file at the given path.
func ParseFrontmatterFile(path string) (*Frontmatter, error) {
	f, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return ParseFrontmatter(f)
}

// extractFrontmatter reads the YAML block between --- delimiters at the
// start of the file. Returns the raw YAML content string.
func extractFrontmatter(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)

	// First line must be "---"
	if !scanner.Scan() {
		return "", fmt.Errorf("frontmatter: file is empty")
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return "", fmt.Errorf("frontmatter: file does not start with '---' delimiter")
	}

	// Read until closing "---"
	var lines []string
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			found = true
			break
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("frontmatter: reading file: %w", err)
	}

	if !found {
		return "", fmt.Errorf("frontmatter: missing closing '---' delimiter")
	}

	if len(lines) == 0 {
		return "", fmt.Errorf("frontmatter: empty content between delimiters")
	}

	return strings.Join(lines, "\n"), nil
}

// openFile is a helper for opening files. Uses os.Open directly.
func openFile(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening SKILL.md: %w", err)
	}
	return f, nil
}
