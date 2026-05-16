package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Snippet represents a reusable SQL fragment stored in a YAML file.
type Snippet struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	SQL         string `yaml:"sql"`
}

// snippetNameRegex validates snippet names: alphanumeric, underscores, hyphens only.
var snippetNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// snippetRefRegex matches {{snippet:name}} patterns in SQL.
var snippetRefRegex = regexp.MustCompile(`\{\{snippet:([a-zA-Z0-9_-]+)\}\}`)

// LoadSnippet loads and validates a single snippet file.
func LoadSnippet(path string) (*Snippet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read snippet file %s: %w", path, err)
	}

	var snippet Snippet
	if err := yaml.Unmarshal(data, &snippet); err != nil {
		return nil, fmt.Errorf("failed to parse snippet YAML %s: %w", path, err)
	}

	// Validate required fields
	if snippet.Name == "" {
		return nil, fmt.Errorf("snippet %s: name is required", path)
	}
	if snippet.SQL == "" {
		return nil, fmt.Errorf("snippet %s: sql is required", path)
	}

	// Validate name matches filename (without .yaml extension)
	expectedName := strings.TrimSuffix(filepath.Base(path), ".yaml")
	if snippet.Name != expectedName {
		return nil, fmt.Errorf("snippet %s: name %q does not match filename %q", path, snippet.Name, expectedName)
	}

	// Validate name format
	if !snippetNameRegex.MatchString(snippet.Name) {
		return nil, fmt.Errorf("snippet %s: invalid name %q (must be alphanumeric with underscores and hyphens)", path, snippet.Name)
	}

	return &snippet, nil
}

// LoadSnippets loads all snippets from the given directory.
// Returns a map of snippet name -> snippet, and logs warnings for invalid snippets.
func LoadSnippets(dir string) (map[string]*Snippet, error) {
	snippets := make(map[string]*Snippet)

	// Read all .yaml files in the directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read snippets directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		snippet, err := LoadSnippet(path)
		if err != nil {
			// Log warning but continue loading other snippets
			fmt.Printf("⚠️  Skipping invalid snippet %s: %v\n", path, err)
			continue
		}

		snippets[snippet.Name] = snippet
	}

	return snippets, nil
}

// ExtractSnippetRefs extracts all snippet references from SQL.
// Returns a slice of snippet names in the order they appear.
func ExtractSnippetRefs(sql string) []string {
	matches := snippetRefRegex.FindAllStringSubmatch(sql, -1)
	refs := make([]string, len(matches))
	for i, match := range matches {
		refs[i] = match[1]
	}
	return refs
}

// ExpandSnippets replaces {{snippet:name}} with snippet SQL in the given SQL string.
// Returns an error if a referenced snippet is not found.
func ExpandSnippets(snippets map[string]*Snippet, sql string) (string, error) {
	// Find all snippet references with indices
	matches := snippetRefRegex.FindAllStringSubmatchIndex(sql, -1)
	if len(matches) == 0 {
		// No snippets to expand, return original SQL
		return sql, nil
	}

	// Expand each reference
	var result strings.Builder
	result.Grow(len(sql))
	lastIndex := 0

	for _, match := range matches {
		// Extract snippet name (group 1)
		snippetName := sql[match[2]:match[3]]

		// Write the part before this match
		result.WriteString(sql[lastIndex:match[0]])

		// Look up the snippet
		snippet, ok := snippets[snippetName]
		if !ok {
			return "", fmt.Errorf("snippet %q not found in snippets directory", snippetName)
		}

		// Write the snippet SQL
		result.WriteString(snippet.SQL)

		// Update last index to after this match
		lastIndex = match[1]
	}

	// Write the remaining part after the last match
	result.WriteString(sql[lastIndex:])

	return result.String(), nil
}
