package loader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSnippet_Valid(t *testing.T) {
	// Create a temporary directory
	dir := t.TempDir()
	path := filepath.Join(dir, "test_snippet.yaml")
	
	content := `name: test_snippet
description: "Test snippet"
sql: |
  SELECT * FROM users
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	snippet, err := LoadSnippet(path)
	if err != nil {
		t.Fatalf("LoadSnippet failed: %v", err)
	}

	if snippet.Name != "test_snippet" {
		t.Errorf("expected name 'test_snippet', got '%s'", snippet.Name)
	}
	if snippet.Description != "Test snippet" {
		t.Errorf("expected description 'Test snippet', got '%s'", snippet.Description)
	}
	if snippet.SQL != "SELECT * FROM users\n" {
		t.Errorf("expected SQL 'SELECT * FROM users\\n', got '%s'", snippet.SQL)
	}
}

func TestLoadSnippet_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	
	content := `description: "Test snippet"
sql: |
  SELECT * FROM users
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadSnippet(path)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestLoadSnippet_MissingSQL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	
	content := `name: test
description: "Test snippet"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadSnippet(path)
	if err == nil {
		t.Fatal("expected error for missing sql, got nil")
	}
}

func TestLoadSnippet_NameMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actual_name.yaml")
	
	content := `name: different_name
sql: |
  SELECT * FROM users
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadSnippet(path)
	if err == nil {
		t.Fatal("expected error for name mismatch, got nil")
	}
}

func TestLoadSnippet_InvalidName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid-name with spaces.yaml")
	
	content := `name: invalid-name with spaces
sql: |
  SELECT * FROM users
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadSnippet(path)
	if err == nil {
		t.Fatal("expected error for invalid name, got nil")
	}
}

func TestLoadSnippets_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	
	snippets, err := LoadSnippets(dir)
	if err != nil {
		t.Fatalf("LoadSnippets failed: %v", err)
	}
	
	if len(snippets) != 0 {
		t.Errorf("expected 0 snippets, got %d", len(snippets))
	}
}

func TestLoadSnippets_MultipleSnippets(t *testing.T) {
	dir := t.TempDir()
	
	// Create two valid snippets
	snippet1 := `name: snippet_one
description: "First snippet"
sql: |
  SELECT * FROM table1
`
	snippet2 := `name: snippet_two
description: "Second snippet"
sql: |
  SELECT * FROM table2
`
	if err := os.WriteFile(filepath.Join(dir, "snippet_one.yaml"), []byte(snippet1), 0644); err != nil {
		t.Fatalf("failed to write snippet1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "snippet_two.yaml"), []byte(snippet2), 0644); err != nil {
		t.Fatalf("failed to write snippet2: %v", err)
	}

	snippets, err := LoadSnippets(dir)
	if err != nil {
		t.Fatalf("LoadSnippets failed: %v", err)
	}
	
	if len(snippets) != 2 {
		t.Errorf("expected 2 snippets, got %d", len(snippets))
	}
	if _, ok := snippets["snippet_one"]; !ok {
		t.Error("expected snippet_one to be loaded")
	}
	if _, ok := snippets["snippet_two"]; !ok {
		t.Error("expected snippet_two to be loaded")
	}
}

func TestLoadSnippets_IgnoresNonYAML(t *testing.T) {
	dir := t.TempDir()
	
	// Create a non-YAML file
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write readme: %v", err)
	}

	snippets, err := LoadSnippets(dir)
	if err != nil {
		t.Fatalf("LoadSnippets failed: %v", err)
	}
	
	if len(snippets) != 0 {
		t.Errorf("expected 0 snippets, got %d", len(snippets))
	}
}

func TestExpandSnippets_NoSnippets(t *testing.T) {
	sql := "SELECT * FROM users"
	result, err := ExpandSnippets(nil, sql)
	if err != nil {
		t.Fatalf("ExpandSnippets failed: %v", err)
	}
	if result != sql {
		t.Errorf("expected %q, got %q", sql, result)
	}
}

func TestExpandSnippets_SingleSnippet(t *testing.T) {
	snippets := map[string]*Snippet{
		"test": {
			Name: "test",
			SQL:  "SELECT * FROM table1",
		},
	}
	
	sql := "SELECT * FROM {{snippet:test}}"
	expected := "SELECT * FROM SELECT * FROM table1"
	
	result, err := ExpandSnippets(snippets, sql)
	if err != nil {
		t.Fatalf("ExpandSnippets failed: %v", err)
	}
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExpandSnippets_MultipleSnippets(t *testing.T) {
	snippets := map[string]*Snippet{
		"filter": {
			Name: "filter",
			SQL:  "WHERE status = 'active'",
		},
		"join": {
			Name: "join",
			SQL:  "LEFT JOIN users u ON u.id = t.user_id",
		},
	}
	
	sql := "SELECT * FROM table1 {{snippet:join}} {{snippet:filter}}"
	expected := "SELECT * FROM table1 LEFT JOIN users u ON u.id = t.user_id WHERE status = 'active'"
	
	result, err := ExpandSnippets(snippets, sql)
	if err != nil {
		t.Fatalf("ExpandSnippets failed: %v", err)
	}
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExpandSnippets_MissingSnippet(t *testing.T) {
	snippets := map[string]*Snippet{}
	
	sql := "SELECT * FROM {{snippet:missing}}"
	_, err := ExpandSnippets(snippets, sql)
	if err == nil {
		t.Fatal("expected error for missing snippet, got nil")
	}
}

func TestExpandSnippets_ParameterInSnippet(t *testing.T) {
	snippets := map[string]*Snippet{
		"date_filter": {
			Name: "date_filter",
			SQL:  "date >= '{{start_date}}' AND date <= '{{end_date}}'",
		},
	}
	
	sql := "SELECT * FROM table1 WHERE {{snippet:date_filter}}"
	expected := "SELECT * FROM table1 WHERE date >= '{{start_date}}' AND date <= '{{end_date}}'"
	
	result, err := ExpandSnippets(snippets, sql)
	if err != nil {
		t.Fatalf("ExpandSnippets failed: %v", err)
	}
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExpandSnippets_DuplicateReferences(t *testing.T) {
	snippets := map[string]*Snippet{
		"base": {
			Name: "base",
			SQL:  "SELECT * FROM base_table",
		},
	}
	
	sql := "SELECT * FROM {{snippet:base}} UNION ALL SELECT * FROM {{snippet:base}}"
	expected := "SELECT * FROM SELECT * FROM base_table UNION ALL SELECT * FROM SELECT * FROM base_table"
	
	result, err := ExpandSnippets(snippets, sql)
	if err != nil {
		t.Fatalf("ExpandSnippets failed: %v", err)
	}
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExtractSnippetRefs(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected []string
	}{
		{
			name:     "no snippets",
			sql:      "SELECT * FROM users",
			expected: []string{},
		},
		{
			name:     "single snippet",
			sql:      "SELECT * FROM {{snippet:base}}",
			expected: []string{"base"},
		},
		{
			name:     "multiple snippets",
			sql:      "SELECT * FROM {{snippet:base}} WHERE {{snippet:filter}}",
			expected: []string{"base", "filter"},
		},
		{
			name:     "duplicate snippets",
			sql:      "SELECT * FROM {{snippet:base}} UNION ALL SELECT * FROM {{snippet:base}}",
			expected: []string{"base", "base"},
		},
		{
			name:     "snippet with hyphens",
			sql:      "SELECT * FROM {{snippet:my-snippet}}",
			expected: []string{"my-snippet"},
		},
		{
			name:     "snippet with underscores",
			sql:      "SELECT * FROM {{snippet:my_snippet}}",
			expected: []string{"my_snippet"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSnippetRefs(tt.sql)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d refs, got %d", len(tt.expected), len(result))
				return
			}
			for i, ref := range result {
				if ref != tt.expected[i] {
					t.Errorf("expected ref %d to be %q, got %q", i, tt.expected[i], ref)
				}
			}
		})
	}
}
